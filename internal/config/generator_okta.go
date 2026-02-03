package config

import (
	"fmt"
	"time"
)

// OktaGeneratorConfig contains configuration for Okta System Log generator
type OktaGeneratorConfig struct {
	// Workers is the number of worker goroutines for Okta log generation
	Workers int `yaml:"workers,omitempty" mapstructure:"workers,omitempty"`
	// Rate is the rate at which logs are generated per worker
	Rate time.Duration `yaml:"rate,omitempty" mapstructure:"rate,omitempty"`
}

// Validate validates the Okta generator configuration
func (c *OktaGeneratorConfig) Validate() error {
	if c.Workers < 1 {
		return fmt.Errorf("Okta generator workers must be 1 or greater, got %d", c.Workers)
	}

	if c.Rate <= 0 {
		return fmt.Errorf("Okta generator rate must be positive, got %v", c.Rate)
	}

	return nil
}
