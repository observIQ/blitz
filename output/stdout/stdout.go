package stdout

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/observiq/blitz/embed"
	"github.com/observiq/blitz/generator/count"
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
	logger  *zap.Logger
	tel     embed.TelemetrySettings
	metrics *output.Metrics
	writer  *bufio.Writer
	mu      sync.Mutex
	// pending settles the count permits of buffered lines, guarded by mu.
	pending []func(error)
	// stopped rejects writes once Stop has begun its final flush, guarded by mu.
	stopped       bool
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

// errStopped rejects a write after Stop's final flush.
var errStopped = errors.New("stdout output is stopped")

// WriteLog buffers the log record for the next flush.
func (o *StdoutOutput) WriteLog(ctx context.Context, data output.LogRecord) error {
	o.metrics.BlitzOutputEntriesReceivedCounter.Add(ctx, 1, outputType, "logs")
	return o.writeLine(ctx, data.Message)
}

// writeLine buffers one line for the next flush. Every signal shares the
// buffer, and mu is held for the whole line, so no record interleaves with
// it, even when a write flushes the buffer part-way.
func (o *StdoutOutput) writeLine(ctx context.Context, line string) error {
	sent := count.Hold(ctx)
	err := o.bufferLine(ctx, line, sent)
	if err != nil {
		// Settle outside mu, as flush does.
		sent(err)
	}
	return err
}

// bufferLine writes line to the buffer and, for a counted record, queues
// sent to settle on the next flush.
func (o *StdoutOutput) bufferLine(ctx context.Context, line string, sent func(error)) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.stopped {
		return errStopped
	}
	if _, err := o.writer.WriteString(line); err != nil {
		return err
	}
	if err := o.writer.WriteByte('\n'); err != nil {
		return err
	}
	if count.Counted(ctx) {
		o.pending = append(o.pending, sent)
	}
	return nil
}

// flush writes the buffer to stdout and settles the buffered lines' permits
// with the result.
func (o *StdoutOutput) flush() {
	o.mu.Lock()
	err := o.writer.Flush()
	pending := o.pending
	o.pending = nil
	o.mu.Unlock()
	// Settle outside mu, so writers do not wait on each permit's locks.
	count.Sends(pending).Settle(err)
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
	o.stopped = true
	o.mu.Unlock()
	o.flush()
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
			o.flush()
			span.End()
		case <-o.stopCh:
			return
		}
	}
}

// WriteMetric buffers a metric record as JSON for the next flush.
func (o *StdoutOutput) WriteMetric(ctx context.Context, data output.MetricRecord) error {
	b, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshal metric: %w", err)
	}
	return o.writeLine(ctx, string(b))
}

// WriteTrace buffers a trace record as JSON for the next flush.
func (o *StdoutOutput) WriteTrace(ctx context.Context, data output.TraceRecord) error {
	b, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshal trace: %w", err)
	}
	return o.writeLine(ctx, string(b))
}

// SupportedTelemetry returns the telemetry types this output can consume.
func (o *StdoutOutput) SupportedTelemetry() []telemetry.Type {
	return []telemetry.Type{telemetry.Logs, telemetry.Metrics, telemetry.Traces}
}
