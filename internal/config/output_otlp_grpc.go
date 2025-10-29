package config

import (
	"fmt"
	"time"
)

const (
	// DefaultOTLPGrpcHost is the default host for OTLP gRPC connections
	DefaultOTLPGrpcHost = "localhost"
	// DefaultOTLPGrpcPort is the default port for OTLP gRPC connections
	DefaultOTLPGrpcPort = 4317
	// DefaultOTLPGrpcWorkers is the default number of workers for OTLP gRPC output
	DefaultOTLPGrpcWorkers = 1
	// DefaultOTLPGrpcBatchTimeout is the default batch timeout for OTLP gRPC output
	DefaultOTLPGrpcBatchTimeout = 1 * time.Second
	// DefaultOTLPGrpcMaxQueueSize is the default maximum queue size for OTLP gRPC output
	DefaultOTLPGrpcMaxQueueSize = 100
	// DefaultOTLPGrpcMaxExportBatchSize is the default maximum export batch size for OTLP gRPC output
	DefaultOTLPGrpcMaxExportBatchSize = 200
)

// OTLPGrpcOutputConfig contains configuration for OTLP gRPC output
type OTLPGrpcOutputConfig struct {
	// Host is the target host for OTLP gRPC connections
	Host string `yaml:"host,omitempty" mapstructure:"host,omitempty"`
	// Port is the target port for OTLP gRPC connections
	Port int `yaml:"port,omitempty" mapstructure:"port,omitempty"`
	// Workers is the number of worker goroutines for OTLP gRPC output
	Workers int `yaml:"workers,omitempty" mapstructure:"workers,omitempty"`
	// BatchTimeout is the timeout for batching log records
	BatchTimeout time.Duration `yaml:"batchTimeout,omitempty" mapstructure:"batchTimeout,omitempty"`
	// MaxQueueSize is the maximum queue size for batching
	MaxQueueSize int `yaml:"maxQueueSize,omitempty" mapstructure:"maxQueueSize,omitempty"`
	// MaxExportBatchSize is the maximum batch size for export
	MaxExportBatchSize int `yaml:"maxExportBatchSize,omitempty" mapstructure:"maxExportBatchSize,omitempty"`

	// EnableTLS enables TLS for OTLP gRPC connections. When true, the TLS configuration will be used.
	// Note: This is separate from the TLS.Insecure field, which controls whether to use insecure credentials (no TLS).
	EnableTLS bool `yaml:"enableTLS,omitempty" mapstructure:"enableTLS,omitempty"`

	TLS `yaml:",inline"`
}

// Validate validates the OTLP gRPC output configuration
func (c *OTLPGrpcOutputConfig) Validate() error {
	if err := ValidateHost(c.Host); err != nil {
		return fmt.Errorf("OTLP gRPC output host validation failed: %w", err)
	}

	if err := ValidatePort(c.Port); err != nil {
		return fmt.Errorf("OTLP gRPC output port validation failed: %w", err)
	}

	if c.Workers < 0 {
		return fmt.Errorf("OTLP gRPC output workers cannot be negative, got %d", c.Workers)
	}

	if c.MaxQueueSize < 0 {
		return fmt.Errorf("OTLP gRPC output max queue size cannot be negative, got %d", c.MaxQueueSize)
	}

	if c.MaxExportBatchSize < 0 {
		return fmt.Errorf("OTLP gRPC output max export batch size cannot be negative, got %d", c.MaxExportBatchSize)
	}

	if c.EnableTLS {
		if err := c.TLS.Validate(); err != nil {
			return fmt.Errorf("OTLP gRPC output TLS validation failed: %w", err)
		}
	}

	return nil
}
