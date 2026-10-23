package output_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/observiq/blitz/embed"
	"github.com/observiq/blitz/generator/count"
	"github.com/observiq/blitz/internal/counttest"
	"github.com/observiq/blitz/output"
	"github.com/observiq/blitz/telemetry"
	"github.com/stretchr/testify/require"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// TestStartSendSpan_gated covers both branches of the shared output send-span
// helper: off yields a non-recording span, on records a named span.
func TestStartSendSpan_gated(t *testing.T) {
	rec := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(rec))

	off := embed.TelemetrySettings{TracerProvider: tp, PerBatchSpans: false}
	_, span := output.StartSendSpan(context.Background(), off, "blitz.output.test.send")
	span.End()
	require.Empty(t, rec.Ended(), "no span expected when PerBatchSpans is off")

	on := embed.TelemetrySettings{TracerProvider: tp, PerBatchSpans: true}
	_, span = output.StartSendSpan(context.Background(), on, "blitz.output.test.send")
	span.End()
	ended := rec.Ended()
	require.Len(t, ended, 1)
	require.Equal(t, "blitz.output.test.send", ended[0].Name())
}

type recordingWriter struct {
	mu      sync.Mutex
	records []output.LogRecord
	err     error
}

func (w *recordingWriter) WriteLog(_ context.Context, rec output.LogRecord) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.err != nil {
		return w.err
	}
	w.records = append(w.records, rec)
	return nil
}

func TestWriterAsLogConsumerPushesEachRecord(t *testing.T) {
	w := &recordingWriter{}
	c := output.WriterAsLogConsumer(w, embed.NopTelemetry())

	batch := []embed.LogRecord{
		{Message: "one"},
		{Message: "two"},
		{Message: "three"},
	}
	if err := c.ConsumeLogs(context.Background(), batch); err != nil {
		t.Fatalf("ConsumeLogs: %v", err)
	}
	if got, want := len(w.records), 3; got != want {
		t.Fatalf("recorded %d, want %d", got, want)
	}
	for i, want := range []string{"one", "two", "three"} {
		if w.records[i].Message != want {
			t.Errorf("record %d: %q, want %q", i, w.records[i].Message, want)
		}
	}
}

func TestWriterAsLogConsumerPerBatchSpanWhenEnabled(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))

	w := &recordingWriter{}
	c := output.WriterAsLogConsumer(w, embed.TelemetrySettings{TracerProvider: tp, PerBatchSpans: true})

	batch := []embed.LogRecord{
		{Message: "one"},
		{Message: "two"},
	}
	if err := c.ConsumeLogs(context.Background(), batch); err != nil {
		t.Fatalf("ConsumeLogs: %v", err)
	}

	spans := exporter.GetSpans()
	if got, want := len(spans), 1; got != want {
		t.Fatalf("exported %d spans, want %d", got, want)
	}
	if got, want := spans[0].Name, "blitz.emit.logs"; got != want {
		t.Errorf("span name %q, want %q", got, want)
	}
}

func TestWriterAsLogConsumerNoSpanWhenDisabled(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))

	w := &recordingWriter{}
	c := output.WriterAsLogConsumer(w, embed.TelemetrySettings{TracerProvider: tp})

	batch := []embed.LogRecord{
		{Message: "one"},
		{Message: "two"},
	}
	if err := c.ConsumeLogs(context.Background(), batch); err != nil {
		t.Fatalf("ConsumeLogs: %v", err)
	}

	if got := exporter.GetSpans(); len(got) != 0 {
		t.Fatalf("exported %d spans, want 0", len(got))
	}
}

func TestWriterAsLogConsumerStopsOnFirstError(t *testing.T) {
	wantErr := errors.New("boom")
	w := &recordingWriter{err: wantErr}
	c := output.WriterAsLogConsumer(w, embed.NopTelemetry())

	err := c.ConsumeLogs(context.Background(), []embed.LogRecord{
		{Message: "one"},
		{Message: "two"},
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected wrapped wantErr, got %v", err)
	}
}

type recordingMetricWriter struct {
	mu      sync.Mutex
	records []output.MetricRecord
	err     error
}

func (w *recordingMetricWriter) WriteMetric(_ context.Context, rec output.MetricRecord) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.err != nil {
		return w.err
	}
	w.records = append(w.records, rec)
	return nil
}

func TestWriterAsMetricConsumerPushesEachPoint(t *testing.T) {
	w := &recordingMetricWriter{}
	c := output.WriterAsMetricConsumer(w, embed.NopTelemetry())

	batch := []embed.MetricPoint{
		{Name: "one"},
		{Name: "two"},
		{Name: "three"},
	}
	if err := c.ConsumeMetrics(context.Background(), batch); err != nil {
		t.Fatalf("ConsumeMetrics: %v", err)
	}
	if got, want := len(w.records), 3; got != want {
		t.Fatalf("recorded %d, want %d", got, want)
	}
	for i, want := range []string{"one", "two", "three"} {
		if w.records[i].Name != want {
			t.Errorf("point %d: %q, want %q", i, w.records[i].Name, want)
		}
	}
}

