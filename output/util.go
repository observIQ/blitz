package output

import "time"

// Int64ToUint64 safely converts an int64 timestamp to uint64.
// UnixNano() returns int64 but OTLP protobuf requires uint64.
// Since timestamps are always non-negative (nanoseconds since Unix epoch),
// this conversion is safe. However, we validate to satisfy static analysis tools.
func Int64ToUint64(nanos int64) uint64 {
	if nanos < 0 {
		// This should never happen with valid timestamps, but handle it gracefully
		return 0
	}
	return uint64(nanos)
}

// TimeToUnixNanoUint64 converts a time.Time to uint64 nanoseconds.
// This is a convenience wrapper around Int64ToUint64 for better readability.
func TimeToUnixNanoUint64(t time.Time) uint64 {
	return Int64ToUint64(t.UnixNano())
}
