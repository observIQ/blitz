package count

import (
	"sync"
)

// Tracker tracks a finite number of telemetry deliveries.
// It is safe for concurrent use by multiple worker goroutines.
//
// Workers call Acquire() before each generation, then settle the permit:
// Commit() once the record is delivered, or Release() when delivery fails so
// a retry can take the permit again. Done() closes on the commit that
// completes the count, so a run ends only after its last record is
// delivered. Delivered means the consumer accepted the record without error;
// a queued output sends it afterwards. In idle mode, workers block on the
// resume channel until Reset() is called (e.g., via SIGUSR1).
//
// A nil *Tracker grants every Acquire, so generators need no nil checks;
// its other methods need a tracker.
type Tracker struct {
	mu        sync.Mutex
	target    int64
	reserved  int64 // permits taken and not released this cycle
	delivered int64 // permits committed this cycle
	cycle     uint64
	doneCh    chan struct{}
	resumeCh  chan struct{}
}

// Permit is one reserved delivery. Settle it exactly once with Commit or
// Release; later calls, and calls on a permit from before a Reset, are
// ignored. The zero Permit (from a nil Tracker) ignores both.
type Permit struct {
	t     *Tracker
	cycle uint64
	// settled is shared by copies of the permit, so a copy cannot settle twice.
	settled *bool
}

// NewTracker creates a tracker with the given count of permits.
// count must be > 0.
func NewTracker(count int64) *Tracker {
	return &Tracker{
		target:   count,
		doneCh:   make(chan struct{}),
		resumeCh: make(chan struct{}),
	}
}

// Acquire reserves one permit and reports whether it was granted. resume is
// the current cycle's resume channel, read under the same lock as the
// grant, so a refused worker that waits on it never misses a Reset that
// lands after the refusal.
func (t *Tracker) Acquire() (p Permit, resume <-chan struct{}, ok bool) {
	if t == nil {
		return Permit{}, nil, true
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.reserved >= t.target {
		return Permit{}, t.resumeCh, false
	}
	t.reserved++
	return Permit{t: t, cycle: t.cycle, settled: new(bool)}, t.resumeCh, true
}

// Commit records the permit's record as delivered. The commit that
// completes the count closes Done().
func (p Permit) Commit() {
	if p.t == nil {
		return
	}
	p.t.mu.Lock()
	defer p.t.mu.Unlock()
	if !p.settle() {
		return
	}
	p.t.delivered++
	if p.t.delivered == p.t.target {
		close(p.t.doneCh)
	}
}

// Release returns the permit after a failed delivery.
func (p Permit) Release() {
	if p.t == nil {
		return
	}
	p.t.mu.Lock()
	defer p.t.mu.Unlock()
	if p.settle() {
		p.t.reserved--
	}
}

// settle marks the permit used and reports whether it still counts. The
// caller holds p.t.mu.
func (p Permit) settle() bool {
	if *p.settled {
		return false
	}
	*p.settled = true
	return p.cycle == p.t.cycle
}

// Done returns a channel that is closed when all permits are delivered.
func (t *Tracker) Done() <-chan struct{} {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.doneCh
}

// ResumeC returns a channel that is closed when Reset() is called. The run
// loop waits on it alongside Done, so a reset mid-run starts a new cycle;
// generators wait on the channel Acquire returns.
func (t *Tracker) ResumeC() <-chan struct{} {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.resumeCh
}

// Emitted returns the number of records delivered this cycle. The name
// predates delivery counting and is kept for API compatibility.
func (t *Tracker) Emitted() int64 {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.delivered
}

// Reset restores the tracker to its original count, allowing generation
// to resume. It closes the current resumeCh to unblock idle workers,
// creates fresh doneCh/resumeCh for the next cycle, and starts a new cycle
// so permits still in flight from the old one no longer count.
func (t *Tracker) Reset() {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.reserved = 0
	t.delivered = 0
	t.cycle++

	// Close old resumeCh to unblock any workers waiting on it.
	close(t.resumeCh)

	// Create fresh channels for the next cycle.
	t.doneCh = make(chan struct{})
	t.resumeCh = make(chan struct{})
}
