// Package config contains the top level configuration structures and logic
package config

import "fmt"

// Config is the configuration for blitz.
type Config struct {
	// Logging configuration for the logger
	Logging Logging `yaml:"logging,omitempty" mapstructure:"logging,omitempty"`
	// Generator configuration
	Generator Generator `yaml:"generator,omitempty" mapstructure:"generator,omitempty"`
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

// NewConfig returns a new config
func NewConfig() *Config {
	return &Config{}
}
