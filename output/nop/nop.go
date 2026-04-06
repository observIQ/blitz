package nop

import (
	"context"
	"fmt"

	"github.com/observiq/blitz/output"
	"github.com/observiq/blitz/telemetry"
	"go.uber.org/zap"
)

// NopOutput is a no-operation output that performs no work
type NopOutput struct {
	logger *zap.Logger
}

// New creates a new no-operation output
func New(logger *zap.Logger) (*NopOutput, error) {
	if logger == nil {
		return nil, fmt.Errorf("logger cannot be nil")
	}

	return &NopOutput{
		logger: logger.Named("output-nop"),
	}, nil
}

// Write performs no work (data is discarded)
func (o *NopOutput) Write(ctx context.Context, data output.LogRecord) error {
	// No-op: data is discarded
	return nil
}

// Stop performs no work
func (o *NopOutput) Stop(ctx context.Context) error {
	o.logger.Info("Stopping NOP output")
	return nil
}

// SupportedTelemetry returns the telemetry types this output can consume.
func (o *NopOutput) SupportedTelemetry() []telemetry.Type {
	return []telemetry.Type{telemetry.Logs}
}
