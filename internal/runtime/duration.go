package runtime

import "time"

// DurationMillis converts a duration to fractional milliseconds for recording
// on the startup-latency histograms. It divides the nanosecond count as a
// float so a sub-millisecond value keeps its fractional part. time.Duration's
// Milliseconds() truncates to an integer, which would drop sub-millisecond
// samples to zero and understate the histogram sum.
func DurationMillis(d time.Duration) float64 {
	return float64(d.Nanoseconds()) / 1e6
}
