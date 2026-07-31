package count

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewTracker(t *testing.T) {
	tracker := NewTracker(10)
	assert.NotNil(t, tracker)
	assert.Equal(t, int64(10), tracker.remaining.Load())
	assert.Equal(t, int64(10), tracker.target)
}

func TestAcquire_Basic(t *testing.T) {
	tracker := NewTracker(3)

	assert.True(t, tracker.Acquire())
	assert.True(t, tracker.Acquire())
	assert.True(t, tracker.Acquire())
	assert.False(t, tracker.Acquire())
	assert.False(t, tracker.Acquire()) // repeated calls stay false
}

func TestAcquire_SinglePermit(t *testing.T) {
	tracker := NewTracker(1)

	assert.True(t, tracker.Acquire())
	assert.False(t, tracker.Acquire())
}

func TestDone_ClosedWhenExhausted(t *testing.T) {
	tracker := NewTracker(2)

	// Not closed yet
	select {
	case <-tracker.Done():
		t.Fatal("Done should not be closed yet")
	default:
	}

	tracker.Acquire()

	// Still not closed
	select {
	case <-tracker.Done():
		t.Fatal("Done should not be closed yet")
	default:
	}

	tracker.Acquire()

	// Now closed
	select {
	case <-tracker.Done():
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Done should be closed after all permits consumed")
	}
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
			for tracker.Acquire() {
				acquired.Add(1)
			}
		}()
	}

	wg.Wait()

	assert.Equal(t, int64(count), acquired.Load())
	assert.Equal(t, int64(0), tracker.remaining.Load())

	// Done should be closed
	select {
	case <-tracker.Done():
	default:
		t.Fatal("Done should be closed")
	}
}

func TestReset(t *testing.T) {
	tracker := NewTracker(3)

	// Exhaust all permits
	for tracker.Acquire() {
	}
	assert.False(t, tracker.Acquire())

	// Reset
	tracker.Reset()

	// Should have permits again
	assert.True(t, tracker.Acquire())
	assert.True(t, tracker.Acquire())
	assert.True(t, tracker.Acquire())
	assert.False(t, tracker.Acquire())
}

func TestReset_UnblocksResumeC(t *testing.T) {
	tracker := NewTracker(1)
	tracker.Acquire()

	resumeCh := tracker.ResumeC()

	// Not closed yet
	select {
	case <-resumeCh:
		t.Fatal("ResumeC should not be closed yet")
	default:
	}

	// Reset from a goroutine. The "not closed yet" check above already ran, so there
	// is no need to delay before resetting.
	go func() {
		tracker.Reset()
	}()

	select {
	case <-resumeCh:
	case <-time.After(time.Second):
		t.Fatal("ResumeC should have been closed by Reset")
	}
}

func TestReset_NewDoneChannel(t *testing.T) {
	tracker := NewTracker(1)
	tracker.Acquire()

	// First Done is closed
	select {
	case <-tracker.Done():
	default:
		t.Fatal("Done should be closed")
	}

	tracker.Reset()

	// New Done is open
	select {
	case <-tracker.Done():
		t.Fatal("Done should not be closed after Reset")
	default:
	}

	// Exhaust again
	tracker.Acquire()

	// New Done is now closed
	select {
	case <-tracker.Done():
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Done should be closed after second exhaustion")
	}
}

func TestReset_MultipleCycles(t *testing.T) {
	tracker := NewTracker(2)

	for cycle := 0; cycle < 5; cycle++ {
		assert.True(t, tracker.Acquire(), "cycle %d: first acquire", cycle)
		assert.True(t, tracker.Acquire(), "cycle %d: second acquire", cycle)
		assert.False(t, tracker.Acquire(), "cycle %d: third acquire should fail", cycle)

		select {
		case <-tracker.Done():
		default:
			t.Fatalf("cycle %d: Done should be closed", cycle)
		}

		tracker.Reset()
	}
}

func TestResumeC_NotClosedWithoutReset(t *testing.T) {
	tracker := NewTracker(1)

	select {
	case <-tracker.ResumeC():
		t.Fatal("ResumeC should not be closed without Reset")
	default:
	}
}

func TestDone_ThreadSafe(t *testing.T) {
	tracker := NewTracker(100)

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for tracker.Acquire() {
			}
			<-tracker.Done()
		}()
	}

	wg.Wait()
}

func TestEmitted(t *testing.T) {
	tracker := NewTracker(10)
	assert.Equal(t, int64(0), tracker.Emitted())

	tracker.Acquire()
	tracker.Acquire()
	tracker.Acquire()
	assert.Equal(t, int64(3), tracker.Emitted())

	// Exhaust the rest
	for tracker.Acquire() {
	}
	assert.Equal(t, int64(10), tracker.Emitted())
}

func TestEmitted_AfterReset(t *testing.T) {
	tracker := NewTracker(5)
	for tracker.Acquire() {
	}
	assert.Equal(t, int64(5), tracker.Emitted())

	tracker.Reset()
	assert.Equal(t, int64(0), tracker.Emitted())

	tracker.Acquire()
	tracker.Acquire()
	assert.Equal(t, int64(2), tracker.Emitted())
}

func TestReset_ConcurrentWithAcquire(t *testing.T) {
	tracker := NewTracker(10)

	var wg sync.WaitGroup

	// Goroutine that acquires
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			tracker.Acquire()
		}
	}()

	// Goroutine that resets periodically. The 1ms spacing is intentional: it spreads
	// the resets so they interleave with the concurrent Acquire calls, which is the
	// concurrency this test exercises. It is not a wait-for-state sleep.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 5; i++ {
			time.Sleep(time.Millisecond)
			tracker.Reset()
		}
	}()

	wg.Wait()
	// No panics, no deadlocks = pass
}
