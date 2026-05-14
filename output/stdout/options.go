package stdout

import (
	"fmt"
	"time"
)

const defaultFlushInterval = 100 * time.Millisecond

// Option is a functional option for configuring the stdout output.
type Option func(*config) error

type config struct {
	flushInterval time.Duration
}

// WithFlushInterval sets the interval at which the internal buffer is flushed to stdout.
func WithFlushInterval(d time.Duration) Option {
	return func(c *config) error {
		if d <= 0 {
			return fmt.Errorf("flush interval must be > 0, got %s", d)
		}
		c.flushInterval = d
		return nil
	}
}
