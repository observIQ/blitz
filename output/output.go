package output

import (
	"context"
	"errors"
	"time"

	"github.com/observiq/blitz/telemetry"
)

// ErrUnsupportedTelemetryType is returned when a writer does not support the requested telemetry type.
var ErrUnsupportedTelemetryType = errors.New("unsupported telemetry type")

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

// MetricType represents the type of metric data point.
type MetricType string

const (
	// MetricTypeGauge represents a gauge metric.
	MetricTypeGauge MetricType = "gauge"
	// MetricTypeSum represents a sum (counter) metric.
	MetricTypeSum MetricType = "sum"
	// MetricTypeCounter represents a monotonic counter metric (OTLP Sum with IsMonotonic=true).
	MetricTypeCounter MetricType = "counter"
	// MetricTypeHistogram represents a histogram metric.
	MetricTypeHistogram MetricType = "histogram"
)

// MetricRecord represents a single metric data point.
type MetricRecord struct {
	// Name is the metric name (e.g. "system.cpu.utilization").
	Name string

	// Description is a human-readable description of the metric.
	Description string

	// Unit is the metric unit (e.g. "s", "By", "1").
	Unit string

	// Type is the metric data point type (gauge or sum).
	Type MetricType

	// IntValue is the integer value for the data point. Exactly one of
	// IntValue or DoubleValue should be set.
	IntValue *int64

	// DoubleValue is the floating-point value for the data point.
	DoubleValue *float64

	// Attributes are key-value pairs associated with this data point.
	Attributes map[string]string

	// ResourceAttributes are key-value pairs associated with the resource
	// that produced this metric (e.g. "service.name", "host.name").
	ResourceAttributes map[string]string

	// Timestamp is when the measurement was taken.
	Timestamp time.Time

	// Histogram fields (only used when Type is MetricTypeHistogram).

	// HistogramCount is the total number of observations.
	HistogramCount *uint64
	// HistogramSum is the sum of all observed values.
	HistogramSum *float64
	// HistogramMin is the minimum observed value.
	HistogramMin *float64
	// HistogramMax is the maximum observed value.
	HistogramMax *float64
	// HistogramBucketBounds are the explicit bucket boundaries.
	HistogramBucketBounds []float64
	// HistogramBucketCounts are the counts for each bucket
	// (length = len(HistogramBucketBounds) + 1).
	HistogramBucketCounts []uint64
}

// Writer can consume log and metric records.
type Writer interface {
	// Write writes the data to the output.
	Write(ctx context.Context, data LogRecord) error

	// WriteMetric writes a metric data point to the output.
	WriteMetric(ctx context.Context, data MetricRecord) error
}

// Output is the interface for outputting data.
type Output interface {
	Writer

	// SupportedTelemetry returns the telemetry types this output can send.
	SupportedTelemetry() []telemetry.Type

	// Stop stops the output.
	Stop(ctx context.Context) error
}
