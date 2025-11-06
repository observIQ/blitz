package stdout

import (
	"context"
	"fmt"
	"os"

	"github.com/observiq/blitz/output"
	"go.uber.org/zap"
)

// StdoutOutput writes log records to standard output
type StdoutOutput struct {
	logger *zap.Logger
}

// New creates a new stdout output
func New(logger *zap.Logger) (*StdoutOutput, error) {
	if logger == nil {
		return nil, fmt.Errorf("logger cannot be nil")
	}

	return &StdoutOutput{
		logger: logger.Named("output-stdout"),
	}, nil
}

// Write writes the log record to stdout
func (o *StdoutOutput) Write(ctx context.Context, data output.LogRecord) error {
	_, err := fmt.Fprintln(os.Stdout, data.Message)
	return err
}

// Stop performs cleanup
func (o *StdoutOutput) Stop(ctx context.Context) error {
	o.logger.Info("Stopping stdout output")
	return nil
}
