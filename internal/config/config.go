// Package config contains the top level configuration structures and logic
package config

import "time"

// Config is the configuration for bindplane-loader.
type Config struct {
	// Logging configuration for the logger
	Logging Logging `yaml:"logging,omitempty" mapstructure:"logging,omitempty"`
	// Generator configuration
	Generator Generator `yaml:"generator,omitempty" mapstructure:"generator,omitempty"`
	// Output configuration
	Output Output `yaml:"output,omitempty" mapstructure:"output,omitempty"`
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
	return nil
}

// NewConfig returns a new config
func NewConfig() *Config {
	return &Config{}
}

// ApplyDefaults applies default values to the configuration
func (c *Config) ApplyDefaults() {
	// Apply generator defaults
	if c.Generator.Type == "" {
		c.Generator.Type = GeneratorTypeNop
	}

	// Apply output defaults
	if c.Output.Type == "" {
		c.Output.Type = OutputTypeNop
	}

	// Apply logging defaults
	if c.Logging.Type == "" {
		c.Logging.Type = LoggingTypeStdout
	}
	if c.Logging.Level == "" {
		c.Logging.Level = LogLevelInfo
	}

	// Apply JSON generator defaults
	if c.Generator.JSON.Workers == 0 {
		c.Generator.JSON.Workers = 1
	}
	if c.Generator.JSON.Rate == 0 {
		c.Generator.JSON.Rate = 1 * time.Second
	}

	// Apply TCP output defaults
	if c.Output.TCP.Workers == 0 {
		c.Output.TCP.Workers = 1
	}

	// Apply UDP output defaults
	if c.Output.UDP.Workers == 0 {
		c.Output.UDP.Workers = 1
	}
}
