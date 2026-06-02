package nop

import (
	"context"
	"fmt"

	"github.com/observiq/blitz/embed"
	"github.com/observiq/blitz/telemetry"
	"go.uber.org/zap"
)

const generatorType = "nop"

// NopGenerator is a no-operation generator that performs no work.
type NopGenerator struct {
	embed.ProducerMarker

	logger *zap.Logger
}

// Compile-time assertion that *NopGenerator implements embed.ProducerModule.
var _ embed.ProducerModule = (*NopGenerator)(nil)

// New creates a new no-operation generator.
func New(logger *zap.Logger) (*NopGenerator, error) {
	if logger == nil {
		return nil, fmt.Errorf("logger cannot be nil")
	}

	return &NopGenerator{
		logger: logger.Named("generator-nop"),
	}, nil
}

// Name returns the module identifier for ProducerModule.
func (g *NopGenerator) Name() string { return generatorType }

// Start starts the nop generator (performs no work).
func (g *NopGenerator) Start(_ context.Context) error {
	g.logger.Info("Starting NOP generator (no work performed)")
	return nil
}

// Stop stops the nop generator (performs no work).
func (g *NopGenerator) Stop(_ context.Context) error {
	g.logger.Info("Stopping NOP generator")
	return nil
}

// SupportedTelemetry returns the telemetry types this generator produces.
func (g *NopGenerator) SupportedTelemetry() []telemetry.Type {
	return []telemetry.Type{telemetry.Logs}
}
