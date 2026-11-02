package count

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func isClosed(ch <-chan struct{}) bool {
	select {
	case <-ch:
		return true
	default:
		return false
	}
}

// acquire takes a permit and fails the test when none is granted.
func acquire(t *testing.T, tracker *Tracker) Permit {
	t.Helper()
	p, _, ok := tracker.Acquire()
	require.True(t, ok)
	return p
}

func TestAcquire_Basic(t *testing.T) {
	tracker := NewTracker(3)

	for i := 0; i < 3; i++ {
		acquire(t, tracker)
	}
	_, _, ok := tracker.Acquire()
	assert.False(t, ok)
	_, _, ok = tracker.Acquire()
	assert.False(t, ok, "repeated calls stay false")
}

func TestAcquire_NilTrackerAlwaysGrants(t *testing.T) {
	var tracker *Tracker
	p, resume, ok := tracker.Acquire()
	assert.True(t, ok)
	assert.Nil(t, resume)
	p.Commit()
	p.Release()
}

func TestDone_ClosesOnLastCommitNotLastAcquire(t *testing.T) {
	tracker := NewTracker(2)

	first := acquire(t, tracker)
	second := acquire(t, tracker)
	assert.False(t, isClosed(tracker.Done()), "every permit taken but none delivered")

	first.Commit()
	assert.False(t, isClosed(tracker.Done()))

	second.Commit()
	assert.True(t, isClosed(tracker.Done()))
}

func TestRelease_ReturnsPermit(t *testing.T) {
	tracker := NewTracker(1)

	p := acquire(t, tracker)
	_, _, ok := tracker.Acquire()
	require.False(t, ok)

	p.Release()
	assert.False(t, isClosed(tracker.Done()), "a failed delivery does not finish the count")

	retry := acquire(t, tracker)
	retry.Commit()
	assert.True(t, isClosed(tracker.Done()))
	assert.Equal(t, int64(1), tracker.Emitted())
}

func TestPermit_SettledOnce(t *testing.T) {
	tracker := NewTracker(2)

	p := acquire(t, tracker)
	p.Commit()
	p.Commit()
	p.Release()
	assert.Equal(t, int64(1), tracker.Emitted(), "a second settle is ignored")
	assert.False(t, isClosed(tracker.Done()))
	_, _, ok := tracker.Acquire()
	assert.True(t, ok)
	_, _, ok = tracker.Acquire()
	assert.False(t, ok, "the release after a commit did not return a permit")
}

func TestPermit_StaleAfterResetIsIgnored(t *testing.T) {
	tracker := NewTracker(1)

	stale := acquire(t, tracker)
	tracker.Reset()
	stale.Commit()
	assert.Equal(t, int64(0), tracker.Emitted(), "a delivery from the old cycle does not count")
	assert.False(t, isClosed(tracker.Done()))

	stale2 := acquire(t, tracker)
	tracker.Reset()
	stale2.Release()
	acquire(t, tracker)
	_, _, ok := tracker.Acquire()
	assert.False(t, ok, "a release from the old cycle does not add a permit")
}

func TestAcquire_ResumeBelongsToTheRefusingCycle(t *testing.T) {
	tracker := NewTracker(1)
	acquire(t, tracker).Commit()

	_, resume, ok := tracker.Acquire()
	require.False(t, ok)
	assert.False(t, isClosed(resume))

	tracker.Reset()
	assert.True(t, isClosed(resume), "the refusing cycle's channel closes on the next reset")

	_, next, ok := tracker.Acquire()
	require.True(t, ok)
	assert.NotEqual(t, resume, next, "a granted permit reports the new cycle's channel")
}

func TestAcquire_Concurrent(t *testing.T) {
	const count = 1000
	const goroutines = 8

	tracker := NewTracker(count)

	var acquired atomic.Int64
	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				p, _, ok := tracker.Acquire()
				if !ok {
					return
				}
				acquired.Add(1)
				p.Commit()
			}
		}()
	}
	wg.Wait()

	assert.Equal(t, int64(count), acquired.Load())
	assert.Equal(t, int64(count), tracker.Emitted())
	assert.True(t, isClosed(tracker.Done()))
}

