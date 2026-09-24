package config

import (
	"fmt"
	"time"

	"github.com/observiq/blitz/generator/f5/catalog"

	// Register the F5 products so product-name validation can see them.
	_ "github.com/observiq/blitz/generator/f5"
)

// F5GeneratorConfig contains configuration for the multi-product F5 log
// generator.
type F5GeneratorConfig struct {
	// Workers is the number of worker goroutines.
	Workers int `yaml:"workers,omitempty" mapstructure:"workers,omitempty"`
	// Rate is the per-worker emission interval.
	Rate time.Duration `yaml:"rate,omitempty" mapstructure:"rate,omitempty"`
	// Hostname is the simulated F5 device hostname in the syslog header.
	Hostname string `yaml:"hostname,omitempty" mapstructure:"hostname,omitempty"`
	// EnabledProducts restricts emission to a subset of product names.
	// Empty = all. Valid tokens: ltm, asm, afm, apm, dns, audit,
	// nginx-plus, nginx-app-protect, irules, f5os.
	EnabledProducts []string `yaml:"enabledProducts,omitempty" mapstructure:"enabledProducts,omitempty"`
	// Weights sets the relative mix ratio per product name. Absent =
	// weight 1. Empty map = all equal.
	Weights map[string]float64 `yaml:"weights,omitempty" mapstructure:"weights,omitempty"`
	// Seed is the deterministic RNG seed. Negative randomizes per worker;
	// 0+ produces byte-identical output across runs for the same seed.
	Seed int64 `yaml:"seed,omitempty" mapstructure:"seed,omitempty"`
}

// Validate validates the F5 generator configuration.
func (c *F5GeneratorConfig) Validate() error {
	if c.Workers < 1 {
		return fmt.Errorf("f5 generator workers must be 1 or greater, got %d", c.Workers)
	}
	if c.Rate <= 0 {
		return fmt.Errorf("f5 generator rate must be positive, got %v", c.Rate)
	}
	for _, name := range c.EnabledProducts {
		if _, ok := catalog.Get(name); !ok {
			return fmt.Errorf("f5 generator enabledProducts: unknown product %q", name)
		}
	}
	for name := range c.Weights {
		if _, ok := catalog.Get(name); !ok {
			return fmt.Errorf("f5 generator weights: unknown product %q", name)
		}
	}
	return nil
}
