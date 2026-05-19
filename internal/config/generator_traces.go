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
