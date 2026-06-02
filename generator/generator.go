package generator

import (
	"context"

	"github.com/observiq/blitz/output"
	"github.com/observiq/blitz/telemetry"
)

// MetricGenerator is the legacy interface for generating metric data.
// Retained until generator/hostmetrics migrates to embed.MetricConsumer
// (PIPE-1023); will be removed in that PR alongside the hostmetrics
// migration.
type MetricGenerator interface {
	// Start starts the metric generator and writes data using the
	// provided metric writer.
	Start(writer output.MetricWriter) error

	// Stop stops the metric generator.
	Stop(ctx context.Context) error

	// SupportedTelemetry returns the telemetry types this generator produces.
	SupportedTelemetry() []telemetry.Type
}

// TraceGenerator is the legacy interface for generating trace data.
// Retained until generator/traces migrates to embed.TraceConsumer
// (PIPE-1024); will be removed in that PR alongside the traces
// migration.
type TraceGenerator interface {
	// Start starts the trace generator and writes data using the
	// provided trace writer.
	Start(writer output.TraceWriter) error

	// Stop stops the trace generator.
	Stop(ctx context.Context) error

	// SupportedTelemetry returns the telemetry types this generator produces.
	SupportedTelemetry() []telemetry.Type
}
