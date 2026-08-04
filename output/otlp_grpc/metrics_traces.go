package otlpgrpc

import (
	"context"
	"fmt"
	"time"

	"github.com/observiq/blitz/output"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	metricspb "go.opentelemetry.io/proto/otlp/metrics/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
)

// WriteMetric sends a metric record to the OTLP gRPC output for processing by workers.
func (o *OTLPGrpc) WriteMetric(ctx context.Context, data output.MetricRecord) error {
	pbMetric := convertMetricRecord(data)

	select {
	case o.metricChan <- pbMetric:
		output.BlitzOutputEntriesReceivedCounter.Add(ctx, 1, outputType, "metrics")
		return nil
	case <-ctx.Done():
		return fmt.Errorf("context cancelled while waiting to write metric: %w", ctx.Err())
	case <-o.ctx.Done():
		return fmt.Errorf("OTLP gRPC output is shutting down")
	}
}

// WriteTrace sends a trace record to the OTLP gRPC output for processing by workers.
func (o *OTLPGrpc) WriteTrace(ctx context.Context, data output.TraceRecord) error {
	pbSpan := convertTraceRecord(data)

	select {
	case o.traceChan <- pbSpan:
		output.BlitzOutputEntriesReceivedCounter.Add(ctx, 1, outputType, "traces")
		return nil
	case <-ctx.Done():
		return fmt.Errorf("context cancelled while waiting to write trace: %w", ctx.Err())
	case <-o.ctx.Done():
		return fmt.Errorf("OTLP gRPC output is shutting down")
	}
}

// convertMetricRecord converts an output.MetricRecord to an OTLP metric protobuf.
func convertMetricRecord(rec output.MetricRecord) *metricspb.Metric {
	ts := uint64(rec.Metadata.Timestamp.UnixNano()) // #nosec G115 -- Unix nanoseconds are always non-negative for current epoch

	m := &metricspb.Metric{
		Name:        rec.Name,
		Description: rec.Description,
		Unit:        rec.Unit,
	}

	attrs := stringMapToKeyValues(rec.Metadata.Attributes)

	switch rec.Type {
	case output.MetricTypeGauge:
		dp := &metricspb.NumberDataPoint{
			TimeUnixNano: ts,
			Attributes:   attrs,
		}
		setNumberDataPointValue(dp, rec)
		m.Data = &metricspb.Metric_Gauge{
			Gauge: &metricspb.Gauge{
				DataPoints: []*metricspb.NumberDataPoint{dp},
			},
		}

	case output.MetricTypeSum, output.MetricTypeCounter:
		dp := &metricspb.NumberDataPoint{
			TimeUnixNano: ts,
			Attributes:   attrs,
		}
		setNumberDataPointValue(dp, rec)
		isMonotonic := rec.Type == output.MetricTypeCounter
		m.Data = &metricspb.Metric_Sum{
			Sum: &metricspb.Sum{
				DataPoints:             []*metricspb.NumberDataPoint{dp},
				AggregationTemporality: metricspb.AggregationTemporality_AGGREGATION_TEMPORALITY_CUMULATIVE,
				IsMonotonic:            isMonotonic,
			},
		}

	case output.MetricTypeHistogram:
		dp := &metricspb.HistogramDataPoint{
			TimeUnixNano:   ts,
			Attributes:     attrs,
			Count:          rec.HistogramCount,
			Sum:            &rec.HistogramSum,
			Min:            &rec.HistogramMin,
			Max:            &rec.HistogramMax,
			ExplicitBounds: rec.HistogramBucketBounds,
			BucketCounts:   rec.HistogramBucketCounts,
		}
		m.Data = &metricspb.Metric_Histogram{
			Histogram: &metricspb.Histogram{
				DataPoints:             []*metricspb.HistogramDataPoint{dp},
				AggregationTemporality: metricspb.AggregationTemporality_AGGREGATION_TEMPORALITY_CUMULATIVE,
			},
		}
	}

	return m
}

func setNumberDataPointValue(dp *metricspb.NumberDataPoint, rec output.MetricRecord) {
	if rec.IntValue != nil {
		dp.Value = &metricspb.NumberDataPoint_AsInt{AsInt: *rec.IntValue}
	} else if rec.DoubleValue != nil {
		dp.Value = &metricspb.NumberDataPoint_AsDouble{AsDouble: *rec.DoubleValue}
	}
}

// convertTraceRecord converts an output.TraceRecord to an OTLP span protobuf.
func convertTraceRecord(rec output.TraceRecord) *tracepb.Span {
	span := &tracepb.Span{
		TraceId:           hexToBytes(rec.TraceID, 16),
		SpanId:            hexToBytes(rec.SpanID, 8),
		Name:              rec.Name,
		Kind:              convertSpanKind(rec.Kind),
		StartTimeUnixNano: uint64(rec.StartTime.UnixNano()), // #nosec G115 -- Unix nanoseconds are always non-negative for current epoch
		EndTimeUnixNano:   uint64(rec.EndTime.UnixNano()),   // #nosec G115 -- Unix nanoseconds are always non-negative for current epoch
		Status: &tracepb.Status{
			Code:    tracepb.Status_StatusCode(rec.StatusCode), // #nosec G115 -- StatusCode values are bounded enum constants (0/1/2)
			Message: rec.StatusMessage,
		},
	}

	if rec.ParentSpanID != "" {
		span.ParentSpanId = hexToBytes(rec.ParentSpanID, 8)
	}

	if len(rec.Metadata.Attributes) > 0 {
		span.Attributes = anyMapToKeyValues(rec.Metadata.Attributes)
	}

	return span
}

