package config

import (
	"fmt"
	"time"
)

// PaloAltoGeneratorConfig contains configuration for Palo Alto generator
type PaloAltoGeneratorConfig struct {
	// Workers is the number of worker goroutines
	Workers int `yaml:"workers,omitempty" mapstructure:"workers,omitempty"`
	// Rate is the generation interval per worker
	Rate time.Duration `yaml:"rate,omitempty" mapstructure:"rate,omitempty"`
}

// Validate validates the palo alto generator configuration
func (c *PaloAltoGeneratorConfig) Validate() error {
	if c.Workers < 1 {
		return fmt.Errorf("palo-alto generator workers must be 1 or greater, got %d", c.Workers)
	}
	if c.Rate <= 0 {
		return fmt.Errorf("palo-alto generator rate must be positive, got %v", c.Rate)
	}
	return nil
}
