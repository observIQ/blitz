package embed

import "context"

// LogConsumer consumes batches of log records produced by blitz modules.
//
// Implementations must be safe for concurrent calls — blitz dispatches
// from worker goroutines and does not serialize consumer invocations.
type LogConsumer interface {
	ConsumeLogs(ctx context.Context, records []LogRecord) error
}

// MetricConsumer consumes batches of metric points produced by blitz modules.
//
// Implementations must be safe for concurrent calls.
type MetricConsumer interface {
	ConsumeMetrics(ctx context.Context, points []MetricPoint) error
}

// TraceConsumer consumes batches of spans produced by blitz modules.
//
// Implementations must be safe for concurrent calls.
type TraceConsumer interface {
	ConsumeTraces(ctx context.Context, spans []Span) error
}
