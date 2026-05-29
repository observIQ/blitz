package otlpgrpc

import (
	"testing"
	"time"

	"github.com/observiq/blitz/output"
	"github.com/observiq/blitz/telemetry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metricspb "go.opentelemetry.io/proto/otlp/metrics/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
	"go.uber.org/zap/zaptest"
)

func TestConvertMetricRecord_Gauge(t *testing.T) {
	v := 42.5
	rec := output.MetricRecord{
		Name:        "cpu.usage",
		Description: "CPU usage",
		Unit:        "percent",
		Type:        output.MetricTypeGauge,
		DoubleValue: &v,
		Metadata: output.MetricPointMetadata{
			Timestamp:  time.Now(),
			Attributes: map[string]string{"host": "test"},
		},
	}

	m := convertMetricRecord(rec)
	assert.Equal(t, "cpu.usage", m.Name)
	assert.Equal(t, "CPU usage", m.Description)
	assert.Equal(t, "percent", m.Unit)

	gauge := m.GetGauge()
	require.NotNil(t, gauge)
	require.Len(t, gauge.DataPoints, 1)
	assert.Equal(t, 42.5, gauge.DataPoints[0].GetAsDouble())
}

func TestConvertMetricRecord_GaugeInt(t *testing.T) {
	v := int64(100)
	rec := output.MetricRecord{
		Name:     "memory.usage",
		Type:     output.MetricTypeGauge,
		IntValue: &v,
		Metadata: output.MetricPointMetadata{Timestamp: time.Now()},
	}

	m := convertMetricRecord(rec)
	gauge := m.GetGauge()
	require.NotNil(t, gauge)
	assert.Equal(t, int64(100), gauge.DataPoints[0].GetAsInt())
}

func TestConvertMetricRecord_Sum(t *testing.T) {
	v := 1500.0
	rec := output.MetricRecord{
		Name:        "disk.io",
		Type:        output.MetricTypeSum,
		DoubleValue: &v,
		Metadata:    output.MetricPointMetadata{Timestamp: time.Now()},
	}

	m := convertMetricRecord(rec)
	sum := m.GetSum()
	require.NotNil(t, sum)
	assert.False(t, sum.IsMonotonic)
	assert.Equal(t, metricspb.AggregationTemporality_AGGREGATION_TEMPORALITY_CUMULATIVE, sum.AggregationTemporality)
}

func TestConvertMetricRecord_Counter(t *testing.T) {
	v := int64(42)
	rec := output.MetricRecord{
		Name:     "requests.total",
		Type:     output.MetricTypeCounter,
		IntValue: &v,
		Metadata: output.MetricPointMetadata{Timestamp: time.Now()},
	}

	m := convertMetricRecord(rec)
	sum := m.GetSum()
	require.NotNil(t, sum)
	assert.True(t, sum.IsMonotonic)
}

func TestConvertMetricRecord_Histogram(t *testing.T) {
	rec := output.MetricRecord{
		Name:                  "request.duration",
		Type:                  output.MetricTypeHistogram,
		HistogramCount:        100,
		HistogramSum:          1500.5,
		HistogramMin:          0.5,
		HistogramMax:          50.0,
		HistogramBucketBounds: []float64{1, 5, 10, 25, 50},
		HistogramBucketCounts: []uint64{10, 30, 40, 15, 5},
		Metadata:              output.MetricPointMetadata{Timestamp: time.Now()},
	}

	m := convertMetricRecord(rec)
	hist := m.GetHistogram()
	require.NotNil(t, hist)
	require.Len(t, hist.DataPoints, 1)
	dp := hist.DataPoints[0]
	assert.Equal(t, uint64(100), dp.Count)
	assert.Equal(t, 1500.5, *dp.Sum)
	assert.Equal(t, 0.5, *dp.Min)
	assert.Equal(t, 50.0, *dp.Max)
	assert.Equal(t, []float64{1, 5, 10, 25, 50}, dp.ExplicitBounds)
	assert.Equal(t, []uint64{10, 30, 40, 15, 5}, dp.BucketCounts)
}

