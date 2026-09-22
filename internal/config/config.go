// Package config contains the top level configuration structures and logic
package config

import (
	"fmt"
	"reflect"
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
	// Outputs is the list of outputs for multi-output (fan-out) mode.
	// If set, takes precedence over the singular Output field, and every
	// generated record is fanned out to all of them.
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
	if err := c.Generator.Validate(); err != nil {
		return err
	}
	if len(c.Outputs) > 0 {
		// Validate the defaulted entries, since that is what gets built.
		for i, o := range c.EffectiveOutputs() {
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
// Comma-separated HostMetrics OS values are expanded into separate generators.
func (c *Config) EffectiveGenerators() []Generator {
	if len(c.Generators) > 0 {
		return expandGenerators(c.Generators)
	}
	return expandGenerators([]Generator{c.Generator})
}

// EffectiveOutputs returns the list of outputs to use.
// If Outputs is set, it takes precedence over the singular Output field.
// The singular Output carries the defaults applied by the override system,
// so each list entry inherits any field it leaves unset (workers, and so on)
// from it, while user-set fields win.
func (c *Config) EffectiveOutputs() []Output {
	if len(c.Outputs) == 0 {
		return []Output{c.Output}
	}
	outs := make([]Output, len(c.Outputs))
	for i, o := range c.Outputs {
		merged := o
		fillZeroFields(reflect.ValueOf(&merged).Elem(), reflect.ValueOf(c.Output))
		outs[i] = merged
	}
	return outs
}

// fillZeroFields sets each zero-valued field of dst to the corresponding
// field of tmpl, recursing into nested structs. A non-zero field in dst is
// left untouched, so an explicitly-set value always wins over the default.
func fillZeroFields(dst, tmpl reflect.Value) {
	for i := 0; i < dst.NumField(); i++ {
		df := dst.Field(i)
		if !df.CanSet() {
			continue
		}
		if df.Kind() == reflect.Struct {
			fillZeroFields(df, tmpl.Field(i))
			continue
		}
		if df.IsZero() {
			df.Set(tmpl.Field(i))
		}
	}
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
