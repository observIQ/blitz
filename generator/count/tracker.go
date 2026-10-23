package count

import (
	"context"
	"sync"
)

// Tracker counts a finite run's deliveries and is safe for concurrent use.
// A worker takes a Permit with Acquire before each record and settles it
// (see Permit); Done closes once the count is delivered, and Reset (SIGUSR1)
// starts a new cycle. A nil *Tracker grants every Acquire, so generators need
// no nil checks; its other methods need a tracker.
type Tracker struct {
	mu        sync.Mutex
	count     int64 // the full count, restored by Reset
	target    int64 // this cycle's count, lowered by Forgo
	reserved  int64 // permits taken and not released this cycle
	delivered int64 // permits delivered this cycle
	cycle     uint64
	doneCh    chan struct{}
	resumeCh  chan struct{}
	wakeCh    chan struct{} // made on a refusal; closed when a permit returns or on Reset
}

// Permit is one reserved delivery. The generator settles it once with Commit
// or Release (later calls are ignored), and each output with its Hold. It
// counts when every part succeeded and goes back to the pool when any
// failed. A permit from before a Reset never counts. The zero Permit, from a
// nil Tracker, is a no-op.
type Permit struct {
	t *Tracker
	// st is shared by copies of the permit.
	st *permitState
}

// permitState is guarded by its own mu, so a record's holds and settles
// contend only with each other; the tracker's lock is taken once more, to
// count or return the permit. Lock order: permitState.mu, then Tracker.mu.
type permitState struct {
	mu        sync.Mutex
	cycle     uint64        // the Reset cycle the permit was granted in
	settled   bool          // the generator's Commit or Release ran
	holds     int           // output sends still owed
	failed    bool          // Release ran or a send failed
	final     bool          // counted or returned; later holds are no-ops
	delivered bool          // counted, meaningful once final
	forgone   bool          // Forgo ran
	doneCh    chan struct{} // made on the first Settled call
}

// NewTracker creates a tracker with the given count of permits.
// count must be > 0.
func NewTracker(count int64) *Tracker {
	return &Tracker{
		count:    count,
		target:   count,
		doneCh:   make(chan struct{}),
		resumeCh: make(chan struct{}),
	}
}

