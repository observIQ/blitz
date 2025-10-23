package config

import (
	"fmt"
)

// GeneratorType represents the type of generator
type GeneratorType string

const (
	// GeneratorTypeNop represents NOP generator
	GeneratorTypeNop GeneratorType = "nop"
	// GeneratorTypeJSON represents JSON generator
	GeneratorTypeJSON GeneratorType = "json"
)

// Generator contains configuration for log generators
type Generator struct {
	// Type specifies the generator type (json)
	Type GeneratorType `yaml:"type,omitempty" mapstructure:"type,omitempty"`
	// JSON contains JSON generator configuration
	JSON JSONGeneratorConfig `yaml:"json,omitempty" mapstructure:"json,omitempty"`
}

// Validate validates the generator configuration
func (g *Generator) Validate() error {
	// Allow empty type - defaults will be applied by override system
	if g.Type == "" {
		return nil
	}

	switch g.Type {
	case GeneratorTypeNop:
		// NOP generator requires no additional validation
	case GeneratorTypeJSON:
		if err := g.JSON.Validate(); err != nil {
			return fmt.Errorf("JSON generator validation failed: %w", err)
		}
	default:
		return fmt.Errorf("invalid generator type: %s, must be one of: nop, json", g.Type)
	}

	return nil
}
