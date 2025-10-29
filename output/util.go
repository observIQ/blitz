package output

import "time"

// int64ToUint64 safely converts an int64 timestamp to uint64.
// UnixNano() returns int64 but OTLP protobuf requires uint64.
// Since timestamps are always non-negative (nanoseconds since Unix epoch),
// this conversion is safe. However, we validate to satisfy static analysis tools.
func int64ToUint64(nanos int64) uint64 {
	if nanos < 0 {
		// This should never happen with valid timestamps, but handle it gracefully
		return 0
	}
	return uint64(nanos)
}

// timeToUnixNanoUint64 converts a time.Time to uint64 nanoseconds.
// This is a convenience wrapper around int64ToUint64 for better readability.
func timeToUnixNanoUint64(t time.Time) uint64 {
	return int64ToUint64(t.UnixNano())
}
