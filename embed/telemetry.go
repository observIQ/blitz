package embed

import (
	"github.com/observiq/blitz/internal/telemetry/logs"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/log"
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

	// LoggerProvider receives blitz's internal logs as OTel log records
	// (bridged from zap). Nil means logs stay zap-only, matching the behavior
	// before OTel logs were supported.
	LoggerProvider log.LoggerProvider

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

// BridgedLogger returns base teed into the bundle's LoggerProvider, so blitz's
// zap logging is also emitted as OTel log records. Unlike metrics and traces,
// which are built per component, the logger is bridged ONCE at the process
// entry point (main for standalone; config.LoadModules and the runner for
// embed) and the bridged logger is shared: a bridged zap logger propagates to
// the child loggers components derive, so re-bridging per component would only
// duplicate records. A nil base yields a nop logger; a nil LoggerProvider
// returns base unchanged (zap only), so the result is always safe to use.
func (t TelemetrySettings) BridgedLogger(base *zap.Logger) *zap.Logger {
	if base == nil {
		base = zap.NewNop()
	}
	return logs.BridgeZap(base, t.LoggerProvider)
}

// NopTelemetry returns a TelemetrySettings wired to no-op providers and a nop
// logger. Use it where a caller has no telemetry to route, most commonly in
// tests and in construction paths that record nothing. It is distinct from a
// zero-value bundle: NopTelemetry emits to no-op meter and tracer providers,
// while a zero-value bundle's nil fields fall back to the process globals.
//
// LoggerProvider is left nil: for logs, "no-op" means no bridge at all, so
// BridgedLogger returns the base logger unchanged (zap only) rather than teeing
// into a no-op provider. That keeps the nop path zero-overhead and preserves
// logger identity for callers that compare loggers.
func NopTelemetry() TelemetrySettings {
	return TelemetrySettings{
		Logger:         zap.NewNop(),
		MeterProvider:  metricnoop.NewMeterProvider(),
		TracerProvider: tracenoop.NewTracerProvider(),
	}
}
