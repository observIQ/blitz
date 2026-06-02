package generator

import (
	"context"

	"github.com/observiq/blitz/output"
	"github.com/observiq/blitz/telemetry"
)

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
