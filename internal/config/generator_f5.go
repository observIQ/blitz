package config

import (
	"fmt"
	"maps"
	"math"
	"slices"
	"time"

	"github.com/observiq/blitz/generator/f5/catalog"
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
	// Empty = all. Valid tokens: asm, afm, apm, dns, audit,
	// nginx-plus, nginx-app-protect, irules, f5os.
	EnabledProducts []string `yaml:"enabledProducts,omitempty" mapstructure:"enabledProducts,omitempty"`
	// Weights sets the relative mix ratio per product name. Absent =
	// weight 1. Empty map = all equal.
	Weights map[string]float64 `yaml:"weights,omitempty" mapstructure:"weights,omitempty"`
	// Seed is the RNG seed. Negative picks a random seed once (logged at
	// start); 0+ is deterministic (same seed + same start time = same output).
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
	seen := make(map[string]bool, len(c.EnabledProducts))
	for _, name := range c.EnabledProducts {
		if !catalog.Known(name) {
			return fmt.Errorf("f5 generator enabledProducts: unknown product %q", name)
		}
		if seen[name] {
			return fmt.Errorf("f5 generator enabledProducts: %q listed more than once", name)
		}
		seen[name] = true
	}
	var sum float64
	for _, name := range slices.Sorted(maps.Keys(c.Weights)) { // stable error and sum
		w := c.Weights[name]
		if !catalog.Known(name) {
			return fmt.Errorf("f5 generator weights: unknown product %q", name)
		}
		if len(seen) > 0 && !seen[name] {
			return fmt.Errorf("f5 generator weights: %q is not in enabledProducts", name)
		}
		if !(w > 0) || math.IsInf(w, 1) { // !(w > 0) also catches NaN
			return fmt.Errorf("f5 generator weights: %q must be positive and finite, got %v", name, w)
		}
		sum += w
	}
	if math.IsInf(sum, 1) {
		// An infinite total would always pick the last product.
		return fmt.Errorf("f5 generator weights: sum must be finite, got %v", sum)
	}
	return nil
}
