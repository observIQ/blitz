package output

import (
	"context"
	"fmt"

	"go.uber.org/zap"
)

// NopOutput is a no-operation output that performs no work
type NopOutput struct {
	logger *zap.Logger
}

// NewNopOutput creates a new no-operation output
func NewNopOutput(logger *zap.Logger) (*NopOutput, error) {
	if logger == nil {
		return nil, fmt.Errorf("logger cannot be nil")
	}

	return &NopOutput{
		logger: logger.Named("output-nop"),
	}, nil
}

// Write performs no work (data is discarded)
func (o *NopOutput) Write(ctx context.Context, data []byte) error {
	// No-op: data is discarded
	return nil
}

// Stop performs no work
func (o *NopOutput) Stop(ctx context.Context) error {
	o.logger.Info("Stopping NOP output")
	return nil
}
