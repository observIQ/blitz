package service

import (
	"context"
	"fmt"
	"time"

	"github.com/observiq/blitz/embed"
	"github.com/observiq/blitz/internal/runtime"
	"github.com/observiq/blitz/output"
	"go.uber.org/zap"
)

// Service manages generators and an output. It delegates orchestration
// of migrated ProducerModule generators to internal/runtime.Runtime and
// keeps the type-switch dispatch only for legacy generators that still
// use the writer-based Start signature.
type Service struct {
	Logger     *zap.Logger
	Generators []any
	Output     output.Output

	runtime *runtime.Runtime
	legacy  []any
}

// New creates a new service with multiple generators and a single output.
func New(logger *zap.Logger, generators []any, output output.Output, tel embed.TelemetrySettings) (*Service, error) {
	if logger == nil {
		return nil, fmt.Errorf("logger cannot be nil")
	}
	if len(generators) == 0 {
		return nil, fmt.Errorf("generators cannot be empty")
	}
	if output == nil {
		return nil, fmt.Errorf("output cannot be nil")
	}

	// Partition generators by type: ProducerModules go to the shared
	// Runtime; everything else stays on legacy writer-based dispatch.
	// Runtime is decoupled from embed (import-cycle avoidance), so the
	// embed.ProducerModule check happens here at the seam.
	var modules []runtime.Module
	var legacy []any
	for _, gen := range generators {
		if pm, ok := gen.(embed.ProducerModule); ok {
			modules = append(modules, pm)
		} else {
			legacy = append(legacy, gen)
		}
	}

	return &Service{
		Logger:     logger,
		Generators: generators,
		Output:     output,
		runtime:    runtime.New(logger, modules, tel.TracerProvider),
		legacy:     legacy,
	}, nil
}

// Start starts all generators. ProducerModules run through Runtime; any
// remaining legacy generator returns an unsupported-type error.
//
// If the type switch hits the default case, Start rolls back: legacy
// generators already started are stopped in reverse order, then the
// Runtime is stopped, then the original error is returned. This prevents
// goroutine/worker leaks when Start aborts midway. Note that legacy
// generators' Stop methods use close(stopCh) under the hood, which
// panics on a second close — callers must NOT call Service.Stop after
// Service.Start returned an error; the rollback already cleaned up.
func (s *Service) Start() error {
	if err := s.runtime.Start(context.Background()); err != nil {
		return err
	}
	started := make([]any, 0, len(s.legacy))
	rollback := func() {
		ctx := context.Background()
		for i := len(started) - 1; i >= 0; i-- {
			if stopper, ok := started[i].(interface {
				Stop(context.Context) error
			}); ok {
				if err := stopper.Stop(ctx); err != nil {
					s.Logger.Warn("legacy generator stop failed during Start rollback",
						zap.Error(err))
				}
			}
		}
		if err := s.runtime.Stop(ctx); err != nil {
			s.Logger.Warn("runtime stop failed during Start rollback",
				zap.Error(err))
		}
	}
	for i, gen := range s.legacy {
		rollback()
		return fmt.Errorf("generator %d has unsupported type %T", i, gen)
	}
	return nil
}

// Stop stops all generators and the output. Stop will block for up to 30 seconds.
func (s *Service) Stop() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Stop the Runtime which stops ProducerModules in reverse declared order.
	var firstErr error
	if err := s.runtime.Stop(ctx); err != nil && firstErr == nil {
		firstErr = err
	}

	if err := s.Output.Stop(ctx); err != nil && firstErr == nil {
		firstErr = fmt.Errorf("stop output: %w", err)
	}

	return firstErr
}