func TestAcquire_ConcurrentWithReleases(t *testing.T) {
	const count = 500
	const goroutines = 8

	tracker := NewTracker(count)

	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			n := 0
			for {
				p, _, ok := tracker.Acquire()
				if !ok {
					return
				}
				n++
				if n%3 == 0 {
					p.Release()
					continue
				}
				p.Commit()
			}
		}()
	}
	wg.Wait()

	assert.Equal(t, int64(count), tracker.Emitted(), "every released permit is delivered by a retry")
	assert.True(t, isClosed(tracker.Done()))
}

func TestReset(t *testing.T) {
	tracker := NewTracker(3)
	for i := 0; i < 3; i++ {
		acquire(t, tracker).Commit()
	}

	tracker.Reset()

	assert.False(t, isClosed(tracker.Done()), "a reset opens a fresh done channel")
	assert.Equal(t, int64(0), tracker.Emitted())
	for i := 0; i < 3; i++ {
		acquire(t, tracker).Commit()
	}
	_, _, ok := tracker.Acquire()
	assert.False(t, ok)
	assert.True(t, isClosed(tracker.Done()))
}

func TestReset_UnblocksResumeC(t *testing.T) {
	tracker := NewTracker(1)
	resumeCh := tracker.ResumeC()
	assert.False(t, isClosed(resumeCh))

	tracker.Reset()

	select {
	case <-resumeCh:
	case <-time.After(time.Second):
		t.Fatal("ResumeC should have been closed by Reset")
	}
}

func TestReset_MultipleCycles(t *testing.T) {
	tracker := NewTracker(2)

	for cycle := 0; cycle < 5; cycle++ {
		acquire(t, tracker).Commit()
		acquire(t, tracker).Commit()
		_, _, ok := tracker.Acquire()
		assert.False(t, ok, "cycle %d: third acquire should fail", cycle)
		assert.True(t, isClosed(tracker.Done()), "cycle %d: Done should be closed", cycle)

		tracker.Reset()
	}
}

func TestDone_ThreadSafe(t *testing.T) {
	tracker := NewTracker(100)

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				p, _, ok := tracker.Acquire()
				if !ok {
					break
				}
				p.Commit()
			}
			<-tracker.Done()
		}()
	}
	wg.Wait()
}

func TestEmitted_CountsDeliveries(t *testing.T) {
	tracker := NewTracker(10)
	assert.Equal(t, int64(0), tracker.Emitted())

	a := acquire(t, tracker)
	b := acquire(t, tracker)
	assert.Equal(t, int64(0), tracker.Emitted(), "taken permits are not deliveries")

	a.Commit()
	b.Release()
	assert.Equal(t, int64(1), tracker.Emitted())
}

func TestReset_ConcurrentWithAcquire(t *testing.T) {
	const target = 10
	tracker := NewTracker(target)

	// Each send hands a reset to the other goroutine, which runs it while
	// this one keeps acquiring, so resets land between acquires.
	resets := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		for range resets {
			tracker.Reset()
		}
	}()

	for i := range 500 {
		if p, _, ok := tracker.Acquire(); ok {
			p.Commit()
		}
		require.LessOrEqual(t, tracker.Emitted(), int64(target), "a cycle never counts past its target")
		if i%25 == 0 {
			resets <- struct{}{}
		}
	}
	close(resets)
	<-done
}

func TestHold_DefersCommitUntilSent(t *testing.T) {
	tracker := NewTracker(1)
	p := acquire(t, tracker)
	send := Hold(WithPermit(context.Background(), p))

	p.Commit()
	assert.False(t, isClosed(tracker.Done()), "the output has not sent the record yet")
	assert.False(t, isClosed(p.Settled()))

	send(nil)
	assert.True(t, isClosed(tracker.Done()))
	assert.True(t, isClosed(p.Settled()))
	assert.True(t, p.Delivered())
}

func TestHold_FailedSendReleases(t *testing.T) {
	tracker := NewTracker(1)
	p := acquire(t, tracker)
	send := Hold(WithPermit(context.Background(), p))
	p.Commit()

	send(errors.New("connection reset"))
	assert.True(t, isClosed(p.Settled()))
	assert.False(t, p.Delivered())
	assert.False(t, isClosed(tracker.Done()))
	_, _, ok := tracker.Acquire()
	assert.True(t, ok, "the failed record's permit is back for a replacement")
}

func TestHold_EverySendMustSucceed(t *testing.T) {
	tracker := NewTracker(1)
	p := acquire(t, tracker)
	ctx := WithPermit(context.Background(), p)
	first, second := Hold(ctx), Hold(ctx)
	p.Commit()

	first(nil)
	assert.False(t, isClosed(p.Settled()))
	second(errors.New("queue full"))
	assert.False(t, p.Delivered())
	assert.Equal(t, int64(0), tracker.Emitted())
}

