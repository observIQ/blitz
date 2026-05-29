package embed_test

import (
	"testing"
	"time"

	"github.com/observiq/blitz/embed"
)

func TestLogRecordZeroValueIsUsable(t *testing.T) {
	var rec embed.LogRecord
	if rec.Message != "" {
		t.Errorf("zero LogRecord Message should be empty, got %q", rec.Message)
	}
	if !rec.Metadata.Timestamp.IsZero() {
		t.Errorf("zero LogRecord Metadata.Timestamp should be zero")
	}
	if rec.Metadata.Attributes != nil {
		t.Errorf("zero LogRecord Metadata.Attributes should be nil, got %v", rec.Metadata.Attributes)
	}
	if rec.Metadata.Resource != nil {
		t.Errorf("zero LogRecord Metadata.Resource should be nil, got %v", rec.Metadata.Resource)
	}
}

func TestLogRecordMetadataCarriesResourceAndAttributes(t *testing.T) {
	rec := embed.LogRecord{
		Message: "GET / 200",
		Metadata: embed.LogRecordMetadata{
			Severity: "INFO",
			Resource: map[string]string{
				"host.name":        "web-01",
				"telemetry.source": "apache",
				"apache.format":    "common",
			},
			Attributes: map[string]any{
				"http.status_code": 200,
				"http.method":      "GET",
			},
		},
	}
	if got := rec.Metadata.Resource["host.name"]; got != "web-01" {
		t.Errorf("Resource[host.name]: want web-01, got %q", got)
	}
	if got := rec.Metadata.Resource["telemetry.source"]; got != "apache" {
		t.Errorf("Resource[telemetry.source]: want apache, got %q", got)
	}
	if got := rec.Metadata.Attributes["http.status_code"]; got != 200 {
		t.Errorf("Attributes[http.status_code]: want 200, got %v", got)
	}
}

func TestMetricPointIntValueAndDoubleValueAreSeparatePointers(t *testing.T) {
	i := int64(42)
	d := 3.14
	point := embed.MetricPoint{
		Name:        "test",
		Type:        embed.MetricTypeGauge,
		IntValue:    &i,
		DoubleValue: &d,
	}
	if *point.IntValue != 42 {
		t.Errorf("IntValue: want 42, got %d", *point.IntValue)
	}
	if *point.DoubleValue != 3.14 {
		t.Errorf("DoubleValue: want 3.14, got %f", *point.DoubleValue)
	}
}

func TestSpanCarriesStartAndEndTimes(t *testing.T) {
	start := time.Date(2026, 5, 19, 12, 0, 0, 0, time.UTC)
	end := start.Add(250 * time.Millisecond)
	span := embed.Span{
		TraceID:   "abc",
		SpanID:    "def",
		Name:      "request",
		Kind:      embed.SpanKindServer,
		StartTime: start,
		EndTime:   end,
	}
	if span.EndTime.Sub(span.StartTime) != 250*time.Millisecond {
		t.Errorf("expected 250ms duration, got %v", span.EndTime.Sub(span.StartTime))
	}
	if span.Kind != embed.SpanKindServer {
		t.Errorf("Kind: want %q, got %q", embed.SpanKindServer, span.Kind)
	}
}
