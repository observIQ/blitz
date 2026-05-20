package embed_test

import (
	"context"
	"testing"

	"github.com/observiq/blitz/embed"
)

// producerExample embeds ProducerMarker and exercises the compile-time
// check that the marker pattern lets out-of-package types satisfy
// ProducerModule.
type producerExample struct {
	embed.ProducerMarker
}

func (producerExample) Name() string                { return "producer-example" }
func (producerExample) Start(context.Context) error { return nil }
func (producerExample) Stop(context.Context) error  { return nil }

// effectorExample embeds EffectorMarker for the parallel check.
type effectorExample struct {
	embed.EffectorMarker
}

func (effectorExample) Name() string                { return "effector-example" }
func (effectorExample) Start(context.Context) error { return nil }
func (effectorExample) Stop(context.Context) error  { return nil }

// Compile-time assertions: the marker pattern lets external types
// satisfy ProducerModule and EffectorModule even though the marker
// methods are unexported.
var (
	_ embed.ProducerModule = producerExample{}
	_ embed.EffectorModule = effectorExample{}
)

func TestProducerModuleAcceptedByConfig(t *testing.T) {
	// Config.Modules accepts ProducerModule. This compiling at all
	// confirms that producerExample satisfies ProducerModule via the
	// embedded marker.
	cfg := embed.Config{
		Modules: []embed.ProducerModule{producerExample{}},
	}
	if len(cfg.Modules) != 1 {
		t.Fatalf("expected 1 module, got %d", len(cfg.Modules))
	}
	if cfg.Modules[0].Name() != "producer-example" {
		t.Fatalf("expected name 'producer-example', got %q", cfg.Modules[0].Name())
	}
}
