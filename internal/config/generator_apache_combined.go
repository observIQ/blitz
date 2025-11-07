package config

import (
	"fmt"
	"time"
)

// ApacheCombinedGeneratorConfig contains configuration for Apache Combined log generator
type ApacheCombinedGeneratorConfig struct {
	// Workers is the number of worker goroutines for Apache Combined generation
	Workers int `yaml:"workers,omitempty" mapstructure:"workers,omitempty"`
	// Rate is the rate at which logs are generated per worker
	Rate time.Duration `yaml:"rate,omitempty" mapstructure:"rate,omitempty"`
}

// Validate validates the Apache Combined generator configuration
func (c *ApacheCombinedGeneratorConfig) Validate() error {
	if c.Workers < 1 {
		return fmt.Errorf("Apache Combined generator workers must be 1 or greater, got %d", c.Workers)
	}

	if c.Rate <= 0 {
		return fmt.Errorf("Apache Combined generator rate must be positive, got %v", c.Rate)
	}

	return nil
}
