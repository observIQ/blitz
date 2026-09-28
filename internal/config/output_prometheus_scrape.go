package config

import (
	"fmt"
	"net"
	"strings"
	"time"
)

// Default prometheus-scrape output configuration values.
const (
	// DefaultPromScrapeListenAddress is the OTel Prometheus exporter default port.
	DefaultPromScrapeListenAddress = "0.0.0.0:9464"
	// DefaultPromScrapeMetricsPath is the conventional exposition path.
	DefaultPromScrapeMetricsPath = "/metrics"
	// DefaultPromScrapeMetricExpiration matches the collector prometheusexporter.
	DefaultPromScrapeMetricExpiration = 5 * time.Minute
)

// PrometheusScrapeOutputConfig contains configuration for the prometheus-scrape
// output: an HTTP /metrics endpoint served in Prometheus text exposition format.
type PrometheusScrapeOutputConfig struct {
	// ListenAddress is the host:port the metrics endpoint binds to.
	ListenAddress string `yaml:"listenAddress,omitempty" mapstructure:"listenAddress,omitempty"`
	// MetricsPath is the URL path the exposition is served on (e.g. /metrics).
	MetricsPath string `yaml:"metricsPath,omitempty" mapstructure:"metricsPath,omitempty"`
	// EmitTimestamps, when true, appends each sample's millisecond timestamp to
	// the exposition. Default false lets the scraper stamp at scrape time.
	EmitTimestamps bool `yaml:"emitTimestamps,omitempty" mapstructure:"emitTimestamps,omitempty"`
	// MetricExpiration drops a series not updated within it, so churned or
	// stopped hosts stop being exposed. 0 keeps series forever.
	MetricExpiration time.Duration `yaml:"metricExpiration,omitempty" mapstructure:"metricExpiration,omitempty"`
}

// Validate validates the prometheus-scrape output configuration. Empty fields
// are allowed: the override system fills defaults before validation.
func (c *PrometheusScrapeOutputConfig) Validate() error {
	if c.ListenAddress != "" {
		if _, _, err := net.SplitHostPort(c.ListenAddress); err != nil {
			return fmt.Errorf("prometheus-scrape output listen address is not a valid host:port: %w", err)
		}
	}
	if c.MetricExpiration < 0 {
		return fmt.Errorf("prometheus-scrape output metric expiration cannot be negative, got %s", c.MetricExpiration)
	}
	if c.MetricsPath != "" && !strings.HasPrefix(c.MetricsPath, "/") {
		return fmt.Errorf("prometheus-scrape output metrics path must start with '/', got %q", c.MetricsPath)
	}
	return nil
}
