package generator

import (
	"context"
	"fmt"

	"github.com/observiq/blitz/output"
	"go.uber.org/zap"
)

// NopGenerator is a no-operation generator that performs no work
type NopGenerator struct {
	logger *zap.Logger
}

// NewNopGenerator creates a new no-operation generator
func NewNopGenerator(logger *zap.Logger) (*NopGenerator, error) {
	if logger == nil {
		return nil, fmt.Errorf("logger cannot be nil")
	}

	return &NopGenerator{
		logger: logger.Named("generator-nop"),
	}, nil
}

// Start starts the nop generator (performs no work)
func (g *NopGenerator) Start(writer output.Writer) error {
	g.logger.Info("Starting NOP generator (no work performed)")
	return nil
}

// Stop stops the nop generator (performs no work)
func (g *NopGenerator) Stop(ctx context.Context) error {
	g.logger.Info("Stopping NOP generator")
	return nil
}
