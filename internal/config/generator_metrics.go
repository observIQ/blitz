package config

import (
	"fmt"
	"time"
)

// MetricDefinition describes a single metric to generate.
type MetricDefinition struct {
	// Name is the metric name (e.g. "system.cpu.utilization").
	Name string `yaml:"name" mapstructure:"name"`
	// Type is the metric type: "gauge" or "sum".
	Type string `yaml:"type" mapstructure:"type"`
	// Description is an optional human-readable description.
	Description string `yaml:"description,omitempty" mapstructure:"description,omitempty"`
	// Unit is the metric unit (e.g. "s", "By", "1", "%").
	Unit string `yaml:"unit,omitempty" mapstructure:"unit,omitempty"`
	// Attributes are key-value pairs attached to every data point.
	Attributes map[string]string `yaml:"attributes,omitempty" mapstructure:"attributes,omitempty"`
	// ValueMin is the minimum value for generated data points (inclusive).
	ValueMin float64 `yaml:"valueMin,omitempty" mapstructure:"valueMin,omitempty"`
	// ValueMax is the maximum value for generated data points (inclusive).
	ValueMax float64 `yaml:"valueMax,omitempty" mapstructure:"valueMax,omitempty"`
}

// Validate validates a single metric definition.
func (m *MetricDefinition) Validate() error {
	if m.Name == "" {
		return fmt.Errorf("metric name is required")
	}
	switch m.Type {
	case "gauge", "sum":
	default:
		return fmt.Errorf("metric %q: type must be \"gauge\" or \"sum\", got %q", m.Name, m.Type)
	}
	if m.ValueMax < m.ValueMin {
		return fmt.Errorf("metric %q: valueMax (%g) must be >= valueMin (%g)", m.Name, m.ValueMax, m.ValueMin)
	}
	return nil
}

// MetricsGeneratorConfig contains configuration for the metrics generator.
type MetricsGeneratorConfig struct {
	// Workers is the number of worker goroutines.
	Workers int `yaml:"workers,omitempty" mapstructure:"workers,omitempty"`
	// Rate is the generation interval per worker.
	Rate time.Duration `yaml:"rate,omitempty" mapstructure:"rate,omitempty"`
	// ResourceAttributes maps keys to one-or-more values. The generator
	// emits data points for each combination of resource attribute values.
	ResourceAttributes map[string][]string `yaml:"resourceAttributes,omitempty" mapstructure:"resourceAttributes,omitempty"`
	// Metrics is the list of metric definitions to generate.
	Metrics []MetricDefinition `yaml:"metrics,omitempty" mapstructure:"metrics,omitempty"`
}

// Validate validates the metrics generator configuration.
func (c *MetricsGeneratorConfig) Validate() error {
	if c.Workers < 1 {
		return fmt.Errorf("metrics generator workers must be 1 or greater, got %d", c.Workers)
	}
	if c.Rate <= 0 {
		return fmt.Errorf("metrics generator rate must be positive, got %v", c.Rate)
	}
	if len(c.Metrics) == 0 {
		return fmt.Errorf("metrics generator requires at least one metric definition")
	}
	for k, vals := range c.ResourceAttributes {
		if len(vals) == 0 {
			return fmt.Errorf("resourceAttribute %q must have at least one value", k)
		}
	}
	for i := range c.Metrics {
		if err := c.Metrics[i].Validate(); err != nil {
			return fmt.Errorf("metrics[%d]: %w", i, err)
		}
	}
	return nil
}
