package output

import (
	"context"
)

type LogRecord struct {
	// Message is the raw log message
	Message []byte

	// ParseFunc is an optional function that will be
	// used by some outputs to parse the message.
	// TODO(jsirianni): Implement to support parsing JSON
	// with OTLP output.
	// ParseFunc func(message []byte) (any, error)
}

// Writer can consume log records.
type Writer interface {
	// Write writes the data to the output.
	Write(ctx context.Context, data LogRecord) error
}

// Output is the interface for outputting data.
type Output interface {
	Writer

	// Stop stops the output.
	Stop(ctx context.Context) error
}