// Acquire reserves one permit and reports whether it was granted. On a
// refusal, wait closes once a permit comes back or Reset is called, so the
// worker can try again. On a grant, wait is the permit's cycle channel,
// which closes on the next Reset; replay compares it to spot a new cycle.
// Both are read under the same lock as the decision, so neither event is
// missed.
func (t *Tracker) Acquire() (p Permit, wait <-chan struct{}, ok bool) {
	if t == nil {
		return Permit{}, nil, true
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.reserved >= t.target {
		if t.wakeCh == nil {
			t.wakeCh = make(chan struct{})
		}
		return Permit{}, t.wakeCh, false
	}
	t.reserved++
	return Permit{t: t, st: &permitState{cycle: t.cycle}}, t.resumeCh, true
}

// Commit settles the generator's part as handed off.
func (p Permit) Commit() { p.settle(false) }

// Release settles the generator's part as failed.
func (p Permit) Release() { p.settle(true) }

func (p Permit) settle(failed bool) {
	if p.t == nil {
		return
	}
	p.st.mu.Lock()
	defer p.st.mu.Unlock()
	if p.st.settled {
		return
	}
	p.st.settled = true
	p.st.failed = p.st.failed || failed
	p.finalize()
}

// finalize counts or returns the permit once the generator and every hold
// have settled. The caller holds p.st.mu.
func (p Permit) finalize() {
	st := p.st
	if st.final || !st.settled || st.holds > 0 {
		return
	}
	st.final = true
	st.delivered = !st.failed
	if st.doneCh != nil {
		close(st.doneCh)
	}
	p.t.mu.Lock()
	defer p.t.mu.Unlock()
	if st.cycle != p.t.cycle {
		return
	}
	if st.failed {
		p.t.reserved--
		p.t.wake()
		return
	}
	p.t.delivered++
	if p.t.delivered == p.t.target {
		close(p.t.doneCh)
	}
}

// Settled returns a channel closed once the permit is counted or returned.
func (p Permit) Settled() <-chan struct{} {
	if p.t == nil {
		return closedCh
	}
	p.st.mu.Lock()
	defer p.st.mu.Unlock()
	if p.st.final {
		return closedCh
	}
	if p.st.doneCh == nil {
		p.st.doneCh = make(chan struct{})
	}
	return p.st.doneCh
}

// Forgo lowers the permit's cycle count by one, for a returned record that
// will never be delivered, so the cycle can still end. It counts once per
// permit, and does nothing for a permit that is unsettled, delivered, or from
// before a Reset. Reset restores the full count.
func (p Permit) Forgo() {
	if p.t == nil {
		return
	}
	p.st.mu.Lock()
	defer p.st.mu.Unlock()
	if !p.st.final || p.st.delivered || p.st.forgone {
		return
	}
	p.st.forgone = true
	t := p.t
	t.mu.Lock()
	defer t.mu.Unlock()
	if p.st.cycle != t.cycle {
		return
	}
	t.target--
	if t.delivered == t.target {
		close(t.doneCh)
	}
}

// Delivered reports whether the record was delivered. It is meaningful once
// Settled() is closed. A permit from before a Reset can report true without
// counting toward the new cycle.
func (p Permit) Delivered() bool {
	if p.t == nil {
		return true
	}
	p.st.mu.Lock()
	defer p.st.mu.Unlock()
	return p.st.delivered
}

var closedCh = func() chan struct{} {
	ch := make(chan struct{})
	close(ch)
	return ch
}()

type permitKey struct{}

// WithPermit returns ctx carrying p, for outputs that send the record later.
func WithPermit(ctx context.Context, p Permit) context.Context {
	if p.t == nil {
		return ctx
	}
	return context.WithValue(ctx, permitKey{}, p)
}

// Counted reports whether ctx carries a permit, so the record counts
// toward a finite run.
func Counted(ctx context.Context) bool {
	_, ok := ctx.Value(permitKey{}).(Permit)
	return ok
}

// Inherit returns dst carrying the permit src carries, if any, for a record
// that crosses a queue onto a context of its own.
func Inherit(dst, src context.Context) context.Context {
	if p, ok := src.Value(permitKey{}).(Permit); ok {
		return context.WithValue(dst, permitKey{}, p)
	}
	return dst
}

// Hold registers one send still owed on the permit ctx carries, and returns
// the function that settles it with the send's result. Take the hold before
// the record can be sent, and call the function once; later calls are
// ignored. Without a permit, or once the permit is counted or returned (its
// Settled channel closed), it is a no-op.
func Hold(ctx context.Context) func(error) {
	p, ok := ctx.Value(permitKey{}).(Permit)
	if !ok {
		return func(error) {}
	}
	st := p.st
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.final {
		return func(error) {}
	}
	st.holds++
	done := false // guarded by st.mu
	return func(err error) {
		st.mu.Lock()
		defer st.mu.Unlock()
		if done {
			return
		}
		done = true
		st.holds--
		st.failed = st.failed || err != nil
		p.finalize()
	}
}

// Sends holds the settle functions for a batch of records sent together.
type Sends []func(error)

// Settle settles every record in the batch with err.
func (s Sends) Settle(err error) {
	for _, sent := range s {
		sent(err)
	}
}

// wake closes the wait channel of any refused worker. The caller holds t.mu.
func (t *Tracker) wake() {
	if t.wakeCh != nil {
		close(t.wakeCh)
		t.wakeCh = nil
	}
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
// to resume. It closes the current resumeCh and wakeCh to unblock idle and
// refused workers, creates fresh doneCh/resumeCh for the next cycle,
// and starts a new cycle so permits still in flight from the old one no
// longer count.
func (t *Tracker) Reset() {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.target = t.count
	t.reserved = 0
	t.delivered = 0
	t.cycle++

	// Close the old channels to unblock any workers waiting on them.
	close(t.resumeCh)
	t.wake()

	// Create fresh channels for the next cycle.
	t.doneCh = make(chan struct{})
	t.resumeCh = make(chan struct{})
}
