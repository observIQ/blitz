package config

// Telemetry configures export of blitz's OWN self-telemetry (its internal
// logs, metrics, and traces). This is distinct from the data blitz generates.
// The existing `metrics` block still governs the Prometheus scrape endpoint
// for self-metrics; this block adds OTLP export for self-traces (and, in a
// later phase, self-logs).
type Telemetry struct {
	// Traces configures OTLP export of blitz's internal spans.
	Traces TracesTelemetry `yaml:"traces,omitempty" mapstructure:"traces,omitempty"`

	// Logs configures OTLP export of blitz's internal logs.
	Logs LogsTelemetry `yaml:"logs,omitempty" mapstructure:"logs,omitempty"`
}

// LogsTelemetry configures OTLP gRPC export of blitz's internal logs. When an
// endpoint is set, blitz's internal zap logging is bridged into an OTel
// LoggerProvider and exported alongside the usual zap output.
type LogsTelemetry struct {
	// OTLPEndpoint is the OTLP gRPC endpoint (host:port). Empty disables log
	// export: blitz logs stay zap-only, matching the behavior before OTel logs
	// were supported.
	OTLPEndpoint string `yaml:"otlpEndpoint,omitempty" mapstructure:"otlpEndpoint,omitempty"`

	// Insecure sends logs over plaintext gRPC (no TLS). Defaults to false.
	Insecure bool `yaml:"insecure,omitempty" mapstructure:"insecure,omitempty"`
}

// TracesTelemetry configures OTLP gRPC export of blitz's internal spans.
type TracesTelemetry struct {
	// OTLPEndpoint is the OTLP gRPC endpoint (host:port). Empty disables trace
	// export: spans are still created but routed to a no-op provider.
	OTLPEndpoint string `yaml:"otlpEndpoint,omitempty" mapstructure:"otlpEndpoint,omitempty"`

	// Insecure sends spans over plaintext gRPC (no TLS). Defaults to false.
	Insecure bool `yaml:"insecure,omitempty" mapstructure:"insecure,omitempty"`

	// PerBatchSpans enables the higher-volume per-emit-cycle spans. Off by
	// default; the coarse session and generator-lifecycle spans do not depend
	// on it.
	PerBatchSpans bool `yaml:"perBatchSpans,omitempty" mapstructure:"perBatchSpans,omitempty"`
}

// Validate validates the telemetry configuration. Export is off by default and
// all fields are optional, so there is nothing to reject today; the method
// exists to match the config-block Validate convention and to host future
// checks (e.g. endpoint format).
func (t Telemetry) Validate() error {
	return nil
}
