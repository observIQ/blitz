// Package config contains the top level configuration structures and logic
package config

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
