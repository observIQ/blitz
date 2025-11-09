package config

import (
	"fmt"
	"time"
)

// NginxGeneratorConfig contains configuration for NGINX log generator
type NginxGeneratorConfig struct {
	// Workers is the number of worker goroutines for NGINX generation
	Workers int `yaml:"workers,omitempty" mapstructure:"workers,omitempty"`
	// Rate is the rate at which logs are generated per worker
	Rate time.Duration `yaml:"rate,omitempty" mapstructure:"rate,omitempty"`
}

// Validate validates the NGINX generator configuration
func (c *NginxGeneratorConfig) Validate() error {
	if c.Workers < 1 {
		return fmt.Errorf("NGINX generator workers must be 1 or greater, got %d", c.Workers)
	}

	if c.Rate <= 0 {
		return fmt.Errorf("NGINX generator rate must be positive, got %v", c.Rate)
	}

	return nil
}
