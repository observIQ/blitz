// Package state holds the FIX session and per-category order-book
// state used by the FIX generator at emit time.
//
// Concurrency: every public method on Session is guarded by an internal
// sync.Mutex. Multiple goroutines may invoke methods concurrently;
// linearization is correct but not lock-free.
//
// Bounded memory: each per-category order book is capped by
// MaxOpenOrdersPerCategory. When the cap is hit, the oldest entry by
// insertion order is evicted.
//
// Determinism: callers supply a *rand.Rand. The Session uses that RNG
// for every non-deterministic choice (instrument selection caching,
// fill-versus-cancel branching, latency jitter). Given the same RNG
// seed + the same time inputs, the Session produces identical state
// transitions.
package state

import (
	"container/list"
	"fmt"
	"math/rand"
	"strconv"
	"sync"
	"time"

	"github.com/observiq/blitz/generator/fix/catalog"
)

// MaxOpenOrdersPerCategory bounds memory consumption.
const MaxOpenOrdersPerCategory = 10000

// OrderStatus tracks the current state of an open order in the book.
type OrderStatus int

const (
	OrderStatusNew OrderStatus = iota
	OrderStatusPartiallyFilled
	OrderStatusFilled
	OrderStatusCanceled
	OrderStatusRejected
	OrderStatusExpired
)

func (s OrderStatus) String() string {
	switch s {
	case OrderStatusNew:
		return "new"
	case OrderStatusPartiallyFilled:
		return "partial"
	case OrderStatusFilled:
		return "filled"
	case OrderStatusCanceled:
		return "canceled"
	case OrderStatusRejected:
		return "rejected"
	case OrderStatusExpired:
		return "expired"
	}
	return "unknown"
}

// Order represents one open order tracked in the per-category book.
type Order struct {
	ClOrdID   string
	OrderID   string
	Symbol    string
	Side      string
	OrderQty  int64
	CumQty    int64
	LeavesQty int64
	Price     string
	Status    OrderStatus
	Submitted time.Time
}

// Session holds the FIX session state for one (SenderCompID,
// TargetCompID) pair plus the per-category order books.
type Session struct {
	SenderCompID string
	TargetCompID string

	mu            sync.Mutex
	nextOutSeqNum int
	lastInSeqNum  int
	execIDCounter int64

	// Per-category open-order books. Each book is an LRU keyed by
	// ClOrdID via a doubly-linked list + map for O(1) eviction.
	books map[catalog.AssetCategory]*orderBook
}

type orderBook struct {
	order map[string]*list.Element // ClOrdID -> list element
	lru   *list.List               // values are *Order
}

// NewSession constructs a fresh Session. SenderCompID and TargetCompID
// identify the two FIX endpoints; nextOutSeqNum starts at 1.
func NewSession(sender, target string) *Session {
	return &Session{
		SenderCompID:  sender,
		TargetCompID:  target,
		nextOutSeqNum: 1,
		books:         make(map[catalog.AssetCategory]*orderBook),
	}
}

// NextOutSeqNum returns and increments the outgoing sequence number.
func (s *Session) NextOutSeqNum() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := s.nextOutSeqNum
	s.nextOutSeqNum++
	return n
}

// RecordInSeqNum updates the last-received sequence number.
func (s *Session) RecordInSeqNum(n int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if n > s.lastInSeqNum {
		s.lastInSeqNum = n
	}
}

// NextExecID returns a fresh monotonically-increasing ExecID.
func (s *Session) NextExecID() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.execIDCounter++
	return fmt.Sprintf("EXE-%010d", s.execIDCounter)
}

// AddOrder inserts a new open order into the per-category book. If the
// book is at capacity, the oldest entry is evicted.
func (s *Session) AddOrder(cat catalog.AssetCategory, o Order) {
	s.mu.Lock()
	defer s.mu.Unlock()
	book := s.bookLocked(cat)
	if _, exists := book.order[o.ClOrdID]; exists {
		return // dedupe
	}
	if book.lru.Len() >= MaxOpenOrdersPerCategory {
		front := book.lru.Front()
		if front != nil {
			evicted := front.Value.(*Order)
			delete(book.order, evicted.ClOrdID)
			book.lru.Remove(front)
		}
	}
	cpy := o
	elem := book.lru.PushBack(&cpy)
	book.order[o.ClOrdID] = elem
}

// LookupOrder returns the order for the given ClOrdID, or false.
func (s *Session) LookupOrder(cat catalog.AssetCategory, clOrdID string) (Order, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, ok := s.books[cat]
	if !ok {
		return Order{}, false
	}
	elem, ok := b.order[clOrdID]
	if !ok {
		return Order{}, false
	}
	return *elem.Value.(*Order), true
}

// UpdateOrderStatus applies a status transition to an existing order
// and returns the updated value. Returns false if the order isn't
// tracked.
func (s *Session) UpdateOrderStatus(cat catalog.AssetCategory, clOrdID string, status OrderStatus, cumQty, leavesQty int64) (Order, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, ok := s.books[cat]
	if !ok {
		return Order{}, false
	}
	elem, ok := b.order[clOrdID]
	if !ok {
		return Order{}, false
	}
	ord := elem.Value.(*Order)
	ord.Status = status
	ord.CumQty = cumQty
	ord.LeavesQty = leavesQty
	// Remove from book on terminal states.
	switch status {
	case OrderStatusFilled, OrderStatusCanceled, OrderStatusRejected, OrderStatusExpired:
		delete(b.order, clOrdID)
		b.lru.Remove(elem)
	}
	return *ord, true
}

// OpenOrderCount returns the number of open orders in a category.
func (s *Session) OpenOrderCount(cat catalog.AssetCategory) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, ok := s.books[cat]
	if !ok {
		return 0
	}
	return b.lru.Len()
}

// PickOpenOrder returns a deterministic random open order from the
// category, or false if none exist. Uses the supplied RNG.
func (s *Session) PickOpenOrder(cat catalog.AssetCategory, r *rand.Rand) (Order, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, ok := s.books[cat]
	if !ok || b.lru.Len() == 0 {
		return Order{}, false
	}
	target := r.Intn(b.lru.Len()) // #nosec G404 -- seeded RNG
	idx := 0
	for e := b.lru.Front(); e != nil; e = e.Next() {
		if idx == target {
			return *e.Value.(*Order), true
		}
		idx++
	}
	return Order{}, false
}

// bookLocked returns (or creates) the book for cat. Caller must hold mu.
func (s *Session) bookLocked(cat catalog.AssetCategory) *orderBook {
	b, ok := s.books[cat]
	if !ok {
		b = &orderBook{
			order: make(map[string]*list.Element),
			lru:   list.New(),
		}
		s.books[cat] = b
	}
	return b
}

// SimulatedLatency returns a deterministic latency value (in
// milliseconds) representing simulated venue round-trip time. Range:
// 1ms-50ms. Used to space out ExecutionReports relative to
// NewOrderSingles when emitting.
func SimulatedLatency(r *rand.Rand) time.Duration {
	ms := 1 + r.Intn(50) // #nosec G404 -- seeded RNG
	return time.Duration(ms) * time.Millisecond
}

// ParseOrderQty parses a string quantity field back to an int64 for
// state tracking. Returns 0 on parse error.
func ParseOrderQty(s string) int64 {
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0
	}
	return n
}