func TestWriterAsMetricConsumerStopsOnFirstError(t *testing.T) {
	wantErr := errors.New("metric boom")
	w := &recordingMetricWriter{err: wantErr}
	c := output.WriterAsMetricConsumer(w, embed.NopTelemetry())

	err := c.ConsumeMetrics(context.Background(), []embed.MetricPoint{
		{Name: "one"},
		{Name: "two"},
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected wantErr, got %v", err)
	}
}

type recordingTraceWriter struct {
	mu    sync.Mutex
	spans []output.TraceRecord
	err   error
}

func (w *recordingTraceWriter) WriteTrace(_ context.Context, rec output.TraceRecord) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.err != nil {
		return w.err
	}
	w.spans = append(w.spans, rec)
	return nil
}

func TestWriterAsTraceConsumerPushesEachSpan(t *testing.T) {
	w := &recordingTraceWriter{}
	c := output.WriterAsTraceConsumer(w, embed.NopTelemetry())

	batch := []embed.Span{
		{Name: "one"},
		{Name: "two"},
		{Name: "three"},
	}
	if err := c.ConsumeTraces(context.Background(), batch); err != nil {
		t.Fatalf("ConsumeTraces: %v", err)
	}
	if got, want := len(w.spans), 3; got != want {
		t.Fatalf("recorded %d, want %d", got, want)
	}
	for i, want := range []string{"one", "two", "three"} {
		if w.spans[i].Name != want {
			t.Errorf("span %d: %q, want %q", i, w.spans[i].Name, want)
		}
	}
}

func TestWriterAsTraceConsumerStopsOnFirstError(t *testing.T) {
	wantErr := errors.New("trace boom")
	w := &recordingTraceWriter{err: wantErr}
	c := output.WriterAsTraceConsumer(w, embed.NopTelemetry())

	err := c.ConsumeTraces(context.Background(), []embed.Span{
		{Name: "one"},
		{Name: "two"},
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected wantErr, got %v", err)
	}
}

func TestWriterAsTraceConsumerPanicsOnNilWriter(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on nil writer, got none")
		}
	}()
	_ = output.WriterAsTraceConsumer(nil, embed.NopTelemetry())
}

func TestWriterAsLogConsumerPanicsOnNilWriter(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on nil writer, got none")
		}
	}()
	_ = output.WriterAsLogConsumer(nil, embed.NopTelemetry())
}

func TestWriterAsMetricConsumerPanicsOnNilWriter(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on nil writer, got none")
		}
	}()
	_ = output.WriterAsMetricConsumer(nil, embed.NopTelemetry())
}

func (w *recordingWriter) Stop(context.Context) error { return nil }
func (w *recordingWriter) SupportedTelemetry() []telemetry.Type {
	return []telemetry.Type{telemetry.Logs}
}
func (w *recordingMetricWriter) Stop(context.Context) error { return nil }
func (w *recordingMetricWriter) SupportedTelemetry() []telemetry.Type {
	return []telemetry.Type{telemetry.Metrics}
}
func (w *recordingTraceWriter) Stop(context.Context) error { return nil }
func (w *recordingTraceWriter) SupportedTelemetry() []telemetry.Type {
	return []telemetry.Type{telemetry.Traces}
}

// permitWriter records whether each write's context carried a count permit.
type permitWriter struct {
	counted []bool
}

func (w *permitWriter) WriteLog(ctx context.Context, _ output.LogRecord) error {
	w.counted = append(w.counted, count.Counted(ctx))
	return nil
}

func (w *permitWriter) WriteMetric(ctx context.Context, _ output.MetricRecord) error {
	w.counted = append(w.counted, count.Counted(ctx))
	return nil
}

func (w *permitWriter) WriteTrace(ctx context.Context, _ output.TraceRecord) error {
	w.counted = append(w.counted, count.Counted(ctx))
	return nil
}

func (w *permitWriter) Stop(context.Context) error { return nil }

func (w *permitWriter) SupportedTelemetry() []telemetry.Type {
	return []telemetry.Type{telemetry.Logs, telemetry.Metrics, telemetry.Traces}
}

// TestWriterAdaptersForwardThePermit pins that every adapter hands the
// consumer's context to the writer, so a queued output can hold the record's
// count permit.
func TestWriterAdaptersForwardThePermit(t *testing.T) {
	w := &permitWriter{}
	tel := embed.NopTelemetry()
	ctx, p := counttest.Held(t)
	require.NoError(t, output.WriterAsLogConsumer(w, tel).ConsumeLogs(ctx, []embed.LogRecord{{Message: "x"}}))
	require.NoError(t, output.WriterAsMetricConsumer(w, tel).ConsumeMetrics(ctx, []embed.MetricPoint{{Name: "m"}}))
	require.NoError(t, output.WriterAsTraceConsumer(w, tel).ConsumeTraces(ctx, []embed.Span{{Name: "s"}}))
	p.Commit()
	require.Equal(t, []bool{true, true, true}, w.counted)
}
