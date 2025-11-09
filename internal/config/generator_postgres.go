package config

import (
	"fmt"
	"time"
)

// PostgresGeneratorConfig contains configuration for PostgreSQL log generator
type PostgresGeneratorConfig struct {
	// Workers is the number of worker goroutines for PostgreSQL generation
	Workers int `yaml:"workers,omitempty" mapstructure:"workers,omitempty"`
	// Rate is the rate at which logs are generated per worker
	Rate time.Duration `yaml:"rate,omitempty" mapstructure:"rate,omitempty"`
}

// Validate validates the PostgreSQL generator configuration
func (c *PostgresGeneratorConfig) Validate() error {
	if c.Workers < 1 {
		return fmt.Errorf("PostgreSQL generator workers must be 1 or greater, got %d", c.Workers)
	}

	if c.Rate <= 0 {
		return fmt.Errorf("PostgreSQL generator rate must be positive, got %v", c.Rate)
	}

	return nil
}

