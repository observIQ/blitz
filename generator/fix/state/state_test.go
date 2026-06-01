package state

import (
	"math/rand"
	"sync"
	"testing"

	"fmt"
	"github.com/observiq/blitz/generator/fix/catalog"
)

func TestNextOutSeqNumMonotonic(t *testing.T) {
	s := NewSession("SENDER", "TARGET")
	for i := 1; i <= 100; i++ {
		if got := s.NextOutSeqNum(); got != i {
			t.Fatalf("NextOutSeqNum() #%d = %d, want %d", i, got, i)
		}
	}
}

func TestNextExecIDMonotonic(t *testing.T) {
	s := NewSession("SENDER", "TARGET")
	seen := make(map[string]bool)
	for i := 0; i < 1000; i++ {
		id := s.NextExecID()
		if seen[id] {
			t.Fatalf("ExecID %q seen twice", id)
		}
		seen[id] = true
	}
}

func TestAddAndLookupOrder(t *testing.T) {
	s := NewSession("SENDER", "TARGET")
	o := Order{ClOrdID: "BLZ-001", OrderID: "ORD-001", Symbol: "AAPL", Side: "1", OrderQty: 100, LeavesQty: 100}
	s.AddOrder(catalog.AssetCategoryEquities, o)

	got, ok := s.LookupOrder(catalog.AssetCategoryEquities, "BLZ-001")
	if !ok {
		t.Fatal("LookupOrder failed for known ClOrdID")
	}
	if got.Symbol != "AAPL" {
		t.Errorf("retrieved order Symbol = %q, want AAPL", got.Symbol)
	}
}

func TestUpdateOrderStatusTerminalRemovesFromBook(t *testing.T) {
	s := NewSession("SENDER", "TARGET")
	s.AddOrder(catalog.AssetCategoryEquities, Order{ClOrdID: "BLZ-002", Symbol: "MSFT", OrderQty: 100, LeavesQty: 100})

	_, ok := s.UpdateOrderStatus(catalog.AssetCategoryEquities, "BLZ-002", OrderStatusFilled, 100, 0)
	if !ok {
		t.Fatal("UpdateOrderStatus returned false for known order")
	}
	if _, ok := s.LookupOrder(catalog.AssetCategoryEquities, "BLZ-002"); ok {
		t.Error("Filled order still tracked")
	}
	if got := s.OpenOrderCount(catalog.AssetCategoryEquities); got != 0 {
		t.Errorf("OpenOrderCount = %d after fill, want 0", got)
	}
}

func TestBookCapEvictsOldest(t *testing.T) {
	s := NewSession("SENDER", "TARGET")
	// Fill beyond cap.
	for i := 0; i < MaxOpenOrdersPerCategory+10; i++ {
		s.AddOrder(catalog.AssetCategoryEquities, Order{
			ClOrdID: idForI(i), Symbol: "X", OrderQty: 1, LeavesQty: 1,
		})
	}
	got := s.OpenOrderCount(catalog.AssetCategoryEquities)
	if got > MaxOpenOrdersPerCategory {
		t.Errorf("OpenOrderCount = %d, want ≤ %d", got, MaxOpenOrdersPerCategory)
	}
	// Earliest orders should be evicted; latest should remain.
	if _, ok := s.LookupOrder(catalog.AssetCategoryEquities, idForI(0)); ok {
		t.Error("oldest order should have been evicted")
	}
	if _, ok := s.LookupOrder(catalog.AssetCategoryEquities, idForI(MaxOpenOrdersPerCategory+5)); !ok {
		t.Error("recent order should still be present")
	}
}

func TestPickOpenOrderDeterministic(t *testing.T) {
	s := NewSession("SENDER", "TARGET")
	for i := 0; i < 10; i++ {
		s.AddOrder(catalog.AssetCategoryEquities, Order{
			ClOrdID: idForI(i), Symbol: "X", OrderQty: 1, LeavesQty: 1,
		})
	}
	a, _ := s.PickOpenOrder(catalog.AssetCategoryEquities, rand.New(rand.NewSource(42)))
	b, _ := s.PickOpenOrder(catalog.AssetCategoryEquities, rand.New(rand.NewSource(42)))
	if a.ClOrdID != b.ClOrdID {
		t.Errorf("PickOpenOrder not deterministic: %q vs %q", a.ClOrdID, b.ClOrdID)
	}
}

func TestConcurrentAccess(t *testing.T) {
	// Stress test for races. Run with -race in CI; this just exercises
	// the mutex paths.
	s := NewSession("SENDER", "TARGET")
	var wg sync.WaitGroup
	for g := 0; g < 10; g++ {
		wg.Add(1)
		go func(seed int) {
			defer wg.Done()
			r := rand.New(rand.NewSource(int64(seed)))
			for i := 0; i < 100; i++ {
				s.NextOutSeqNum()
				s.NextExecID()
				s.AddOrder(catalog.AssetCategoryEquities, Order{
					ClOrdID: fmt.Sprintf("G%d-%d", seed, i), Symbol: "X", OrderQty: 1, LeavesQty: 1,
				})
				s.PickOpenOrder(catalog.AssetCategoryEquities, r)
			}
		}(g)
	}
	wg.Wait()
}

func TestSimulatedLatencyDeterministic(t *testing.T) {
	a := SimulatedLatency(rand.New(rand.NewSource(42)))
	b := SimulatedLatency(rand.New(rand.NewSource(42)))
	if a != b {
		t.Errorf("SimulatedLatency not deterministic: %v vs %v", a, b)
	}
}

func TestParseOrderQty(t *testing.T) {
	cases := []struct {
		in   string
		want int64
	}{
		{"100", 100},
		{"0", 0},
		{"garbage", 0},
		{"-5", -5},
	}
	for _, c := range cases {
		if got := ParseOrderQty(c.in); got != c.want {
			t.Errorf("ParseOrderQty(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}

func idForI(i int) string {
	return fmt.Sprintf("BLZ-%08d", i)
}
