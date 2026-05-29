package output

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestMetricTypeConstants(t *testing.T) {
	assert.Equal(t, MetricType("gauge"), MetricTypeGauge)
	assert.Equal(t, MetricType("sum"), MetricTypeSum)
	assert.Equal(t, MetricType("counter"), MetricTypeCounter)
	assert.Equal(t, MetricType("histogram"), MetricTypeHistogram)
}

func TestSpanKindConstants(t *testing.T) {
	assert.Equal(t, SpanKind("internal"), SpanKindInternal)
	assert.Equal(t, SpanKind("server"), SpanKindServer)
	assert.Equal(t, SpanKind("client"), SpanKindClient)
	assert.Equal(t, SpanKind("producer"), SpanKindProducer)
	assert.Equal(t, SpanKind("consumer"), SpanKindConsumer)
}

func TestMetricRecordIntValue(t *testing.T) {
	v := int64(42)
	rec := MetricRecord{
		Name:     "cpu.usage",
		Type:     MetricTypeGauge,
		IntValue: &v,
		Metadata: MetricPointMetadata{
			Timestamp: time.Now(),
			Attributes: map[string]string{
				"host": "test-host",
			},
		},
	}

	assert.NotNil(t, rec.IntValue)
	assert.Equal(t, int64(42), *rec.IntValue)
	assert.Nil(t, rec.DoubleValue)
	assert.Equal(t, "cpu.usage", rec.Name)
	assert.Equal(t, MetricTypeGauge, rec.Type)
}

func TestMetricRecordDoubleValue(t *testing.T) {
	v := 3.14
	rec := MetricRecord{
		Name:        "cpu.percent",
		Type:        MetricTypeGauge,
		DoubleValue: &v,
		Metadata:    MetricPointMetadata{Timestamp: time.Now()},
	}

	assert.NotNil(t, rec.DoubleValue)
	assert.Equal(t, 3.14, *rec.DoubleValue)
	assert.Nil(t, rec.IntValue)
}

func TestMetricRecordHistogram(t *testing.T) {
	rec := MetricRecord{
		Name:                  "request.duration",
		Type:                  MetricTypeHistogram,
		HistogramCount:        100,
		HistogramSum:          1500.5,
		HistogramMin:          0.5,
		HistogramMax:          50.0,
		HistogramBucketBounds: []float64{1, 5, 10, 25, 50},
		HistogramBucketCounts: []uint64{10, 30, 40, 15, 5},
		Metadata:              MetricPointMetadata{Timestamp: time.Now()},
	}

	assert.Equal(t, MetricTypeHistogram, rec.Type)
	assert.Equal(t, uint64(100), rec.HistogramCount)
	assert.Equal(t, 1500.5, rec.HistogramSum)
	assert.Equal(t, 0.5, rec.HistogramMin)
	assert.Equal(t, 50.0, rec.HistogramMax)
	assert.Len(t, rec.HistogramBucketBounds, 5)
	assert.Len(t, rec.HistogramBucketCounts, 5)
}

func TestTraceRecord(t *testing.T) {
	start := time.Now()
	end := start.Add(100 * time.Millisecond)

	rec := TraceRecord{
		TraceID:       "abc123",
		SpanID:        "span456",
		ParentSpanID:  "parent789",
		Name:          "HTTP GET /api/users",
		Kind:          SpanKindServer,
		StartTime:     start,
		EndTime:       end,
		StatusCode:    0,
		StatusMessage: "",
		Metadata: SpanMetadata{
			Attributes: map[string]any{"http.method": "GET", "http.status_code": 200},
		},
	}

	assert.Equal(t, "abc123", rec.TraceID)
	assert.Equal(t, "span456", rec.SpanID)
	assert.Equal(t, "parent789", rec.ParentSpanID)
	assert.Equal(t, SpanKindServer, rec.Kind)
	assert.Equal(t, start, rec.StartTime)
	assert.Equal(t, end, rec.EndTime)
	assert.Equal(t, "GET", rec.Metadata.Attributes["http.method"])
}

func TestErrUnsupportedTelemetryType(t *testing.T) {
	assert.NotNil(t, ErrUnsupportedTelemetryType)
	assert.Error(t, ErrUnsupportedTelemetryType)
	assert.Contains(t, ErrUnsupportedTelemetryType.Error(), "unsupported telemetry type")
}

// Compile-time interface assertions
var _ MetricWriter = (*metricWriterImpl)(nil)
var _ TraceWriter = (*traceWriterImpl)(nil)

type metricWriterImpl struct{}

func (m *metricWriterImpl) WriteMetric(_ context.Context, _ MetricRecord) error { return nil }

type traceWriterImpl struct{}

func (t *traceWriterImpl) WriteTrace(_ context.Context, _ TraceRecord) error { return nil }
