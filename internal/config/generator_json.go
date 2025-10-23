package config

import (
	"fmt"
	"time"
)

// JSONGeneratorConfig contains configuration for JSON log generator
type JSONGeneratorConfig struct {
	// Workers is the number of worker goroutines for JSON generation
	Workers int `yaml:"workers,omitempty" mapstructure:"workers,omitempty"`
	// Rate is the rate at which logs are generated per worker
	Rate time.Duration `yaml:"rate,omitempty" mapstructure:"rate,omitempty"`
}

// Validate validates the JSON generator configuration
func (c *JSONGeneratorConfig) Validate() error {
	if c.Workers < 1 {
		return fmt.Errorf("JSON generator workers must be 1 or greater, got %d", c.Workers)
	}

	if c.Rate <= 0 {
		return fmt.Errorf("JSON generator rate must be positive, got %v", c.Rate)
	}

	return nil
}
