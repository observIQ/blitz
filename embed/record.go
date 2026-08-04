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
//
// Resource and Attributes describe the entity emitting the record and
// the per-record dimensions, following OpenTelemetry semantics:
//
//   - Resource identifies the source (host, process, module). Per-record
//     Resource entries take precedence over `embed.Host.Resource` on key
//     conflict; `embed.Host.Resource` provides host-process-wide ambient
//     values that no individual generator should hardcode (deployment
//     ID, environment name, etc.).
//   - Attributes are key-value pairs the generator knows about this
//     specific record. Values are `any` to accommodate strings, ints,
//     bools, slices, and other JSON-compatible types.
//
// Both maps may be nil (treated as empty by consumers). Generators
// SHOULD populate at minimum `host.name` and `telemetry.source` (the
// module identifier — `apache`, `fix`, `wel`, etc.) in Resource, plus
// any format / version / module-specific dimensions the generator
// internally knows.
type LogRecordMetadata struct {
	Timestamp  time.Time
	Severity   string
	Attributes map[string]any
	Resource   map[string]any
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

	// Metadata carries timestamp, per-record Attributes, and Resource.
	// See MetricPointMetadata for the conventions.
	Metadata MetricPointMetadata
}

// MetricPointMetadata is the metadata for a metric data point. See
// LogRecordMetadata's doc comment for the Resource/Attributes semantics
// (host.name, telemetry.source, per-record dimensions, etc.) — they
// apply identically here.
//
// Attribute values are typed as map[string]string (rather than
// map[string]any used for logs and spans) to match the OpenTelemetry
// metric-attribute typing convention.
type MetricPointMetadata struct {
	Timestamp  time.Time
	Attributes map[string]string
	Resource   map[string]any
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
	// StatusCode is the span status code (0=Unset, 1=Ok, 2=Error).
	StatusCode int
	// StatusMessage is the span status message.
	StatusMessage string

	// Metadata carries per-record Attributes and Resource. See
	// SpanMetadata for the conventions.
	Metadata SpanMetadata
}

// SpanMetadata is the metadata for a trace span. See LogRecordMetadata's
// doc comment for Resource/Attributes semantics (host.name,
// telemetry.source, per-record dimensions, etc.) — they apply
// identically here.
//
// Attribute values are typed as map[string]any to match the
// OpenTelemetry span-attribute typing convention (strings, ints, bools,
// arrays).
type SpanMetadata struct {
	Attributes map[string]any
	Resource   map[string]any
}
