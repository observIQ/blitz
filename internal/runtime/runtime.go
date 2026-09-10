package runtime

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// tracerScope is the instrumentation scope for the runtime's self-telemetry
// spans.
const tracerScope = "github.com/observiq/blitz/internal/runtime"

// Module is the narrow lifecycle interface Runtime operates on. The
// embed.ProducerModule type can be wrapped to satisfy this contract —
// Runtime stays decoupled from the embed package to avoid an import
// cycle.
type Module interface {
	// Name returns the module's identifier used in logs and errors.
	Name() string

	// Start begins module execution. Returning a nil error means the
	// module is running; resources must be released by Stop.
	Start(ctx context.Context) error

	// Stop terminates module execution.
	Stop(ctx context.Context) error
}

// Runtime is the shared lifecycle core. It owns module orchestration:
// starting every configured Module, then stopping each on shutdown.
//
// Runtime is not constructed directly by external callers — cli.Runner
// and embed.Runner each wrap a Runtime with their own process-level or
// host-level concerns. CLI adds signal handling, YAML loading, and
// output wiring; embed adds host-supplied consumers and resource
// attributes.
//
// Runtime emits blitz's session-level self-telemetry: a root "blitz.session"
// span covering Start to Stop, with a child "blitz.generator.run" span per
// module (bounded by that module's lifetime). These spans are decoupled from
// the embed package; the caller passes a raw trace.TracerProvider.
type Runtime struct {
	logger  *zap.Logger
	modules []Module
	tracer  trace.Tracer
	metrics *Metrics

	// sessionSpan and moduleSpans hold the open self-telemetry spans between
	// Start and Stop. Start and Stop are called once each and never
	// concurrently, so no synchronization is needed. moduleSpans is
	// index-aligned with modules.
	sessionSpan trace.Span
	moduleSpans []trace.Span
}

// New returns a Runtime configured with the given logger, modules, tracer
// provider, and meter provider. A nil tracerProvider or meterProvider falls
// back to the process global, so emission is always safe. It returns an error
// only if the runtime's own metric instruments cannot be built.
func New(logger *zap.Logger, modules []Module, tracerProvider trace.TracerProvider, meterProvider metric.MeterProvider) (*Runtime, error) {
	if logger == nil {
		logger = zap.NewNop()
	}
	if tracerProvider == nil {
		tracerProvider = otel.GetTracerProvider()
	}
	metrics, err := NewMetrics(meterProvider)
	if err != nil {
		return nil, fmt.Errorf("build runtime metrics: %w", err)
	}
	return &Runtime{
		logger:  logger,
		modules: modules,
		tracer:  tracerProvider.Tracer(tracerScope),
		metrics: metrics,
	}, nil
}

// Start begins every configured module. If any module's Start returns
// an error, Start stops the modules already started (in reverse order)
// and returns the failure.
func (r *Runtime) Start(ctx context.Context) error {
	sessionStart := time.Now()
	ctx, r.sessionSpan = r.tracer.Start(ctx, "blitz.session")
	r.moduleSpans = make([]trace.Span, 0, len(r.modules))

	started := make([]Module, 0, len(r.modules))
	for _, m := range r.modules {
		mctx, mspan := r.tracer.Start(ctx, "blitz.generator.run",
			trace.WithAttributes(attribute.String("blitz.generator.name", m.Name())))
		moduleStart := time.Now()
		if err := m.Start(mctx); err != nil {
			mspan.End()
			// Roll back: stop modules already started, in reverse order.
			for i := len(started) - 1; i >= 0; i-- {
				if stopErr := started[i].Stop(ctx); stopErr != nil {
					r.logger.Warn("module stop failed during start rollback",
						zap.String("module", started[i].Name()),
						zap.Error(stopErr))
				}
				r.moduleSpans[i].End()
			}
			r.sessionSpan.End()
			return fmt.Errorf("start module %s: %w", m.Name(), err)
		}
		r.metrics.BlitzModuleStartupDurationHistogram.Record(ctx, DurationMillis(time.Since(moduleStart)), m.Name())
		started = append(started, m)
		r.moduleSpans = append(r.moduleSpans, mspan)
	}
	r.metrics.BlitzSessionStartupDurationHistogram.Record(ctx, DurationMillis(time.Since(sessionStart)))
	return nil
}

// Stop terminates every module in reverse start order. Stop returns the
// first error encountered but continues stopping the remaining modules.
func (r *Runtime) Stop(ctx context.Context) error {
	var firstErr error
	for i := len(r.modules) - 1; i >= 0; i-- {
		m := r.modules[i]
		if err := m.Stop(ctx); err != nil {
			r.logger.Warn("module stop failed",
				zap.String("module", m.Name()),
				zap.Error(err))
			if firstErr == nil {
				firstErr = fmt.Errorf("stop module %s: %w", m.Name(), err)
			}
		}
		if i < len(r.moduleSpans) && r.moduleSpans[i] != nil {
			r.moduleSpans[i].End()
		}
	}
	if r.sessionSpan != nil {
		r.sessionSpan.End()
	}
	return firstErr
}
