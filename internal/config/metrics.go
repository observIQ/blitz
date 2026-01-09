package config

const (
	// DefaultMetricsPort is the default port for the metrics HTTP server
	DefaultMetricsPort = 9100
)

// Metrics is the configuration for metrics
type Metrics struct {
	// Port is the HTTP port for the metrics endpoint
	Port int `yaml:"port" mapstructure:"port"`
}

// Validate validates the metrics configuration
func (m *Metrics) Validate() error {
	return ValidatePort(m.Port)
}
