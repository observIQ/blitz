package config

import (
	"fmt"
	"time"

	"github.com/observiq/blitz/generator/fix/catalog"
)

// FIXGeneratorConfig contains configuration for the FIX (Financial
// Information eXchange) protocol generator.
type FIXGeneratorConfig struct {
	// Workers is the number of worker goroutines.
	Workers int `yaml:"workers,omitempty" mapstructure:"workers,omitempty"`
	// Rate is the per-worker emission interval.
	Rate time.Duration `yaml:"rate,omitempty" mapstructure:"rate,omitempty"`
	// Version selects the FIX protocol version: "4.2", "4.4", or
	// "5.0sp2". Defaults to "4.4" if empty.
	Version string `yaml:"version,omitempty" mapstructure:"version,omitempty"`
	// SenderCompID is the base tag 49 value (worker index appended).
	SenderCompID string `yaml:"senderCompID,omitempty" mapstructure:"senderCompID,omitempty"`
	// TargetCompID is the tag 56 value.
	TargetCompID string `yaml:"targetCompID,omitempty" mapstructure:"targetCompID,omitempty"`
	// EnabledCategories restricts emission to a subset of asset
	// categories. Empty = all. Valid tokens: equities, fx, futures,
	// options, govbonds, corpbonds, structured, otcderivs, repos,
	// moneymarket.
	EnabledCategories []string `yaml:"enabledCategories,omitempty" mapstructure:"enabledCategories,omitempty"`
	// Seed is the deterministic RNG seed. Negative randomizes per
	// worker; 0+ produces byte-identical output across runs for the
	// same seed.
	Seed int64 `yaml:"seed,omitempty" mapstructure:"seed,omitempty"`
}

// Validate validates the FIX generator configuration.
func (c *FIXGeneratorConfig) Validate() error {
	if c.Workers < 1 {
		return fmt.Errorf("fix generator workers must be 1 or greater, got %d", c.Workers)
	}
	if c.Rate <= 0 {
		return fmt.Errorf("fix generator rate must be positive, got %v", c.Rate)
	}
	if c.Version != "" {
		if _, err := catalog.VersionFromString(c.Version); err != nil {
			return fmt.Errorf("fix generator version invalid: %w", err)
		}
	}
	for _, cat := range c.EnabledCategories {
		if _, err := catalog.AssetCategoryFromString(cat); err != nil {
			return fmt.Errorf("fix generator enabledCategories invalid: %w", err)
		}
	}
	return nil
}
