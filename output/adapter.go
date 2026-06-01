package output

import (
	"context"

	"github.com/observiq/blitz/embed"
)

// WriterAsLogConsumer wraps a Writer so it can be used in contexts that
// expect an embed.LogConsumer. The adapter pushes each record in the
// batch through Writer.Write in order, returning the first error it
// encounters.
//
// CLI generator wiring uses this adapter to bridge migrated modules
// (which talk to embed.LogConsumer) with the existing Output instances
// (which implement Writer).
//
// Panics on nil writer — a nil writer is a programming bug, not a
// runtime condition, and catching it at construction surfaces the
// failure at the boundary rather than deep in ConsumeLogs.
func WriterAsLogConsumer(w Writer) embed.LogConsumer {
	if w == nil {
		panic("output.WriterAsLogConsumer: writer cannot be nil")
	}
	return &writerAsLogConsumer{w: w}
}

type writerAsLogConsumer struct {
	w Writer
}

func (a *writerAsLogConsumer) ConsumeLogs(ctx context.Context, records []embed.LogRecord) error {
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
// implement MetricWriter.
//
// Panics on nil writer; see WriterAsLogConsumer for the rationale.
func WriterAsMetricConsumer(w MetricWriter) embed.MetricConsumer {
	if w == nil {
		panic("output.WriterAsMetricConsumer: writer cannot be nil")
	}
	return &writerAsMetricConsumer{w: w}
}

type writerAsMetricConsumer struct {
	w MetricWriter
}

func (a *writerAsMetricConsumer) ConsumeMetrics(ctx context.Context, points []embed.MetricPoint) error {
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
// implement TraceWriter.
//
// Panics on nil writer — a nil writer is a programming bug, not a
// runtime condition, and catching it at construction surfaces the
// failure at the boundary rather than deep in ConsumeTraces.
func WriterAsTraceConsumer(w TraceWriter) embed.TraceConsumer {
	if w == nil {
		panic("output.WriterAsTraceConsumer: writer cannot be nil")
	}
	return &writerAsTraceConsumer{w: w}
}

type writerAsTraceConsumer struct {
	w TraceWriter
}

func (a *writerAsTraceConsumer) ConsumeTraces(ctx context.Context, spans []embed.Span) error {
	for i := range spans {
		if err := a.w.WriteTrace(ctx, spans[i]); err != nil {
			return err
		}
	}
	return nil
}
