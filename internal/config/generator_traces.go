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
	// attributes describe. Set, it wins over the environment host and
	// replaces only that host's name. Empty uses the environment host's
	// hostname, or with no environment host deterministically generates one
	// from Seed via datagen.GenerateHostname.
	Hostname string `yaml:"hostname,omitempty" mapstructure:"hostname,omitempty"`
	// Seed seeds span content (Name, Kind, StatusCode, Attributes, child
	// count) per the contract in docs/datagen.md: negative randomizes (the
	// resolved seed is logged), 0 or positive is deterministic (worker N
	// uses Seed+N), and an omitted key defaults to -1. TraceID and SpanID
	// come from crypto/rand, not Seed, so IDs stay globally unique. Host
	// identity is not governed by this seed: the loaders always build the
	// simulated environment, which picks the host from
	// `environment.seed_config`.
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
