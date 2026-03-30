package config

import (
	"fmt"
	"time"
)

// HEC event format types
const (
	// HECEventFormatRaw sends the raw log message string as the event value
	HECEventFormatRaw = "raw"
	// HECEventFormatParsed uses ParseFunc to send structured JSON as the event value
	HECEventFormatParsed = "parsed"
)

// Default HEC output configuration values
const (
	DefaultHECPort            = 8088
	DefaultHECWorkers         = 1
	DefaultHECBatchSize       = 100
	DefaultHECBatchTimeout    = 5 * time.Second
	DefaultHECEventFormat     = HECEventFormatRaw
	DefaultHECEnableACK       = true
	DefaultHECACKPollInterval = 10 * time.Second
	DefaultHECACKTimeout      = 5 * time.Minute
	DefaultHECMaxRetries      = 3
	DefaultHECSource          = "blitz"
	DefaultHECSourceType      = "_json"
	DefaultHECEnableTLS       = true
)

// HECOutputConfig contains configuration for Splunk HEC output
type HECOutputConfig struct {
	// Host is the target host for HEC connections
	Host string `yaml:"host,omitempty" mapstructure:"host,omitempty"`
	// Port is the target port for HEC connections (default 8088)
	Port int `yaml:"port,omitempty" mapstructure:"port,omitempty"`
	// Token is the Splunk HEC authentication token
	Token string `yaml:"token,omitempty" mapstructure:"token,omitempty"`
	// Workers is the number of worker goroutines for HEC output
	Workers int `yaml:"workers,omitempty" mapstructure:"workers,omitempty"`
	// BatchSize is the maximum number of events per batch
	BatchSize int `yaml:"batchSize,omitempty" mapstructure:"batchSize,omitempty"`
	// BatchTimeout is the maximum time to wait before flushing a partial batch
	BatchTimeout time.Duration `yaml:"batchTimeout,omitempty" mapstructure:"batchTimeout,omitempty"`
	// EventFormat controls how log records are formatted: "raw" or "parsed"
	EventFormat string `yaml:"eventFormat,omitempty" mapstructure:"eventFormat,omitempty"`

	// EnableACK enables Splunk indexer acknowledgement
	EnableACK bool `yaml:"enableAck,omitempty" mapstructure:"enableAck,omitempty"`
	// ACKPollInterval is how often to poll for ACK status
	ACKPollInterval time.Duration `yaml:"ackPollInterval,omitempty" mapstructure:"ackPollInterval,omitempty"`
	// ACKTimeout is how long to wait for an ACK before resending
	ACKTimeout time.Duration `yaml:"ackTimeout,omitempty" mapstructure:"ackTimeout,omitempty"`
	// MaxRetries is the maximum number of resend attempts per batch before dropping
	MaxRetries int `yaml:"maxRetries,omitempty" mapstructure:"maxRetries,omitempty"`

	// Source is the default source metadata for HEC events
	Source string `yaml:"source,omitempty" mapstructure:"source,omitempty"`
	// SourceType is the default sourcetype metadata for HEC events
	SourceType string `yaml:"sourceType,omitempty" mapstructure:"sourceType,omitempty"`
	// Index is the target index for HEC events (empty = token default)
	Index string `yaml:"index,omitempty" mapstructure:"index,omitempty"`

	// EnableTLS enables TLS for HEC connections
	EnableTLS bool `yaml:"enableTLS,omitempty" mapstructure:"enableTLS,omitempty"`

	TLS `yaml:",inline"`
}

// Validate validates the HEC output configuration
func (c *HECOutputConfig) Validate() error {
	if err := ValidateHost(c.Host); err != nil {
		return fmt.Errorf("HEC output host validation failed: %w", err)
	}

	if err := ValidatePort(c.Port); err != nil {
		return fmt.Errorf("HEC output port validation failed: %w", err)
	}

	if c.Token == "" {
		return fmt.Errorf("HEC output token cannot be empty")
	}

	if c.Workers <= 0 {
		return fmt.Errorf("HEC output workers must be at least 1, got %d", c.Workers)
	}

	if c.BatchSize < 0 {
		return fmt.Errorf("HEC output batch size cannot be negative, got %d", c.BatchSize)
	}

	if c.MaxRetries < 0 {
		return fmt.Errorf("HEC output max retries cannot be negative, got %d", c.MaxRetries)
	}

	switch c.EventFormat {
	case "", HECEventFormatRaw, HECEventFormatParsed:
	default:
		return fmt.Errorf("HEC output event format must be one of: raw, parsed; got %q", c.EventFormat)
	}

	if c.EnableTLS {
		if err := c.TLS.Validate(); err != nil {
			return fmt.Errorf("HEC output TLS validation failed: %w", err)
		}
	}

	return nil
}
