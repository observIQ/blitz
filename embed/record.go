package embed

import "time"

// LogRecord represents a single log entry.
type LogRecord struct {
	// Message is the raw log message.
	Message string

	// ParseFunc is an optional function that some outputs use to parse
	// the message into a map[string]any structure.
	ParseFunc func(message string) (map[string]any, error)

	// Metadata is the metadata for a log record.
	Metadata LogRecordMetadata
}

// LogRecordMetadata is the metadata for a log record.
type LogRecordMetadata struct {
	Timestamp time.Time
	Severity  string
}

// MetricType represents the type of a metric data point.
type MetricType string

const (
	// MetricTypeGauge represents a gauge metric.
	MetricTypeGauge MetricType = "gauge"
	// MetricTypeSum represents a sum metric.
	MetricTypeSum MetricType = "sum"
	// MetricTypeCounter represents a counter metric.
	MetricTypeCounter MetricType = "counter"
	// MetricTypeHistogram represents a histogram metric.
	MetricTypeHistogram MetricType = "histogram"
)

// MetricPoint represents a single metric data point.
type MetricPoint struct {
	// Name is the metric name.
	Name string
	// Description is the metric description.
	Description string
	// Unit is the metric unit.
	Unit string
	// Type is the metric type (gauge, sum, counter, histogram).
	Type MetricType
	// IntValue is the integer value for gauge/sum/counter metrics.
	// Mutually exclusive with DoubleValue.
	IntValue *int64
	// DoubleValue is the floating-point value for gauge/sum/counter metrics.
	// Mutually exclusive with IntValue.
	DoubleValue *float64
	// Timestamp is the time of the metric data point.
	Timestamp time.Time
	// Attributes are key-value pairs associated with the metric.
	Attributes map[string]string
	// Resource is the resource associated with the metric.
	Resource map[string]string

	// HistogramCount is the total count of observations.
	HistogramCount uint64
	// HistogramSum is the sum of all observations.
	HistogramSum float64
	// HistogramMin is the minimum observed value.
	HistogramMin float64
	// HistogramMax is the maximum observed value.
	HistogramMax float64
	// HistogramBucketBounds are the explicit bucket boundaries.
	HistogramBucketBounds []float64
	// HistogramBucketCounts are the counts for each bucket.
	HistogramBucketCounts []uint64
}

// SpanKind represents the kind of a trace span.
type SpanKind string

const (
	// SpanKindInternal represents an internal span.
	SpanKindInternal SpanKind = "internal"
	// SpanKindServer represents a server span.
	SpanKindServer SpanKind = "server"
	// SpanKindClient represents a client span.
	SpanKindClient SpanKind = "client"
	// SpanKindProducer represents a producer span.
	SpanKindProducer SpanKind = "producer"
	// SpanKindConsumer represents a consumer span.
	SpanKindConsumer SpanKind = "consumer"
)

// Span represents a single trace span.
type Span struct {
	// TraceID is the trace identifier.
	TraceID string
	// SpanID is the span identifier.
	SpanID string
	// ParentSpanID is the parent span identifier (empty for root spans).
	ParentSpanID string
	// Name is the span name.
	Name string
	// Kind is the span kind.
	Kind SpanKind
	// StartTime is the start time of the span.
	StartTime time.Time
	// EndTime is the end time of the span.
	EndTime time.Time
	// Attributes are key-value pairs associated with the span.
	Attributes map[string]any
	// StatusCode is the span status code (0=Unset, 1=Ok, 2=Error).
	StatusCode int
	// StatusMessage is the span status message.
	StatusMessage string
}
