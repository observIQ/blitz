package generator

import (
	"context"

	"github.com/observiq/blitz/output"
	"github.com/observiq/blitz/telemetry"
)

// Generator is the interface for generating log data.
type Generator interface {
	// Start starts the generator and writes data using the
	// provided generator writer.
	Start(writer output.Writer) error

	// Stop stops the generator.
	Stop(ctx context.Context) error

	// SupportedTelemetry returns the telemetry types this generator produces.
	SupportedTelemetry() []telemetry.Type
}

// MetricGenerator is the interface for generating metric data.
type MetricGenerator interface {
	// Start starts the metric generator and writes data using the
	// provided metric writer.
	Start(writer output.MetricWriter) error

	// Stop stops the metric generator.
	Stop(ctx context.Context) error

	// SupportedTelemetry returns the telemetry types this generator produces.
	SupportedTelemetry() []telemetry.Type
}

// TraceGenerator is the interface for generating trace data.
type TraceGenerator interface {
	// Start starts the trace generator and writes data using the
	// provided trace writer.
	Start(writer output.TraceWriter) error

	// Stop stops the trace generator.
	Stop(ctx context.Context) error

	// SupportedTelemetry returns the telemetry types this generator produces.
	SupportedTelemetry() []telemetry.Type
}
