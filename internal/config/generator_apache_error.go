package config

import (
	"fmt"
	"time"
)

// ApacheErrorGeneratorConfig contains configuration for Apache Error log generator
type ApacheErrorGeneratorConfig struct {
	// Workers is the number of worker goroutines for Apache Error generation
	Workers int `yaml:"workers,omitempty" mapstructure:"workers,omitempty"`
	// Rate is the rate at which logs are generated per worker
	Rate time.Duration `yaml:"rate,omitempty" mapstructure:"rate,omitempty"`
}

// Validate validates the Apache Error generator configuration
func (c *ApacheErrorGeneratorConfig) Validate() error {
	if c.Workers < 1 {
		return fmt.Errorf("Apache Error generator workers must be 1 or greater, got %d", c.Workers)
	}

	if c.Rate <= 0 {
		return fmt.Errorf("Apache Error generator rate must be positive, got %v", c.Rate)
	}

	return nil
}
