package config

import (
	"fmt"
	"time"
)

// TracesGeneratorConfig contains configuration for trace generator
type TracesGeneratorConfig struct {
	// Workers is the number of worker goroutines for trace generation
	Workers int `yaml:"workers,omitempty" mapstructure:"workers,omitempty"`
	// Rate is the rate at which traces are generated per worker
	Rate time.Duration `yaml:"rate,omitempty" mapstructure:"rate,omitempty"`
	// Hostname is the simulated hostname every emitted span's Resource
	// attributes describe. Empty = deterministically generate from Seed
	// via datagen.GenerateHostname so configs are reproducible across
	// runs without pinning a hostname literal.
	Hostname string `yaml:"hostname,omitempty" mapstructure:"hostname,omitempty"`
	// Seed controls per-worker RNG seeding for span content (Name,
	// Kind, StatusCode, Attributes, child-count). Negative → randomize
	// (worker N gets time.Now().UnixNano()+N). Positive → deterministic
	// (worker N gets seed Seed+N).
	//
	// **YAML omitted ⇒ randomize.** Because the YAML zero-value is 0
	// and the architectural intent is that generator data output is
	// stochastic by default, the dispatch layer translates `seed: 0`
	// (and an omitted `seed:` key) into the negative-randomize path.
	// Set an explicit positive value for reproducibility.
	//
	// TraceID and SpanID are governed by crypto/rand, NOT Seed, so
	// IDs stay globally unique even when two blitz instances share a
	// Seed. Per-machine identity determinism (hostname) is governed
	// separately by `environment.seed_config` (PIPE-1036).
	Seed int64 `yaml:"seed,omitempty" mapstructure:"seed,omitempty"`
}

// Validate validates the traces generator configuration
func (c *TracesGeneratorConfig) Validate() error {
	if c.Workers < 1 {
		return fmt.Errorf("traces generator workers must be 1 or greater, got %d", c.Workers)
	}

	if c.Rate <= 0 {
		return fmt.Errorf("traces generator rate must be positive, got %v", c.Rate)
	}

	return nil
}
