package output

import (
	"context"
	"errors"

	"github.com/observiq/blitz/telemetry"
)

// MultiOutput fans one generated stream out to several child outputs,
// implementing Output plus MetricWriter and TraceWriter so callers treat it
// like a single output. It records no telemetry itself; each child emits its
// own output_type-tagged metrics, so there is no double-counting.
type MultiOutput struct {
	outputs []Output
}

// NewMultiOutput wraps the given outputs.
func NewMultiOutput(outputs ...Output) *MultiOutput {
	return &MultiOutput{outputs: outputs}
}

// WriteLog fans a log record to children that implement LogWriter and
// declare Logs; others are skipped. Best-effort: a failing child does not
// stop the others; the aggregated error is returned (nil if all succeed).
func (m *MultiOutput) WriteLog(ctx context.Context, data LogRecord) error {
	var errs []error
	for _, o := range m.outputs {
		w, ok := o.(LogWriter)
		if !ok || !supports(o, telemetry.Logs) {
			continue
		}
		if err := w.WriteLog(ctx, data); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// WriteMetric fans a metric record to children that implement MetricWriter
// and declare Metrics; others are skipped.
func (m *MultiOutput) WriteMetric(ctx context.Context, data MetricRecord) error {
	var errs []error
	for _, o := range m.outputs {
		w, ok := o.(MetricWriter)
		if !ok || !supports(o, telemetry.Metrics) {
			continue
		}
		if err := w.WriteMetric(ctx, data); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// WriteTrace fans a trace record to children that implement TraceWriter and
// declare Traces; others are skipped.
func (m *MultiOutput) WriteTrace(ctx context.Context, data TraceRecord) error {
	var errs []error
	for _, o := range m.outputs {
		w, ok := o.(TraceWriter)
		if !ok || !supports(o, telemetry.Traces) {
			continue
		}
		if err := w.WriteTrace(ctx, data); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// Stop stops every child, aggregating errors; one failure does not skip the rest.
func (m *MultiOutput) Stop(ctx context.Context) error {
	var errs []error
	for _, o := range m.outputs {
		if err := o.Stop(ctx); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// SupportedTelemetry returns the union of the children's supported types.
func (m *MultiOutput) SupportedTelemetry() []telemetry.Type {
	seen := make(map[telemetry.Type]struct{})
	var union []telemetry.Type
	for _, o := range m.outputs {
		for _, t := range o.SupportedTelemetry() {
			if _, dup := seen[t]; dup {
				continue
			}
			seen[t] = struct{}{}
			union = append(union, t)
		}
	}
	return union
}

// supports reports whether the output declares the given telemetry type.
func supports(o Output, t telemetry.Type) bool {
	for _, s := range o.SupportedTelemetry() {
		if s == t {
			return true
		}
	}
	return false
}
