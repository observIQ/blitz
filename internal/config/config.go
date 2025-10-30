// Package config contains the top level configuration structures and logic
package config

import "time"

// Config is the configuration for blitz.
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

	// Apply winevt generator defaults
	if c.Generator.Winevt.Workers == 0 {
		c.Generator.Winevt.Workers = 1
	}
	if c.Generator.Winevt.Rate == 0 {
		c.Generator.Winevt.Rate = 1 * time.Second
	}

	// Apply TCP output defaults
	if c.Output.TCP.Workers == 0 {
		c.Output.TCP.Workers = 1
	}

	// Apply UDP output defaults
	if c.Output.UDP.Workers == 0 {
		c.Output.UDP.Workers = 1
	}

	// Apply OTLP gRPC output defaults
	if c.Output.OTLPGrpc.Host == "" {
		c.Output.OTLPGrpc.Host = DefaultOTLPGrpcHost
	}
	if c.Output.OTLPGrpc.Port == 0 {
		c.Output.OTLPGrpc.Port = DefaultOTLPGrpcPort
	}
	if c.Output.OTLPGrpc.Workers == 0 {
		c.Output.OTLPGrpc.Workers = DefaultOTLPGrpcWorkers
	}
	if c.Output.OTLPGrpc.BatchTimeout == 0 {
		c.Output.OTLPGrpc.BatchTimeout = DefaultOTLPGrpcBatchTimeout
	}
	if c.Output.OTLPGrpc.MaxQueueSize == 0 {
		c.Output.OTLPGrpc.MaxQueueSize = DefaultOTLPGrpcMaxQueueSize
	}
	if c.Output.OTLPGrpc.MaxExportBatchSize == 0 {
		c.Output.OTLPGrpc.MaxExportBatchSize = DefaultOTLPGrpcMaxExportBatchSize
	}
}