func TestHold_SettlesOnce(t *testing.T) {
	tracker := NewTracker(2)
	p := acquire(t, tracker)
	ctx := WithPermit(context.Background(), p)
	send, other := Hold(ctx), Hold(ctx)
	p.Commit()

	send(nil)
	send(nil)
	assert.False(t, isClosed(p.Settled()), "a second settle of one hold does not stand in for another")
	other(nil)
	assert.Equal(t, int64(1), tracker.Emitted())
}

func TestHold_GeneratorFailureWithPendingSend(t *testing.T) {
	tracker := NewTracker(1)
	p := acquire(t, tracker)
	send := Hold(WithPermit(context.Background(), p))

	p.Release()
	assert.False(t, isClosed(p.Settled()), "the queued send still owes its result")
	send(nil)
	assert.False(t, p.Delivered(), "one failed part fails the record")
	_, _, ok := tracker.Acquire()
	assert.True(t, ok)
}

func TestHold_WithoutPermitIsNoop(t *testing.T) {
	Hold(context.Background())(errors.New("ignored"))
	var p Permit
	assert.True(t, isClosed(p.Settled()))
	assert.True(t, p.Delivered())
}

func TestHold_AfterSettleIsNoop(t *testing.T) {
	tracker := NewTracker(2)
	p := acquire(t, tracker)
	ctx := WithPermit(context.Background(), p)
	p.Commit()
	require.True(t, isClosed(p.Settled()))

	Hold(ctx)(errors.New("late"))
	assert.True(t, p.Delivered())
	assert.Equal(t, int64(1), tracker.Emitted())
}

func TestAcquire_RefusedWaitClosesWhenPermitReturns(t *testing.T) {
	tracker := NewTracker(1)
	p := acquire(t, tracker)
	send := Hold(WithPermit(context.Background(), p))
	p.Commit()

	_, wait, ok := tracker.Acquire()
	require.False(t, ok)
	assert.False(t, isClosed(wait))

	send(errors.New("connection reset"))
	assert.True(t, isClosed(wait), "a returned permit wakes refused workers")
	_, _, ok = tracker.Acquire()
	assert.True(t, ok)
}

func TestAcquire_RefusedWaitClosesOnReset(t *testing.T) {
	tracker := NewTracker(1)
	acquire(t, tracker).Commit()
	_, wait, ok := tracker.Acquire()
	require.False(t, ok)

	tracker.Reset()
	assert.True(t, isClosed(wait))
}

func TestAcquire_DeliveryDoesNotWakeRefused(t *testing.T) {
	tracker := NewTracker(2)
	a := acquire(t, tracker)
	acquire(t, tracker)
	_, wait, ok := tracker.Acquire()
	require.False(t, ok)

	a.Commit()
	assert.False(t, isClosed(wait), "a delivered permit frees no slot")
}

// TestRelease_NoWaiterAllocatesNoWakeChannel pins that returning a permit
// with no refused worker waiting costs no channel: against a destination
// that never accepts, every record is returned.
func TestRelease_NoWaiterAllocatesNoWakeChannel(t *testing.T) {
	tracker := NewTracker(1)
	allocs := testing.AllocsPerRun(100, func() {
		p, _, _ := tracker.Acquire()
		p.Release()
	})
	assert.Equal(t, 1.0, allocs, "only the permit's own state")
}

func TestAcquire_RefusedWaitClosesAfterEarlierRelease(t *testing.T) {
	tracker := NewTracker(1)
	acquire(t, tracker).Release()
	p := acquire(t, tracker)
	_, wait, ok := tracker.Acquire()
	require.False(t, ok)
	assert.False(t, isClosed(wait), "an earlier return must not leave the wait closed")

	p.Release()
	assert.True(t, isClosed(wait))
}

func TestCounted(t *testing.T) {
	assert.False(t, Counted(context.Background()))
	assert.True(t, Counted(WithPermit(context.Background(), acquire(t, NewTracker(1)))))
	var nilTracker *Tracker
	p, _, _ := nilTracker.Acquire()
	assert.False(t, Counted(WithPermit(context.Background(), p)), "an uncounted run carries no permit")
}

