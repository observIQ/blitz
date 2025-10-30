package output

import (
	"context"
	"time"
)

type LogRecord struct {
	// Message is the raw log message
	Message string

	// ParseFunc is an optional function that will be
	// used by some outputs to parse the message to a
	// map[string]any structure.
	ParseFunc func(message string) (map[string]any, error)

	// Metadata is the metadata for a log record.
	Metadata LogRecordMetadata
}

// LogRecordMetadata is the metadata for a log record.
type LogRecordMetadata struct {
	Timestamp time.Time
	Severity  string
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
