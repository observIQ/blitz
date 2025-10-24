package config

import (
	"fmt"
)

// TCPOutputConfig contains configuration for TCP output
type TCPOutputConfig struct {
	// Host is the target host for TCP connections
	Host string `yaml:"host,omitempty" mapstructure:"host,omitempty"`
	// Port is the target port for TCP connections
	Port int `yaml:"port,omitempty" mapstructure:"port,omitempty"`
	// Workers is the number of worker goroutines for TCP output
	Workers int `yaml:"workers,omitempty" mapstructure:"workers,omitempty"`

	TLS `yaml:",inline"`
}

// Validate validates the TCP output configuration
func (c *TCPOutputConfig) Validate() error {
	if err := ValidateHost(c.Host); err != nil {
		return fmt.Errorf("TCP output host validation failed: %w", err)
	}

	if err := ValidatePort(c.Port); err != nil {
		return fmt.Errorf("TCP output port validation failed: %w", err)
	}

	if c.Workers < 0 {
		return fmt.Errorf("TCP output workers cannot be negative, got %d", c.Workers)
	}

	if err := c.TLS.Validate(); err != nil {
		return fmt.Errorf("TCP output TLS validation failed: %w", err)
	}

	return nil
}
