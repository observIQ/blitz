package nop

import (
	"context"
	"fmt"

	"github.com/observiq/blitz/embed"
	"github.com/observiq/blitz/output"
	"github.com/observiq/blitz/telemetry"
	"go.uber.org/zap"
)

// outputType is the output_type attribute value for nop metrics.
const outputType = "nop"

// NopOutput is a no-operation output that discards every record. It still
// records how many records it received, so a load run can measure how much was
// pushed into the void, and it bridges its logger like every other output.
type NopOutput struct {
	logger  *zap.Logger
	metrics *output.Metrics
}

// New creates a new no-operation output. tel carries blitz's self-telemetry
// providers: metrics route through tel.MeterProvider and the logger is bridged
// into tel.LoggerProvider.
func New(logger *zap.Logger, tel embed.TelemetrySettings) (*NopOutput, error) {
	if logger == nil {
		return nil, fmt.Errorf("logger cannot be nil")
	}

	m, err := output.NewMetrics(tel.MeterProvider)
	if err != nil {
		return nil, fmt.Errorf("build output metrics: %w", err)
	}

	return &NopOutput{
		logger:  logger.Named("output-nop"),
		metrics: m,
	}, nil
}

// Write discards the record after counting it. The write is a synchronous
// no-op, already bracketed by the consumer adapter's emit span, so it gets no
// span of its own.
func (o *NopOutput) Write(ctx context.Context, data output.LogRecord) error {
	o.metrics.BlitzOutputEntriesReceivedCounter.Add(ctx, 1, outputType, "logs")
	return nil
}

// Stop performs no work
func (o *NopOutput) Stop(_ context.Context) error {
	o.logger.Info("Stopping NOP output")
	return nil
}

// SupportedTelemetry returns the telemetry types this output can consume.
func (o *NopOutput) SupportedTelemetry() []telemetry.Type {
	return []telemetry.Type{telemetry.Logs}
}
