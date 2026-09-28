package config

import (
	"fmt"
	"net/url"
	"time"
)

// Remote-write protocol versions.
const (
	// PromRWVersion1 is classic remote-write 1.0 (snappy + prompb.WriteRequest).
	PromRWVersion1 = "1.0"
	// PromRWVersion2 is remote-write 2.0 (symbol table, metadata, created timestamps).
	PromRWVersion2 = "2.0"
)

// Default prometheus-remote-write output configuration values.
const (
	DefaultPromRWVersion      = PromRWVersion1
	DefaultPromRWBatchSize    = 500
	DefaultPromRWBatchTimeout = 5 * time.Second
	DefaultPromRWTimeout      = 30 * time.Second
)

// PrometheusRemoteWriteOutputConfig contains configuration for the
// prometheus-remote-write output.
type PrometheusRemoteWriteOutputConfig struct {
	// Endpoint is the full remote-write URL (e.g. http://host:9090/api/v1/write).
	Endpoint string `yaml:"endpoint,omitempty" mapstructure:"endpoint,omitempty"`
	// Version selects the remote-write wire protocol: "1.0" or "2.0".
	Version string `yaml:"version,omitempty" mapstructure:"version,omitempty"`
	// BatchSize is the maximum number of series buffered before a flush.
	BatchSize int `yaml:"batchSize,omitempty" mapstructure:"batchSize,omitempty"`
	// BatchTimeout is the maximum time to wait before flushing a partial batch.
	BatchTimeout time.Duration `yaml:"batchTimeout,omitempty" mapstructure:"batchTimeout,omitempty"`
	// Timeout is the per-request HTTP timeout.
	Timeout time.Duration `yaml:"timeout,omitempty" mapstructure:"timeout,omitempty"`
	// Headers are extra HTTP headers sent on every request (e.g. authorization).
	Headers map[string]string `yaml:"headers,omitempty" mapstructure:"headers,omitempty"`
}

// Validate validates the prometheus-remote-write output configuration.
func (c *PrometheusRemoteWriteOutputConfig) Validate() error {
	if c.Endpoint == "" {
		return fmt.Errorf("prometheus-remote-write output endpoint cannot be empty")
	}
	u, err := url.Parse(c.Endpoint)
	if err != nil {
		return fmt.Errorf("prometheus-remote-write output endpoint is not a valid URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("prometheus-remote-write output endpoint scheme must be http or https, got %q", u.Scheme)
	}

	switch c.Version {
	case "", PromRWVersion1, PromRWVersion2:
	default:
		return fmt.Errorf("prometheus-remote-write output version must be one of: 1.0, 2.0; got %q", c.Version)
	}

	if c.BatchSize < 0 {
		return fmt.Errorf("prometheus-remote-write output batch size cannot be negative, got %d", c.BatchSize)
	}
	if c.BatchTimeout < 0 {
		return fmt.Errorf("prometheus-remote-write output batch timeout cannot be negative, got %s", c.BatchTimeout)
	}
	if c.Timeout < 0 {
		return fmt.Errorf("prometheus-remote-write output timeout cannot be negative, got %s", c.Timeout)
	}
	return nil
}