func TestConvertTraceRecord(t *testing.T) {
	start := time.Now()
	end := start.Add(100 * time.Millisecond)

	rec := output.TraceRecord{
		TraceID:       "abcdef0123456789abcdef0123456789",
		SpanID:        "0123456789abcdef",
		ParentSpanID:  "fedcba9876543210",
		Name:          "GET /api/users",
		Kind:          output.SpanKindServer,
		StartTime:     start,
		EndTime:       end,
		StatusCode:    0,
		StatusMessage: "",
		Metadata: output.SpanMetadata{
			Attributes: map[string]any{"http.method": "GET", "http.status_code": 200},
		},
	}

	span := convertTraceRecord(rec)
	assert.Equal(t, "GET /api/users", span.Name)
	assert.Equal(t, tracepb.Span_SPAN_KIND_SERVER, span.Kind)
	assert.NotEmpty(t, span.TraceId)
	assert.NotEmpty(t, span.SpanId)
	assert.NotEmpty(t, span.ParentSpanId)
	assert.Len(t, span.Attributes, 2)
	assert.Equal(t, tracepb.Status_StatusCode(0), span.Status.Code)
}

func TestConvertSpanKind(t *testing.T) {
	tests := []struct {
		input    output.SpanKind
		expected tracepb.Span_SpanKind
	}{
		{output.SpanKindInternal, tracepb.Span_SPAN_KIND_INTERNAL},
		{output.SpanKindServer, tracepb.Span_SPAN_KIND_SERVER},
		{output.SpanKindClient, tracepb.Span_SPAN_KIND_CLIENT},
		{output.SpanKindProducer, tracepb.Span_SPAN_KIND_PRODUCER},
		{output.SpanKindConsumer, tracepb.Span_SPAN_KIND_CONSUMER},
		{output.SpanKind("unknown"), tracepb.Span_SPAN_KIND_UNSPECIFIED},
	}

	for _, tc := range tests {
		assert.Equal(t, tc.expected, convertSpanKind(tc.input))
	}
}

func TestHexToBytes(t *testing.T) {
	b := hexToBytes("abcdef01", 4)
	assert.Equal(t, []byte{0xab, 0xcd, 0xef, 0x01}, b)
}

func TestMetricBatch(t *testing.T) {
	batch := newMetricBatch(2, time.Second)
	assert.True(t, batch.isEmpty())
	assert.False(t, batch.isFull())

	batch.add(&metricspb.Metric{Name: "test1"})
	assert.False(t, batch.isEmpty())
	assert.False(t, batch.isFull())

	batch.add(&metricspb.Metric{Name: "test2"})
	assert.True(t, batch.isFull())

	metrics := batch.getAndClear()
	assert.Len(t, metrics, 2)
	assert.True(t, batch.isEmpty())
	batch.timer.Stop()
}

func TestTraceBatch(t *testing.T) {
	batch := newTraceBatch(2, time.Second)
	assert.True(t, batch.isEmpty())

	batch.add(&tracepb.Span{Name: "span1"})
	batch.add(&tracepb.Span{Name: "span2"})
	assert.True(t, batch.isFull())

	spans := batch.getAndClear()
	assert.Len(t, spans, 2)
	assert.True(t, batch.isEmpty())
	batch.timer.Stop()
}

func TestOTLPGrpc_SupportedTelemetry(t *testing.T) {
	logger := zaptest.NewLogger(t)
	otlp, err := New(logger, WithHost("localhost"), WithPort("4317"))
	require.NoError(t, err)

	types := otlp.SupportedTelemetry()
	assert.Equal(t, []telemetry.Type{telemetry.Logs, telemetry.Metrics, telemetry.Traces}, types)

	otlp.Stop(t.Context())
}
