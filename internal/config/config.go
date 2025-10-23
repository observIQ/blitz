// Package config contains the top level configuration structures and logic
// Package config contains the top level configuration structures and logic
package config

// Config is the configuration for bindplane-loader.
type Config struct {
	// Logging configuration for the logger
	Logging Logging `yaml:"logging,omitempty" mapstructure:"logging,omitempty"`
}

// Validate validates the entire configuration
func (c *Config) Validate() error {
	if err := c.Logging.Validate(); err != nil {
		return err
	}
	return nil
}

// NewConfig returns a new config
func NewConfig() *Config {
	return &Config{}
}
