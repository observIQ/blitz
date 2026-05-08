package datagen

import (
	"math/rand"
	"testing"
)

func TestNewPool(t *testing.T) {
	t.Run("creates pool with items", func(t *testing.T) {
		p := NewPool("a", "b", "c")
		if p.Len() != 3 {
			t.Errorf("expected Len() = 3, got %d", p.Len())
		}
	})

	t.Run("empty pool", func(t *testing.T) {
		p := NewPool[string]()
		if p.Len() != 0 {
			t.Errorf("expected Len() = 0, got %d", p.Len())
		}
	})
}

func TestPoolAll(t *testing.T) {
	items := []string{"x", "y", "z"}
	p := NewPool(items...)
	all := p.All()
	if len(all) != 3 {
		t.Fatalf("expected 3 items, got %d", len(all))
	}
	for i, v := range items {
		if all[i] != v {
			t.Errorf("All()[%d] = %q, want %q", i, all[i], v)
		}
	}

	// Ensure returned slice is a copy
	all[0] = "modified"
	if p.All()[0] == "modified" {
		t.Error("All() should return a copy, not the internal slice")
	}
}

func TestPoolRandom(t *testing.T) {
	p := NewPool("a", "b", "c")
	r := rand.New(rand.NewSource(42))

	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		v := p.Random(r)
		seen[v] = true
	}
	// With 100 draws from 3 items, we should see all of them
	if len(seen) != 3 {
		t.Errorf("expected to see all 3 items, saw %d: %v", len(seen), seen)
	}
}

func TestPoolRandomDeterministic(t *testing.T) {
	p := NewPool(1, 2, 3, 4, 5)

	r1 := rand.New(rand.NewSource(99))
	r2 := rand.New(rand.NewSource(99))

	for i := 0; i < 20; i++ {
		v1 := p.Random(r1)
		v2 := p.Random(r2)
		if v1 != v2 {
			t.Fatalf("draw %d: same seed produced different results: %d vs %d", i, v1, v2)
		}
	}
}

func TestPoolRandomN(t *testing.T) {
	p := NewPool("a", "b", "c", "d", "e")
	r := rand.New(rand.NewSource(42))

	t.Run("n less than pool size", func(t *testing.T) {
		result := p.RandomN(r, 3)
		if len(result) != 3 {
			t.Errorf("expected 3 items, got %d", len(result))
		}
		// Check uniqueness
		seen := make(map[string]bool)
		for _, v := range result {
			if seen[v] {
				t.Errorf("duplicate item %q in RandomN result", v)
			}
			seen[v] = true
		}
	})

	t.Run("n equals pool size", func(t *testing.T) {
		result := p.RandomN(r, 5)
		if len(result) != 5 {
			t.Errorf("expected 5 items, got %d", len(result))
		}
	})

	t.Run("n exceeds pool size returns all", func(t *testing.T) {
		result := p.RandomN(r, 10)
		if len(result) != 5 {
			t.Errorf("expected 5 items (pool size), got %d", len(result))
		}
	})

	t.Run("n zero returns empty", func(t *testing.T) {
		result := p.RandomN(r, 0)
		if len(result) != 0 {
			t.Errorf("expected 0 items, got %d", len(result))
		}
	})
}

func TestMerge(t *testing.T) {
	p1 := NewPool("a", "b")
	p2 := NewPool("c", "d")
	p3 := NewPool("e")

	merged := Merge(p1, p2, p3)
	if merged.Len() != 5 {
		t.Errorf("expected merged Len() = 5, got %d", merged.Len())
	}

	all := merged.All()
	expected := []string{"a", "b", "c", "d", "e"}
	for i, v := range expected {
		if all[i] != v {
			t.Errorf("merged.All()[%d] = %q, want %q", i, all[i], v)
		}
	}
}

func TestMergeEmpty(t *testing.T) {
	merged := Merge[string]()
	if merged.Len() != 0 {
		t.Errorf("expected merged Len() = 0, got %d", merged.Len())
	}
}

func TestPoolRandomPanicsOnEmpty(t *testing.T) {
	p := NewPool[string]()
	r := rand.New(rand.NewSource(42))

	defer func() {
		if recover() == nil {
			t.Error("expected panic on Random() with empty pool")
		}
	}()
	p.Random(r)
}

func TestPoolWithInts(t *testing.T) {
	p := NewPool(200, 201, 204, 301, 404, 500)
	r := rand.New(rand.NewSource(42))

	v := p.Random(r)
	found := false
	for _, item := range p.All() {
		if item == v {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Random() returned %d which is not in the pool", v)
	}
}
