package stdout

import (
	"fmt"
	"time"

	"github.com/observiq/blitz/embed"
)

const defaultFlushInterval = 100 * time.Millisecond

// Option is a functional option for configuring the stdout output.
type Option func(*config) error

type config struct {
	flushInterval time.Duration
	tel           embed.TelemetrySettings
}

// WithTelemetry sets the OTel providers blitz routes its self-telemetry
// through: metrics via tel.MeterProvider, the log bridge via tel.LoggerProvider,
// and the gated flush span via tel.TracerProvider.
func WithTelemetry(tel embed.TelemetrySettings) Option {
	return func(c *config) error {
		c.tel = tel
		return nil
	}
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
