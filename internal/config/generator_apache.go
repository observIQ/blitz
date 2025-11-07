package config

import (
	"fmt"
	"time"
)

// ApacheGeneratorConfig contains configuration for Apache log generator
type ApacheGeneratorConfig struct {
	// Workers is the number of worker goroutines for Apache generation
	Workers int `yaml:"workers,omitempty" mapstructure:"workers,omitempty"`
	// Rate is the rate at which logs are generated per worker
	Rate time.Duration `yaml:"rate,omitempty" mapstructure:"rate,omitempty"`
}

// Validate validates the Apache generator configuration
func (c *ApacheGeneratorConfig) Validate() error {
	if c.Workers < 1 {
		return fmt.Errorf("Apache generator workers must be 1 or greater, got %d", c.Workers)
	}

	if c.Rate <= 0 {
		return fmt.Errorf("Apache generator rate must be positive, got %v", c.Rate)
	}

	return nil
}
