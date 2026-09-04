package embed

import (
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
	metricnoop "go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/trace"
	tracenoop "go.opentelemetry.io/otel/trace/noop"
	"go.uber.org/zap"
)

// TelemetrySettings bundles the OTel providers and logger blitz uses to emit
// its own self-telemetry. It is the single value threaded from a host (or the
// standalone CLI) down through module construction, so every generator, output,
// and the runtime records to the same providers.
//
// Every field is optional. A nil provider means "fall back to the process
// global" (via the generated NewMetrics and, in later phases, the tracer and
// logger constructors), so a zero-value bundle behaves exactly as blitz did
// before providers were injectable.
type TelemetrySettings struct {
	// Logger is blitz's internal diagnostic logger. Nil means callers supply
	// their own fallback (a nop logger).
	Logger *zap.Logger

	// MeterProvider is the source of blitz's internal metric instruments. Nil
	// falls back to otel.GetMeterProvider().
	MeterProvider metric.MeterProvider

	// TracerProvider is the source of blitz's internal spans. Nil falls back to
	// otel.GetTracerProvider().
	TracerProvider trace.TracerProvider

	// PerBatchSpans enables the higher-volume per-emit-cycle spans. Off by
	// default; the always-on coarse spans do not depend on it.
	PerBatchSpans bool
}

// Tracer returns a tracer for the given instrumentation scope from the bundle's
// TracerProvider. A nil TracerProvider falls back to the process global, so the
// result is always safe to use.
func (t TelemetrySettings) Tracer(scope string) trace.Tracer {
	tp := t.TracerProvider
	if tp == nil {
		tp = otel.GetTracerProvider()
	}
	return tp.Tracer(scope)
}

// NopTelemetry returns a TelemetrySettings wired to no-op providers and a nop
// logger. Use it where a caller has no telemetry to route, most commonly in
// tests and in construction paths that record nothing. It is distinct from a
// zero-value bundle: NopTelemetry emits to no-op providers, while a zero-value
// bundle's nil fields fall back to the process globals.
func NopTelemetry() TelemetrySettings {
	return TelemetrySettings{
		Logger:         zap.NewNop(),
		MeterProvider:  metricnoop.NewMeterProvider(),
		TracerProvider: tracenoop.NewTracerProvider(),
	}
}
