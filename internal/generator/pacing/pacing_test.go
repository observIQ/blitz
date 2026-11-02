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
