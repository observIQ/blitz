package count

import (
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
