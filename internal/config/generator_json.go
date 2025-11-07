package config

import (
	"fmt"
	"time"

	"github.com/observiq/blitz/generator/json"
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

	if c.Type != "" && c.Type != json.LogTypeDefault && c.Type != json.LogTypePII {
		return fmt.Errorf("JSON generator type must be one of: %s, %s, got %q", json.LogTypeDefault, json.LogTypePII, c.Type)
	}

	return nil
}
