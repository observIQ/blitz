package config

import (
	"fmt"
	"time"
)

// WinevtGeneratorConfig contains configuration for Windows Event generator
type WinevtGeneratorConfig struct {
	// Workers is the number of worker goroutines
	Workers int `yaml:"workers,omitempty" mapstructure:"workers,omitempty"`
	// Rate is the generation interval per worker
	Rate time.Duration `yaml:"rate,omitempty" mapstructure:"rate,omitempty"`
}

// Validate validates the winevt generator configuration
func (c *WinevtGeneratorConfig) Validate() error {
	if c.Workers < 1 {
		return fmt.Errorf("winevt generator workers must be 1 or greater, got %d", c.Workers)
	}
	if c.Rate <= 0 {
		return fmt.Errorf("winevt generator rate must be positive, got %v", c.Rate)
	}
	return nil
}
