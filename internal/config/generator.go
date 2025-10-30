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
	// GeneratorTypeWinevt represents Windows Event XML generator
	GeneratorTypeWinevt GeneratorType = "winevt"
)

// Generator contains configuration for log generators
type Generator struct {
	// Type specifies the generator type (json)
	Type GeneratorType `yaml:"type,omitempty" mapstructure:"type,omitempty"`
	// JSON contains JSON generator configuration
	JSON JSONGeneratorConfig `yaml:"json,omitempty" mapstructure:"json,omitempty"`
	// Winevt contains Windows Event generator configuration
	Winevt WinevtGeneratorConfig `yaml:"winevt,omitempty" mapstructure:"winevt,omitempty"`
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
	case GeneratorTypeWinevt:
		if err := g.Winevt.Validate(); err != nil {
			return fmt.Errorf("winevt generator validation failed: %w", err)
		}
	default:
		return fmt.Errorf("invalid generator type: %s, must be one of: nop, json, winevt", g.Type)
	}

	return nil
}
