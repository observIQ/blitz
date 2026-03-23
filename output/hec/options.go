package hec

import (
	"crypto/tls"
	"time"
)

// Option is a functional option for configuring the HEC output
type Option func(*Config) error

// Config holds configuration for HEC output
type Config struct {
	host            string
	port            string
	token           string
	workers         int
	batchSize       int
	batchTimeout    time.Duration
	eventFormat     string
	enableACK       bool
	ackPollInterval time.Duration
	ackTimeout      time.Duration
	maxRetries      int
	source          string
	sourceType      string
	index           string
	enableTLS       bool
	tlsConfig       *tls.Config
}

// WithHost sets the target host for HEC connections
func WithHost(host string) Option {
	return func(cfg *Config) error {
		cfg.host = host
		return nil
	}
}

// WithPort sets the target port for HEC connections
func WithPort(port string) Option {
	return func(cfg *Config) error {
		cfg.port = port
		return nil
	}
}

// WithToken sets the Splunk HEC authentication token
func WithToken(token string) Option {
	return func(cfg *Config) error {
		cfg.token = token
		return nil
	}
}

// WithWorkers sets the number of worker goroutines
func WithWorkers(workers int) Option {
	return func(cfg *Config) error {
		cfg.workers = workers
		return nil
	}
}

// WithBatchSize sets the maximum number of events per batch
func WithBatchSize(size int) Option {
	return func(cfg *Config) error {
		cfg.batchSize = size
		return nil
	}
}

// WithBatchTimeout sets the maximum time to wait before flushing a partial batch
func WithBatchTimeout(timeout time.Duration) Option {
	return func(cfg *Config) error {
		cfg.batchTimeout = timeout
		return nil
	}
}

// WithEventFormat sets the event format: "raw" or "parsed"
func WithEventFormat(format string) Option {
	return func(cfg *Config) error {
		cfg.eventFormat = format
		return nil
	}
}

// WithEnableACK enables or disables Splunk indexer acknowledgement
func WithEnableACK(enable bool) Option {
	return func(cfg *Config) error {
		cfg.enableACK = enable
		return nil
	}
}

// WithACKPollInterval sets how often to poll for ACK status
func WithACKPollInterval(interval time.Duration) Option {
	return func(cfg *Config) error {
		cfg.ackPollInterval = interval
		return nil
	}
}

// WithACKTimeout sets how long to wait for an ACK before resending
func WithACKTimeout(timeout time.Duration) Option {
	return func(cfg *Config) error {
		cfg.ackTimeout = timeout
		return nil
	}
}

// WithMaxRetries sets the maximum number of resend attempts per batch
func WithMaxRetries(retries int) Option {
	return func(cfg *Config) error {
		cfg.maxRetries = retries
		return nil
	}
}

// WithSource sets the default source metadata for HEC events
func WithSource(source string) Option {
	return func(cfg *Config) error {
		cfg.source = source
		return nil
	}
}

// WithSourceType sets the default sourcetype metadata for HEC events
func WithSourceType(sourceType string) Option {
	return func(cfg *Config) error {
		cfg.sourceType = sourceType
		return nil
	}
}

// WithIndex sets the target index for HEC events
func WithIndex(index string) Option {
	return func(cfg *Config) error {
		cfg.index = index
		return nil
	}
}

// WithEnableTLS enables TLS for HEC connections
func WithEnableTLS(enable bool) Option {
	return func(cfg *Config) error {
		cfg.enableTLS = enable
		return nil
	}
}

// WithTLSConfig sets the TLS configuration for secure connections
func WithTLSConfig(tlsConfig *tls.Config) Option {
	return func(cfg *Config) error {
		cfg.tlsConfig = tlsConfig
		return nil
	}
}
