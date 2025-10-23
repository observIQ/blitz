package config

import (
	"fmt"
)

// OutputType represents the type of output
type OutputType string

const (
	// OutputTypeNop represents NOP output
	OutputTypeNop OutputType = "nop"
	// OutputTypeTCP represents TCP output
	OutputTypeTCP OutputType = "tcp"
	// OutputTypeUDP represents UDP output
	OutputTypeUDP OutputType = "udp"
)

// Output contains configuration for output destinations
type Output struct {
	// Type specifies the output type (tcp or udp)
	Type OutputType `yaml:"type,omitempty" mapstructure:"type,omitempty"`
	// UDP contains UDP output configuration
	UDP UDPOutputConfig `yaml:"udp,omitempty" mapstructure:"udp,omitempty"`
	// TCP contains TCP output configuration
	TCP TCPOutputConfig `yaml:"tcp,omitempty" mapstructure:"tcp,omitempty"`
}

// Validate validates the output configuration
func (o *Output) Validate() error {
	// Allow empty type - defaults will be applied by override system
	if o.Type == "" {
		return nil
	}

	switch o.Type {
	case OutputTypeNop:
		// NOP output requires no additional validation
	case OutputTypeTCP:
		if err := o.TCP.Validate(); err != nil {
			return fmt.Errorf("TCP output validation failed: %w", err)
		}
	case OutputTypeUDP:
		if err := o.UDP.Validate(); err != nil {
			return fmt.Errorf("UDP output validation failed: %w", err)
		}
	default:
		return fmt.Errorf("invalid output type: %s, must be one of: nop, tcp, udp", o.Type)
	}

	return nil
}
