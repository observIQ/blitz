package output

import (
	"context"

	"github.com/observiq/blitz/embed"
	"go.opentelemetry.io/otel/trace"
)

// adapterTracerScope is the instrumentation scope for the per-emit-cycle spans
// the consumer adapters create when TelemetrySettings.PerBatchSpans is on.
const adapterTracerScope = "github.com/observiq/blitz/output"

// noopSendSpan is a non-recording span returned by StartSendSpan when per-batch
// spans are off, so callers can always defer span.End() without a nil check.
var noopSendSpan = trace.SpanFromContext(context.Background())

// StartSendSpan starts a gated per-batch send span from ctx, so an output's
// worker can trace the actual (async) send parented to the emit span the
// consumer adapter opened. When tel.PerBatchSpans is off it returns ctx
// unchanged and a non-recording span, so the caller always defers span.End().
//
// The send runs in a worker goroutine after Write enqueued the batch, so the
// emit span has usually already ended by the time this fires. That is expected:
// OTel permits a child of an ended span, and the trace then reads as a brief
// enqueue (emit) with a later, longer send child, which is the correct picture
// of an asynchronous output. Callers carry the emit ctx through their internal
// channel to reach here.
func StartSendSpan(ctx context.Context, tel embed.TelemetrySettings, name string) (context.Context, trace.Span) {
	if !tel.PerBatchSpans {
		return ctx, noopSendSpan
	}
	return tel.Tracer(adapterTracerScope).Start(ctx, name)
}

// WriterAsLogConsumer wraps a Writer so it can be used in contexts that
// expect an embed.LogConsumer. The adapter pushes each record in the
// batch through Writer.Write in order, returning the first error it
// encounters.
//
// CLI generator wiring uses this adapter to bridge migrated modules
// (which talk to embed.LogConsumer) with the existing Output instances
// (which implement Writer). tel carries blitz's self-telemetry providers;
// when tel.PerBatchSpans is set, each ConsumeLogs call is wrapped in a span.
//
// Panics on nil writer — a nil writer is a programming bug, not a
// runtime condition, and catching it at construction surfaces the
// failure at the boundary rather than deep in ConsumeLogs.
func WriterAsLogConsumer(w Writer, tel embed.TelemetrySettings) embed.LogConsumer {
	if w == nil {
		panic("output.WriterAsLogConsumer: writer cannot be nil")
	}
	return &writerAsLogConsumer{w: w, tel: tel}
}

type writerAsLogConsumer struct {
	w   Writer
	tel embed.TelemetrySettings
}

func (a *writerAsLogConsumer) ConsumeLogs(ctx context.Context, records []embed.LogRecord) error {
	if a.tel.PerBatchSpans {
		var span trace.Span
		ctx, span = a.tel.Tracer(adapterTracerScope).Start(ctx, "blitz.emit.logs")
		defer span.End()
	}
	for i := range records {
		if err := a.w.Write(ctx, records[i]); err != nil {
			return err
		}
	}
	return nil
}

// WriterAsMetricConsumer wraps a MetricWriter so it can be used in
// contexts that expect an embed.MetricConsumer. The adapter pushes each
// point in the batch through MetricWriter.WriteMetric in order,
// returning the first error it encounters.
//
// Standalone CLI metric-generator wiring uses this adapter to bridge
// modules that talk to embed.MetricConsumer with existing outputs that
// implement MetricWriter. tel.PerBatchSpans wraps each call in a span.
//
// Panics on nil writer; see WriterAsLogConsumer for the rationale.
func WriterAsMetricConsumer(w MetricWriter, tel embed.TelemetrySettings) embed.MetricConsumer {
	if w == nil {
		panic("output.WriterAsMetricConsumer: writer cannot be nil")
	}
	return &writerAsMetricConsumer{w: w, tel: tel}
}

type writerAsMetricConsumer struct {
	w   MetricWriter
	tel embed.TelemetrySettings
}

func (a *writerAsMetricConsumer) ConsumeMetrics(ctx context.Context, points []embed.MetricPoint) error {
	if a.tel.PerBatchSpans {
		var span trace.Span
		ctx, span = a.tel.Tracer(adapterTracerScope).Start(ctx, "blitz.emit.metrics")
		defer span.End()
	}
	for i := range points {
		if err := a.w.WriteMetric(ctx, points[i]); err != nil {
			return err
		}
	}
	return nil
}

// WriterAsTraceConsumer wraps a TraceWriter so it can be used in
// contexts that expect an embed.TraceConsumer. The adapter pushes each
// span in the batch through TraceWriter.WriteTrace in order, returning
// the first error it encounters.
//
// Standalone CLI trace-generator wiring uses this adapter to bridge
// modules that talk to embed.TraceConsumer with existing outputs that
// implement TraceWriter. tel.PerBatchSpans wraps each call in a span.
//
// Panics on nil writer — a nil writer is a programming bug, not a
// runtime condition, and catching it at construction surfaces the
// failure at the boundary rather than deep in ConsumeTraces.
func WriterAsTraceConsumer(w TraceWriter, tel embed.TelemetrySettings) embed.TraceConsumer {
	if w == nil {
		panic("output.WriterAsTraceConsumer: writer cannot be nil")
	}
	return &writerAsTraceConsumer{w: w, tel: tel}
}

type writerAsTraceConsumer struct {
	w   TraceWriter
	tel embed.TelemetrySettings
}

func (a *writerAsTraceConsumer) ConsumeTraces(ctx context.Context, spans []embed.Span) error {
	if a.tel.PerBatchSpans {
		var span trace.Span
		ctx, span = a.tel.Tracer(adapterTracerScope).Start(ctx, "blitz.emit.traces")
		defer span.End()
	}
	for i := range spans {
		if err := a.w.WriteTrace(ctx, spans[i]); err != nil {
			return err
		}
	}
	return nil
}
