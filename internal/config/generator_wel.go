package config

import (
	"fmt"
	"time"
)

// WelGeneratorConfig contains configuration for the Windows Event Log generator.
type WelGeneratorConfig struct {
	// Workers is the number of worker goroutines
	Workers int `yaml:"workers,omitempty" mapstructure:"workers,omitempty"`
	// Rate is the generation interval per worker
	Rate time.Duration `yaml:"rate,omitempty" mapstructure:"rate,omitempty"`
	// Channels is the list of channels to generate events for (empty = all)
	Channels []string `yaml:"channels,omitempty" mapstructure:"channels,omitempty"`
	// Computer is the computer name for generated events
	Computer string `yaml:"computer,omitempty" mapstructure:"computer,omitempty"`
	// Domain is the domain name for Active Directory-related events
	Domain string `yaml:"domain,omitempty" mapstructure:"domain,omitempty"`
	// Role controls which event pool is eligible (workstation, member, dc)
	Role string `yaml:"role,omitempty" mapstructure:"role,omitempty"`
	// ManageEventSources controls whether event sources are registered/deregistered automatically
	ManageEventSources bool `yaml:"manageEventSources,omitempty" mapstructure:"manageEventSources,omitempty"`
}

// Validate validates the WEL generator configuration.
func (c *WelGeneratorConfig) Validate() error {
	if c.Workers < 1 {
		return fmt.Errorf("wel generator workers must be 1 or greater, got %d", c.Workers)
	}
	if c.Rate <= 0 {
		return fmt.Errorf("wel generator rate must be positive, got %v", c.Rate)
	}
	switch c.Role {
	case "", "workstation", "member", "dc":
		// valid
	default:
		return fmt.Errorf("wel generator role must be one of: workstation, member, dc; got %q", c.Role)
	}
	return nil
}
