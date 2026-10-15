package wire

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestUptimeMSNeverWrapsNegative(t *testing.T) {
	boot := time.Unix(1700000000, 0)
	require.Equal(t, uint32(0), UptimeMS(boot, boot.Add(-5*time.Second)), "a time before boot clamps to 0")
	require.Equal(t, uint32(1500), UptimeMS(boot, boot.Add(1500*time.Millisecond)))
	require.Equal(t, uint32(0), UptimeMS(boot, time.Time{}), "zero time is 0")
}

func TestBootPrecedesAnyGeneratedFlow(t *testing.T) {
	now := time.Unix(1700000000, 0)
	// The flow generator back-dates StartTime by up to its 60s active timeout; boot must precede
	// it by a realistic uptime, not by a hair.
	require.True(t, Boot(now).Before(now.Add(-24*time.Hour)))
}
