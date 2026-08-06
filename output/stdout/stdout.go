package stdout

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/observiq/blitz/embed"
	"github.com/observiq/blitz/output"
	"github.com/observiq/blitz/telemetry"
	"go.uber.org/zap"
)

// outputType is the output_type attribute value for stdout metrics.
const outputType = "stdout"

// StdoutOutput writes log records to standard output via a buffered writer.
// Records are batched in memory and flushed to os.Stdout periodically, reducing
// per-record syscall overhead under high worker counts.
type StdoutOutput struct {
	logger        *zap.Logger
	tel           embed.TelemetrySettings
	metrics       *output.Metrics
	writer        *bufio.Writer
	mu            sync.Mutex
	flushInterval time.Duration
	stopCh        chan struct{}
	doneCh        chan struct{}
}

// New creates a new stdout output. The goroutine that drives periodic flushing
// starts immediately and runs until Stop is called.
func New(logger *zap.Logger, opts ...Option) (*StdoutOutput, error) {
	if logger == nil {
		return nil, fmt.Errorf("logger cannot be nil")
	}

	cfg := &config{
		flushInterval: defaultFlushInterval,
	}

	for _, opt := range opts {
		if err := opt(cfg); err != nil {
			return nil, fmt.Errorf("invalid option: %w", err)
		}
	}

	m, err := output.NewMetrics(cfg.tel.MeterProvider)
	if err != nil {
		return nil, fmt.Errorf("build output metrics: %w", err)
	}

	o := &StdoutOutput{
		logger:        logger.Named("output-stdout"),
		tel:           cfg.tel,
		metrics:       m,
		writer:        bufio.NewWriter(os.Stdout),
		flushInterval: cfg.flushInterval,
		stopCh:        make(chan struct{}),
		doneCh:        make(chan struct{}),
	}

	go o.flushLoop()

	return o, nil
}

// Write buffers the log record for the next flush.
func (o *StdoutOutput) Write(ctx context.Context, data output.LogRecord) error {
	o.metrics.BlitzOutputEntriesReceivedCounter.Add(ctx, 1, outputType, "logs")

	o.mu.Lock()
	defer o.mu.Unlock()

	if _, err := o.writer.WriteString(data.Message); err != nil {
		return err
	}
	return o.writer.WriteByte('\n')
}

// Stop signals the flush goroutine to exit and performs a best-effort flush of
// any buffered records. Records remaining in the buffer on a hard kill are lost.
func (o *StdoutOutput) Stop(ctx context.Context) error {
	o.logger.Info("Stopping stdout output")
	close(o.stopCh)

	select {
	case <-o.doneCh:
	case <-ctx.Done():
	}

	o.mu.Lock()
	_ = o.writer.Flush()
	o.mu.Unlock()

	return nil
}

func (o *StdoutOutput) flushLoop() {
	ticker := time.NewTicker(o.flushInterval)
	defer ticker.Stop()
	defer close(o.doneCh)

	for {
		select {
		case <-ticker.C:
			// The flush is the actual stdout I/O, batching many buffered
			// records, so it gets a standalone gated span.
			_, span := output.StartSendSpan(context.Background(), o.tel, "blitz.output.stdout.flush")
			o.mu.Lock()
			_ = o.writer.Flush()
			o.mu.Unlock()
			span.End()
		case <-o.stopCh:
			return
		}
	}
}

// WriteMetric writes a metric record to stdout as JSON.
func (o *StdoutOutput) WriteMetric(_ context.Context, data output.MetricRecord) error {
	b, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshal metric: %w", err)
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	_, err = fmt.Fprintln(os.Stdout, string(b))
	return err
}

// WriteTrace writes a trace record to stdout as JSON.
func (o *StdoutOutput) WriteTrace(_ context.Context, data output.TraceRecord) error {
	b, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshal trace: %w", err)
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	_, err = fmt.Fprintln(os.Stdout, string(b))
	return err
}

// SupportedTelemetry returns the telemetry types this output can consume.
func (o *StdoutOutput) SupportedTelemetry() []telemetry.Type {
	return []telemetry.Type{telemetry.Logs, telemetry.Metrics, telemetry.Traces}
}
