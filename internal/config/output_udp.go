package config

import (
	"fmt"
)

// UDPOutputConfig contains configuration for UDP output
type UDPOutputConfig struct {
	// Host is the target host for UDP connections
	Host string `yaml:"host,omitempty" mapstructure:"host,omitempty"`
	// Port is the target port for UDP connections
	Port int `yaml:"port,omitempty" mapstructure:"port,omitempty"`
	// Workers is the number of worker goroutines for UDP output
	Workers int `yaml:"workers,omitempty" mapstructure:"workers,omitempty"`
}

// Validate validates the UDP output configuration
func (c *UDPOutputConfig) Validate() error {
	if err := ValidateHost(c.Host); err != nil {
		return fmt.Errorf("UDP output host validation failed: %w", err)
	}

	if err := ValidatePort(c.Port); err != nil {
		return fmt.Errorf("UDP output port validation failed: %w", err)
	}

	if c.Workers < 0 {
		return fmt.Errorf("UDP output workers cannot be negative, got %d", c.Workers)
	}

	return nil
}
