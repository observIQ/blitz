package pacing

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestNewWriteBackoffStartsAtTheRate pins that the first interval is the rate
// (within the library's 50% jitter), not its 500ms default, and that a rate
// above 5s still paces the worker at the rate.
func TestNewWriteBackoffStartsAtTheRate(t *testing.T) {
	for _, rate := range []time.Duration{time.Millisecond, 10 * time.Second} {
		bo := NewWriteBackoff(rate)
		first := bo.NextBackOff()
		require.GreaterOrEqual(t, first, rate/2, "rate %s", rate)
		require.LessOrEqual(t, first, rate*3/2, "rate %s", rate)
		require.Equal(t, max(5*time.Second, rate), bo.MaxInterval)
		require.Zero(t, bo.MaxElapsedTime, "never gives up")
	}
}

// TestAwaitWakeRestartsAtTheRate pins that a wake rearms the timer at the
// rate, not at a backoff grown by earlier wakes: a parked worker did not fail.
func TestAwaitWakeRestartsAtTheRate(t *testing.T) {
	bo := NewWriteBackoff(time.Millisecond)
	for range 30 {
		bo.NextBackOff() // grown to the 5s cap
	}
	timer := time.NewTimer(time.Hour)
	defer timer.Stop()
	wait := make(chan struct{})
	close(wait)

	require.True(t, AwaitWake(make(chan struct{}), wait, bo, timer))
	select {
	case <-timer.C:
	case <-time.After(time.Second):
		t.Fatal("timer rearmed at the grown backoff, not the rate")
	}
}

// TestAwaitWakeStops pins that a closed stop channel ends the wait.
func TestAwaitWakeStops(t *testing.T) {
	stop := make(chan struct{})
	close(stop)
	timer := time.NewTimer(time.Hour)
	defer timer.Stop()
	require.False(t, AwaitWake(stop, make(chan struct{}), NewWriteBackoff(time.Millisecond), timer))
}
