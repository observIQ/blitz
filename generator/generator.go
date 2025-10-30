package generator

import (
	"context"

	"github.com/observiq/blitz/output"
)

// Generator is the interface for generating data.
type Generator interface {
	// Start starts the generator and writes data using the
	// provided generator writer.
	Start(writer output.Writer) error

	// Stop stops the generator.
	Stop(ctx context.Context) error
}
