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
	// Outputs is the list of outputs for multi-output (fan-out) mode; every
	// generated record is fanned out to all of them. It is the alternative to
	// the singular Output, never a supplement: setting both is rejected. Each
	// entry takes only its type's defaults, never values from Output.
	Outputs []Output `yaml:"outputs,omitempty" mapstructure:"outputs,omitempty"`
	// Metrics configuration (Prometheus scrape endpoint for self-metrics)
	Metrics Metrics `yaml:"metrics,omitempty" mapstructure:"metrics,omitempty"`
	// Environment configures the simulated datagen.Environment identities
	// that generators draw their host.name/OS from (PIPE-1036).
	Environment EnvironmentConfig `yaml:"environment,omitempty" mapstructure:"environment,omitempty"`
	// Telemetry configures export of blitz's own self-telemetry (self-traces,
	// and later self-logs) via OTLP.
	Telemetry Telemetry `yaml:"telemetry,omitempty" mapstructure:"telemetry,omitempty"`
	// OnFinish controls behavior when finite generation completes.
	// One of: "exit" (default), "idle"
	OnFinish string `yaml:"onFinish,omitempty" mapstructure:"onFinish,omitempty"`
}

// Validate validates the entire configuration
func (c *Config) Validate() error {
	if err := c.Logging.Validate(); err != nil {
		return err
	}
	if len(c.Generators) > 0 {
		// Validate the defaulted entries, since that is what runs.
		for i, g := range c.EffectiveGenerators() {
			if err := g.Validate(); err != nil {
				return fmt.Errorf("generators[%d] validation failed: %w", i, err)
			}
		}
	} else if err := c.Generator.Validate(); err != nil {
		return err
	}
	if len(c.Outputs) > 0 {
		for i, o := range c.Outputs {
			if err := o.Validate(); err != nil {
				return fmt.Errorf("outputs[%d] validation failed: %w", i, err)
			}
		}
	} else if err := c.Output.Validate(); err != nil {
		return err
	}
	if err := c.Metrics.Validate(); err != nil {
		return err
	}
	if err := c.Environment.Validate(); err != nil {
		return err
	}
	if err := c.Telemetry.Validate(); err != nil {
		return err
	}
	if c.OnFinish != "" && c.OnFinish != "exit" && c.OnFinish != "idle" {
		return fmt.Errorf("onFinish must be one of: exit, idle, got %q", c.OnFinish)
	}
	return nil
}

// EffectiveGenerators returns the list of generators to use.
// If Generators is set, it takes precedence over the singular Generator field.
// The singular Generator carries the defaults applied by the override system,
// so each list entry inherits any field it leaves unset (workers, and so on)
// from it, while user-set fields win.
// Comma-separated HostMetrics OS values are expanded into separate generators.
func (c *Config) EffectiveGenerators() []Generator {
	if len(c.Generators) == 0 {
		return expandGenerators([]Generator{c.Generator})
	}
	merged := make([]Generator, len(c.Generators))
	for i, g := range c.Generators {
		m := g
		fillZeroFields(reflect.ValueOf(&m).Elem(), reflect.ValueOf(c.Generator))
		merged[i] = m
	}
	return expandGenerators(merged)
}

// EffectiveOutputs returns the list of outputs to use: Outputs when set,
// otherwise the singular Output. Each Outputs entry already carries its type's
// defaults (ApplyListDefaults at load), never values from the singular Output.
func (c *Config) EffectiveOutputs() []Output {
	if len(c.Outputs) == 0 {
		return []Output{c.Output}
	}
	return c.Outputs
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
