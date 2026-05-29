package config

import (
	"fmt"
)

// OutputType represents the type of output
type OutputType string

const (
	// OutputTypeNop represents NOP output
	OutputTypeNop OutputType = "nop"
	// OutputTypeStdout represents stdout output
	OutputTypeStdout OutputType = "stdout"
	// OutputTypeTCP represents TCP output
	OutputTypeTCP OutputType = "tcp"
	// OutputTypeUDP represents UDP output
	OutputTypeUDP OutputType = "udp"
	// OutputTypeSyslog represents Syslog output (wrapping TCP/UDP)
	OutputTypeSyslog OutputType = "syslog"
	// OutputTypeOTLPGrpc represents OTLP gRPC output
	OutputTypeOTLPGrpc OutputType = "otlp-grpc"
	// OutputTypeFile represents File output
	OutputTypeFile OutputType = "file"
	// OutputTypeHEC represents Splunk HEC output
	OutputTypeHEC OutputType = "hec"
)

// Output contains configuration for output destinations
type Output struct {
	// Type specifies the output type (tcp, udp, or otlp-grpc)
	Type OutputType `yaml:"type,omitempty" mapstructure:"type,omitempty"`
	// UDP contains UDP output configuration
	UDP UDPOutputConfig `yaml:"udp,omitempty" mapstructure:"udp,omitempty"`
	// TCP contains TCP output configuration
	TCP TCPOutputConfig `yaml:"tcp,omitempty" mapstructure:"tcp,omitempty"`
	// Syslog contains Syslog output configuration
	Syslog SyslogOutputConfig `yaml:"syslog,omitempty" mapstructure:"syslog,omitempty"`
	// OTLPGrpc contains OTLP gRPC output configuration
	OTLPGrpc OTLPGrpcOutputConfig `yaml:"otlp-grpc,omitempty" mapstructure:"otlp-grpc,omitempty"`
	// File contains File output configuration
	File FileOutputConfig `yaml:"file,omitempty" mapstructure:"file,omitempty"`
	// HEC contains Splunk HEC output configuration
	HEC HECOutputConfig `yaml:"hec,omitempty" mapstructure:"hec,omitempty"`
	// Stdout contains stdout output configuration
	Stdout StdoutOutputConfig `yaml:"stdout,omitempty" mapstructure:"stdout,omitempty"`
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
	case OutputTypeStdout:
		if err := o.Stdout.Validate(); err != nil {
			return fmt.Errorf("Stdout output validation failed: %w", err)
		}
	case OutputTypeTCP:
		if err := o.TCP.Validate(); err != nil {
			return fmt.Errorf("TCP output validation failed: %w", err)
		}
	case OutputTypeUDP:
		if err := o.UDP.Validate(); err != nil {
			return fmt.Errorf("UDP output validation failed: %w", err)
		}
	case OutputTypeSyslog:
		if err := o.Syslog.Validate(); err != nil {
			return fmt.Errorf("Syslog output validation failed: %w", err)
		}
	case OutputTypeOTLPGrpc:
		if err := o.OTLPGrpc.Validate(); err != nil {
			return fmt.Errorf("OTLP gRPC output validation failed: %w", err)
		}
	case OutputTypeFile:
		if err := o.File.Validate(); err != nil {
			return fmt.Errorf("File output validation failed: %w", err)
		}
	case OutputTypeHEC:
		if err := o.HEC.Validate(); err != nil {
			return fmt.Errorf("HEC output validation failed: %w", err)
		}
	default:
		return fmt.Errorf("invalid output type: %s, must be one of: nop, stdout, tcp, udp, syslog, otlp-grpc, file, hec", o.Type)
	}

	return nil
}
