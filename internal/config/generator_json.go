package config

import (
	"fmt"
	"time"

	"github.com/observiq/blitz/internal/generator/logtypes"
)

// JSONGeneratorConfig contains configuration for JSON log generator
type JSONGeneratorConfig struct {
	// Workers is the number of worker goroutines for JSON generation
	Workers int `yaml:"workers,omitempty" mapstructure:"workers,omitempty"`
	// Rate is the rate at which logs are generated per worker
	Rate time.Duration `yaml:"rate,omitempty" mapstructure:"rate,omitempty"`
	// Type is the type of log to generate. Valid values: "default", "pii"
	Type string `yaml:"type,omitempty" mapstructure:"type,omitempty"`
}

// Validate validates the JSON generator configuration
func (c *JSONGeneratorConfig) Validate() error {
	if c.Workers < 1 {
		return fmt.Errorf("JSON generator workers must be 1 or greater, got %d", c.Workers)
	}

	if c.Rate <= 0 {
		return fmt.Errorf("JSON generator rate must be positive, got %v", c.Rate)
	}

	switch c.Type {
	case "", logtypes.LogTypeDefault, logtypes.LogTypePII:
		// Valid log type, no error
	default:
		return fmt.Errorf("JSON generator type must be one of: %s, %s, got %q", logtypes.LogTypeDefault, logtypes.LogTypePII, c.Type)
	}

	return nil
}