// TestInherit pins that a record moved onto its own context keeps the permit
// it carried, and that an uncounted record stays uncounted.
func TestInherit(t *testing.T) {
	type key struct{}
	dst := context.WithValue(context.Background(), key{}, "dst")
	tracker := NewTracker(1)
	p := acquire(t, tracker)

	got := Inherit(dst, WithPermit(context.Background(), p))
	assert.Equal(t, "dst", got.Value(key{}), "dst's own values stay")
	send := Hold(got)
	p.Commit()
	assert.False(t, isClosed(tracker.Done()), "the hold on the inherited context is the permit's")
	send(nil)
	assert.True(t, isClosed(tracker.Done()))

	assert.Equal(t, dst, Inherit(dst, context.Background()), "an uncounted source leaves dst as it is")
}

// TestSends_Settle pins that a batch's settle reaches every record in it.
func TestSends_Settle(t *testing.T) {
	for name, err := range map[string]error{"delivered": nil, "failed": errors.New("boom")} {
		t.Run(name, func(t *testing.T) {
			tracker := NewTracker(2)
			a, b := acquire(t, tracker), acquire(t, tracker)
			sends := Sends{Hold(WithPermit(context.Background(), a)), Hold(WithPermit(context.Background(), b))}
			a.Commit()
			b.Commit()

			sends.Settle(err)
			for _, p := range []Permit{a, b} {
				assert.True(t, isClosed(p.Settled()))
				assert.Equal(t, err == nil, p.Delivered())
			}
		})
	}
}

// BenchmarkCountedRecord is one record's tracker work in a count run that
// fans out to two outputs: acquire, a hold per output, both sends, commit.
func BenchmarkCountedRecord(b *testing.B) {
	tracker := NewTracker(int64(b.N) + 1)
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			p, _, _ := tracker.Acquire()
			ctx := WithPermit(context.Background(), p)
			a, c := Hold(ctx), Hold(ctx)
			p.Commit()
			a(nil)
			c(nil)
		}
	})
}

// failed takes a permit and returns it as failed.
func failed(t *testing.T, tracker *Tracker) Permit {
	t.Helper()
	p := acquire(t, tracker)
	p.Release()
	return p
}

// TestForgo_LowersThisCycleOnly pins that forgoing a returned record lowers
// the count for the current cycle, closing Done when the delivered records
// meet it, and that Reset restores the full count.
func TestForgo_LowersThisCycleOnly(t *testing.T) {
	tracker := NewTracker(2)
	acquire(t, tracker).Commit()
	failed(t, tracker).Forgo()
	assert.True(t, isClosed(tracker.Done()), "one delivered record meets a count lowered to one")

	tracker.Reset()
	for range 2 {
		p, _, ok := tracker.Acquire()
		require.True(t, ok, "Reset restores the full count")
		p.Commit()
	}
	assert.True(t, isClosed(tracker.Done()), "the full count is delivered")
}

// TestForgo_AllForgone pins that forgoing every record ends the cycle, and
// that the zero permit is a no-op.
func TestForgo_AllForgone(t *testing.T) {
	tracker := NewTracker(1)
	failed(t, tracker).Forgo()
	assert.True(t, isClosed(tracker.Done()), "nothing left to deliver")
	tracker.Reset()
	acquire(t, tracker).Commit()
	assert.True(t, isClosed(tracker.Done()), "Reset restores the full count")
	assert.NotPanics(t, Permit{}.Forgo)
}

// TestForgo_IgnoresWhatStillCounts pins that Forgo leaves the count alone for
// a permit from before a Reset, one not yet settled, one delivered, and a
// second call on the same permit.
func TestForgo_IgnoresWhatStillCounts(t *testing.T) {
	tracker := NewTracker(1)
	stale := failed(t, tracker)
	tracker.Reset()
	stale.Forgo()
	assert.False(t, isClosed(tracker.Done()), "a permit from before the Reset")

	open := acquire(t, tracker)
	open.Forgo()
	assert.False(t, isClosed(tracker.Done()), "an unsettled permit")

	tracker = NewTracker(2)
	delivered := acquire(t, tracker)
	delivered.Commit()
	delivered.Forgo()
	assert.False(t, isClosed(tracker.Done()), "a delivered permit")

	tracker = NewTracker(2)
	twice := failed(t, tracker)
	twice.Forgo()
	twice.Forgo()
	assert.False(t, isClosed(tracker.Done()), "a second Forgo on one permit")
}
