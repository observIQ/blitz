package embed

import (
	"context"
	"errors"

	"github.com/observiq/blitz/internal/runtime"
	"go.uber.org/zap"
)

// New constructs a Runner that operates the configured ProducerModules.
//
// Modules are expected to already have their consumers wired at
// construction time (e.g. apachegen.New(..., host.Logs)). New does not
// re-wire consumers; the Host passed to Runner.Start is used for the
// runtime's internal logging and to expose resource attributes that
// modules may consult.
//
// Returns an error if Modules is empty.
func New(cfg Config) (Runner, error) {
	if len(cfg.Modules) == 0 {
		return nil, errors.New("embed.Config.Modules cannot be empty")
	}
	return &runner{cfg: cfg}, nil
}

type runner struct {
	cfg     Config
	rt      *runtime.Runtime
	started bool
}

func (r *runner) Start(ctx context.Context, host Host) error {
	if r.started {
		return errors.New("embed runner already started")
	}
	logger := host.Logger
	if logger == nil {
		logger = zap.NewNop()
	}
	rtModules := make([]runtime.Module, len(r.cfg.Modules))
	for i, m := range r.cfg.Modules {
		rtModules[i] = m
	}
	r.rt = runtime.New(logger, rtModules)
	if err := r.rt.Start(ctx); err != nil {
		return err
	}
	r.started = true
	return nil
}

func (r *runner) Stop(ctx context.Context) error {
	if !r.started {
		return nil
	}
	r.started = false
	return r.rt.Stop(ctx)
}
