package count

import (
	"sync"
	"sync/atomic"
)

// Tracker tracks a finite number of telemetry generation permits.
// It is safe for concurrent use by multiple worker goroutines.
//
// Workers call Acquire() before each generation. When all permits are
// consumed, Acquire() returns false and Done() is closed. In idle mode,
// workers block on ResumeC() until Reset() is called (e.g., via SIGUSR1).
type Tracker struct {
	remaining atomic.Int64
	target    int64

	mu       sync.Mutex
	doneCh   chan struct{}
	doneOnce sync.Once
	resumeCh chan struct{}
}

// NewTracker creates a tracker with the given count of permits.
// count must be > 0.
func NewTracker(count int64) *Tracker {
	t := &Tracker{
		target:   count,
		doneCh:   make(chan struct{}),
		resumeCh: make(chan struct{}),
	}
	t.remaining.Store(count)
	return t
}

// Acquire attempts to consume one permit. Returns true if the permit was
// granted, false if the count is exhausted. When the last permit is
// consumed, Done() is closed.
//
// Acquire uses a CAS loop that never overshoots the target count.
func (t *Tracker) Acquire() bool {
	for {
		cur := t.remaining.Load()
		if cur <= 0 {
			return false
		}
		if t.remaining.CompareAndSwap(cur, cur-1) {
			if cur-1 == 0 {
				t.doneOnce.Do(func() { close(t.doneCh) })
			}
			return true
		}
	}
}

// Done returns a channel that is closed when all permits are consumed.
func (t *Tracker) Done() <-chan struct{} {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.doneCh
}

// ResumeC returns a channel that is closed when Reset() is called.
// Workers in idle mode select on this to detect when generation should restart.
func (t *Tracker) ResumeC() <-chan struct{} {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.resumeCh
}

// Emitted returns the number of permits that have been consumed.
func (t *Tracker) Emitted() int64 {
	return t.target - t.remaining.Load()
}

// Reset restores the tracker to its original count, allowing generation
// to resume. It closes the current resumeCh to unblock idle workers,
// and creates fresh doneCh/resumeCh for the next cycle.
func (t *Tracker) Reset() {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.remaining.Store(t.target)

	// Close old resumeCh to unblock any workers waiting on it.
	close(t.resumeCh)

	// Create fresh channels for the next cycle.
	t.doneCh = make(chan struct{})
	t.doneOnce = sync.Once{}
	t.resumeCh = make(chan struct{})
}