func convertSpanKind(kind output.SpanKind) tracepb.Span_SpanKind {
	switch kind {
	case output.SpanKindInternal:
		return tracepb.Span_SPAN_KIND_INTERNAL
	case output.SpanKindServer:
		return tracepb.Span_SPAN_KIND_SERVER
	case output.SpanKindClient:
		return tracepb.Span_SPAN_KIND_CLIENT
	case output.SpanKindProducer:
		return tracepb.Span_SPAN_KIND_PRODUCER
	case output.SpanKindConsumer:
		return tracepb.Span_SPAN_KIND_CONSUMER
	default:
		return tracepb.Span_SPAN_KIND_UNSPECIFIED
	}
}

func stringMapToKeyValues(m map[string]string) []*commonpb.KeyValue {
	if len(m) == 0 {
		return nil
	}
	kvs := make([]*commonpb.KeyValue, 0, len(m))
	for k, v := range m {
		kvs = append(kvs, &commonpb.KeyValue{
			Key:   k,
			Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: v}},
		})
	}
	return kvs
}

func anyMapToKeyValues(m map[string]any) []*commonpb.KeyValue {
	if len(m) == 0 {
		return nil
	}
	kvs := make([]*commonpb.KeyValue, 0, len(m))
	for k, v := range m {
		av := toAnyValueSimple(v)
		if av != nil {
			kvs = append(kvs, &commonpb.KeyValue{Key: k, Value: av})
		}
	}
	return kvs
}

func toAnyValueSimple(v any) *commonpb.AnyValue {
	switch x := v.(type) {
	case string:
		return &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: x}}
	case int:
		return &commonpb.AnyValue{Value: &commonpb.AnyValue_IntValue{IntValue: int64(x)}}
	case int64:
		return &commonpb.AnyValue{Value: &commonpb.AnyValue_IntValue{IntValue: x}}
	case float64:
		return &commonpb.AnyValue{Value: &commonpb.AnyValue_DoubleValue{DoubleValue: x}}
	case bool:
		return &commonpb.AnyValue{Value: &commonpb.AnyValue_BoolValue{BoolValue: x}}
	default:
		return &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: fmt.Sprintf("%v", x)}}
	}
}

func hexToBytes(hex string, size int) []byte {
	b := make([]byte, size)
	for i := 0; i < len(hex) && i/2 < size; i += 2 {
		if i+1 < len(hex) {
			b[i/2] = hexByte(hex[i])<<4 | hexByte(hex[i+1])
		}
	}
	return b
}

func hexByte(c byte) byte {
	switch {
	case c >= '0' && c <= '9':
		return c - '0'
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10
	default:
		return 0
	}
}

// buildMetricRequest builds an OTLP ExportMetricsServiceRequest from prepared metrics.
func buildMetricRequest(metrics []*metricspb.Metric, resource map[string]any) *metricspb.ResourceMetrics {
	resourceAttrs := make([]*commonpb.KeyValue, 0, len(resource)+1)
	resourceAttrs = append(resourceAttrs, &commonpb.KeyValue{
		Key:   "service.name",
		Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "blitz"}},
	})
	resourceAttrs = append(resourceAttrs, anyMapToKeyValues(resource)...)

	return &metricspb.ResourceMetrics{
		Resource: &resourcepb.Resource{
			Attributes: resourceAttrs,
		},
		ScopeMetrics: []*metricspb.ScopeMetrics{
			{
				Metrics: metrics,
			},
		},
	}
}

// buildTraceRequest builds an OTLP ResourceSpans from prepared spans.
func buildTraceRequest(spans []*tracepb.Span) *tracepb.ResourceSpans {
	return &tracepb.ResourceSpans{
		Resource: &resourcepb.Resource{
			Attributes: []*commonpb.KeyValue{
				{
					Key:   "service.name",
					Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "blitz"}},
				},
			},
		},
		ScopeSpans: []*tracepb.ScopeSpans{
			{
				Spans: spans,
			},
		},
	}
}

// metricBatch holds a batch of metrics to be sent
type metricBatch struct {
	metrics []*metricspb.Metric
	maxSize int
	timer   *time.Timer
}

func newMetricBatch(maxSize int, timeout time.Duration) *metricBatch {
	return &metricBatch{
		metrics: make([]*metricspb.Metric, 0, maxSize),
		maxSize: maxSize,
		timer:   time.NewTimer(timeout),
	}
}

func (b *metricBatch) add(m *metricspb.Metric) { b.metrics = append(b.metrics, m) }
func (b *metricBatch) isFull() bool            { return len(b.metrics) >= b.maxSize }
func (b *metricBatch) isEmpty() bool           { return len(b.metrics) == 0 }

func (b *metricBatch) getAndClear() []*metricspb.Metric {
	metrics := b.metrics
	b.metrics = make([]*metricspb.Metric, 0, b.maxSize)
	return metrics
}

// traceBatch holds a batch of spans to be sent
type traceBatch struct {
	spans   []*tracepb.Span
	maxSize int
	timer   *time.Timer
}

func newTraceBatch(maxSize int, timeout time.Duration) *traceBatch {
	return &traceBatch{
		spans:   make([]*tracepb.Span, 0, maxSize),
		maxSize: maxSize,
		timer:   time.NewTimer(timeout),
	}
}

func (b *traceBatch) add(s *tracepb.Span) { b.spans = append(b.spans, s) }
func (b *traceBatch) isFull() bool        { return len(b.spans) >= b.maxSize }
func (b *traceBatch) isEmpty() bool       { return len(b.spans) == 0 }

func (b *traceBatch) getAndClear() []*tracepb.Span {
	spans := b.spans
	b.spans = make([]*tracepb.Span, 0, b.maxSize)
	return spans
}
