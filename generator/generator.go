package generator

import (
	"context"

	"github.com/observiq/blitz/output"
	"github.com/observiq/blitz/telemetry"
)

// Generator is the interface for generating data.
type Generator interface {
	// SupportedTelemetry returns the telemetry types this generator can produce.
	SupportedTelemetry() []telemetry.Type

	// Start starts the generator and writes data using the
	// provided generator writer.
	Start(writer output.Writer) error

	// Stop stops the generator.
	Stop(ctx context.Context) error
}
