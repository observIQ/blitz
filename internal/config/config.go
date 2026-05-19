// Package config contains the top level configuration structures and logic
package config

import (
	"fmt"
	"strings"
)

// Config is the configuration for blitz.
type Config struct {
	// Logging configuration for the logger
	Logging Logging `yaml:"logging,omitempty" mapstructure:"logging,omitempty"`
	// Generator configuration
	Generator Generator `yaml:"generator,omitempty" mapstructure:"generator,omitempty"`
	// Generators is the list of generators for multi-generator mode.
	// If set, takes precedence over the singular Generator field.
	Generators []Generator `yaml:"generators,omitempty" mapstructure:"generators,omitempty"`
	// Output configuration
	Output Output `yaml:"output,omitempty" mapstructure:"output,omitempty"`
	// Metrics configuration
	Metrics Metrics `yaml:"metrics,omitempty" mapstructure:"metrics,omitempty"`
	// OnFinish controls behavior when finite generation completes.
	// One of: "exit" (default), "idle"
	OnFinish string `yaml:"onFinish,omitempty" mapstructure:"onFinish,omitempty"`
}

// Validate validates the entire configuration
func (c *Config) Validate() error {
	if err := c.Logging.Validate(); err != nil {
		return err
	}
	if err := c.Generator.Validate(); err != nil {
		return err
	}
	if err := c.Output.Validate(); err != nil {
		return err
	}
	if err := c.Metrics.Validate(); err != nil {
		return err
	}
	if c.OnFinish != "" && c.OnFinish != "exit" && c.OnFinish != "idle" {
		return fmt.Errorf("onFinish must be one of: exit, idle, got %q", c.OnFinish)
	}
	return nil
}

// EffectiveGenerators returns the list of generators to use.
// If Generators is set, it takes precedence over the singular Generator field.
// Comma-separated HostMetrics OS values are expanded into separate generators.
func (c *Config) EffectiveGenerators() []Generator {
	if len(c.Generators) > 0 {
		return expandGenerators(c.Generators)
	}
	return expandGenerators([]Generator{c.Generator})
}

// expandGenerators expands comma-separated HostMetrics OS values.
func expandGenerators(gens []Generator) []Generator {
	var result []Generator
	for _, g := range gens {
		if g.Type == GeneratorTypeHostMetrics && strings.Contains(g.HostMetrics.OS, ",") {
			parts := strings.Split(g.HostMetrics.OS, ",")
			for _, os := range parts {
				trimmed := strings.TrimSpace(os)
				if trimmed == "" {
					continue
				}
				expanded := g
				expanded.HostMetrics.OS = trimmed
				result = append(result, expanded)
			}
		} else {
			result = append(result, g)
		}
	}
	return result
}

// NewConfig returns a new config
func NewConfig() *Config {
	return &Config{}
}
