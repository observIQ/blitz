package output

import (
	"context"
	"errors"

	"github.com/observiq/blitz/embed"
	"github.com/observiq/blitz/telemetry"
)

// ErrUnsupportedTelemetryType is returned when an output does not support
// the requested telemetry type.
var ErrUnsupportedTelemetryType = errors.New("unsupported telemetry type")

// LogRecord aliases embed.LogRecord; defined here for backward
// compatibility with existing output and generator callers. The
// canonical definition lives in the embed package.
type LogRecord = embed.LogRecord

// LogRecordMetadata aliases embed.LogRecordMetadata.
type LogRecordMetadata = embed.LogRecordMetadata

// MetricType aliases embed.MetricType.
type MetricType = embed.MetricType

const (
	// MetricTypeGauge represents a gauge metric.
	MetricTypeGauge = embed.MetricTypeGauge
	// MetricTypeSum represents a sum metric.
	MetricTypeSum = embed.MetricTypeSum
	// MetricTypeCounter represents a counter metric.
	MetricTypeCounter = embed.MetricTypeCounter
	// MetricTypeHistogram represents a histogram metric.
	MetricTypeHistogram = embed.MetricTypeHistogram
)

// MetricRecord aliases embed.MetricPoint. The embed canonical name is
// MetricPoint; this alias preserves the older MetricRecord name for
// existing call sites while later PRs migrate to the new name.
type MetricRecord = embed.MetricPoint

// MetricPointMetadata aliases embed.MetricPointMetadata so callers in
// this package can construct MetricRecord values without depending on
// embed directly.
type MetricPointMetadata = embed.MetricPointMetadata

// SpanKind aliases embed.SpanKind.
type SpanKind = embed.SpanKind

const (
	// SpanKindInternal represents an internal span.
	SpanKindInternal = embed.SpanKindInternal
	// SpanKindServer represents a server span.
	SpanKindServer = embed.SpanKindServer
	// SpanKindClient represents a client span.
	SpanKindClient = embed.SpanKindClient
	// SpanKindProducer represents a producer span.
	SpanKindProducer = embed.SpanKindProducer
	// SpanKindConsumer represents a consumer span.
	SpanKindConsumer = embed.SpanKindConsumer
)

// TraceRecord aliases embed.Span. The embed canonical name is Span; this
// alias preserves the older TraceRecord name for existing call sites.
type TraceRecord = embed.Span

// SpanMetadata aliases embed.SpanMetadata so callers in this package
// can construct TraceRecord values without depending on embed directly.
type SpanMetadata = embed.SpanMetadata

// Writer can consume log records.
type Writer interface {
	// Write writes the data to the output.
	Write(ctx context.Context, data LogRecord) error
}

// MetricWriter can consume metric records.
type MetricWriter interface {
	// WriteMetric writes a metric record to the output.
	WriteMetric(ctx context.Context, data MetricRecord) error
}

// TraceWriter can consume trace records.
type TraceWriter interface {
	// WriteTrace writes a trace record to the output.
	WriteTrace(ctx context.Context, data TraceRecord) error
}

// Output is the interface for outputting data.
type Output interface {
	Writer

	// Stop stops the output.
	Stop(ctx context.Context) error

	// SupportedTelemetry returns the telemetry types this output can consume.
	SupportedTelemetry() []telemetry.Type
}
