package embed

import (
	"context"
	"errors"
	"sync"

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

// runner is the concrete embed.Runner. The mu Mutex protects the joint
// invariant on (started, rt, resource) — when started is true, rt must
// be non-nil and resource holds the cloned host attributes. Locking
// across Start and Stop also serializes a host that accidentally calls
// them concurrently from different goroutines.
type runner struct {
	cfg Config

	mu       sync.Mutex
	rt       *runtime.Runtime
	resource map[string]any // cloned from host.Resource at Start
	started  bool
}

func (r *runner) Start(ctx context.Context, host Host) error {
	r.mu.Lock()
	defer r.mu.Unlock()

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
	rt := runtime.New(logger, rtModules, host.TracerProvider)
	if err := rt.Start(ctx); err != nil {
		return err
	}
	r.rt = rt
	r.resource = cloneResource(host.Resource)
	r.started = true
	return nil
}

func (r *runner) Stop(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.started {
		return nil
	}
	rt := r.rt
	r.rt = nil
	r.resource = nil
	r.started = false
	return rt.Stop(ctx)
}
