package generator

import "context"

type generatorWriter interface {
	Write(ctx context.Context, data []byte) error
}

// Generator is the interface for generating data.
type Generator interface {
	// Start starts the generator and writes data using the
	// provided generator writer.
	Start(writer generatorWriter) error

	// Stop stops the generator.
	Stop(ctx context.Context) error
}
