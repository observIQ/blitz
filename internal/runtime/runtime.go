package runtime

import (
	"context"
	"fmt"

	"go.uber.org/zap"
)

// Module is the narrow lifecycle interface Runtime operates on. Both
// embed.ProducerModule and the legacy generator.Generator interfaces
// can be wrapped to satisfy this contract — Runtime stays decoupled
// from the embed package to avoid an import cycle.
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
type Runtime struct {
	logger  *zap.Logger
	modules []Module
}

// New returns a Runtime configured with the given logger and modules.
func New(logger *zap.Logger, modules []Module) *Runtime {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Runtime{
		logger:  logger,
		modules: modules,
	}
}

// Start begins every configured module. If any module's Start returns
// an error, Start stops the modules already started (in reverse order)
// and returns the failure.
func (r *Runtime) Start(ctx context.Context) error {
	started := make([]Module, 0, len(r.modules))
	for _, m := range r.modules {
		if err := m.Start(ctx); err != nil {
			// Roll back: stop modules already started, in reverse order.
			for i := len(started) - 1; i >= 0; i-- {
				if stopErr := started[i].Stop(ctx); stopErr != nil {
					r.logger.Warn("module stop failed during start rollback",
						zap.String("module", started[i].Name()),
						zap.Error(stopErr))
				}
			}
			return fmt.Errorf("start module %s: %w", m.Name(), err)
		}
		started = append(started, m)
	}
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
	}
	return firstErr
}
