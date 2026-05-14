package config

import (
	"fmt"
	"time"
)

// DefaultStdoutFlushInterval is the default interval for flushing the stdout buffer.
const DefaultStdoutFlushInterval = 100 * time.Millisecond

// StdoutOutputConfig contains configuration for the stdout output.
type StdoutOutputConfig struct {
	// FlushInterval is how often the internal buffer is flushed to stdout.
	FlushInterval time.Duration `yaml:"flushInterval,omitempty" mapstructure:"flushInterval,omitempty"`
}

// Validate validates the stdout output configuration.
func (c *StdoutOutputConfig) Validate() error {
	if c.FlushInterval < 0 {
		return fmt.Errorf("stdout output flush interval cannot be negative, got %s", c.FlushInterval)
	}
	return nil
}
