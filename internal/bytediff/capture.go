package bytediff

import (
	"context"
	"sync"

	"github.com/observiq/blitz/output"
	"github.com/observiq/blitz/telemetry"
)

// LogCapture is a LogWriter that records every LogRecord written to it.
// Use it as the destination when capturing a generator's output for a
// byte-diff comparison.
type LogCapture struct {
	mu      sync.Mutex
	records []output.LogRecord
}

// NewLogCapture returns an empty LogCapture.
func NewLogCapture() *LogCapture {
	return &LogCapture{}
}

// WriteLog appends the record to the capture buffer. Safe for concurrent
// calls from generator workers.
func (c *LogCapture) WriteLog(_ context.Context, data output.LogRecord) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.records = append(c.records, data)
	return nil
}

// Records returns a copy of every record captured so far. The returned
// slice is independent of the capture's internal state.
func (c *LogCapture) Records() []output.LogRecord {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]output.LogRecord, len(c.records))
	copy(out, c.records)
	return out
}

// Messages returns just the Message field of every captured record, in
// order. Most byte-diff regression checks compare Message values, since
// that's where wire-format bytes land.
func (c *LogCapture) Messages() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]string, len(c.records))
	for i, rec := range c.records {
		out[i] = rec.Message
	}
	return out
}

// Reset empties the capture buffer.
func (c *LogCapture) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.records = c.records[:0]
}

// Stop is a no-op; the capture holds no resources.
func (c *LogCapture) Stop(context.Context) error { return nil }

// SupportedTelemetry reports that the capture accepts logs.
func (c *LogCapture) SupportedTelemetry() []telemetry.Type {
	return []telemetry.Type{telemetry.Logs}
}
