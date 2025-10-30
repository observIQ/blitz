package generator

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/observiq/blitz/internal/winevt/templates"
	"github.com/observiq/blitz/output"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"
)

// WinevtGenerator generates Windows Event XML logs using templates.
type WinevtGenerator struct {
	logger  *zap.Logger
	workers int
	rate    time.Duration

	wg     sync.WaitGroup
	stopCh chan struct{}
	meter  metric.Meter

	// Metrics
	winevtLogsGenerated metric.Int64Counter
	winevtActiveWorkers metric.Int64Gauge
	winevtWriteErrors   metric.Int64Counter
}

// NewWinevtGenerator creates a new Windows Event generator.
func NewWinevtGenerator(logger *zap.Logger, workers int, rate time.Duration) (*WinevtGenerator, error) {
	if logger == nil {
		return nil, fmt.Errorf("logger cannot be nil")
	}
	if workers < 1 {
		return nil, fmt.Errorf("workers must be 1 or greater, got %d", workers)
	}

	meter := otel.Meter("blitz-generator")

	winevtLogsGenerated, err := meter.Int64Counter(
		"blitz.generator.logs.generated",
		metric.WithDescription("Total number of logs generated"),
	)
	if err != nil {
		return nil, fmt.Errorf("create logs generated counter: %w", err)
	}

	winevtActiveWorkers, err := meter.Int64Gauge(
		"blitz.generator.workers.active",
		metric.WithDescription("Number of active worker goroutines"),
	)
	if err != nil {
		return nil, fmt.Errorf("create active workers gauge: %w", err)
	}

	winevtWriteErrors, err := meter.Int64Counter(
		"blitz.generator.write.errors",
		metric.WithDescription("Total number of write errors"),
	)
	if err != nil {
		return nil, fmt.Errorf("create write errors counter: %w", err)
	}

	return &WinevtGenerator{
		logger:              logger,
		workers:             workers,
		rate:                rate,
		stopCh:              make(chan struct{}),
		meter:               meter,
		winevtLogsGenerated: winevtLogsGenerated,
		winevtActiveWorkers: winevtActiveWorkers,
		winevtWriteErrors:   winevtWriteErrors,
	}, nil
}

// Start starts the Windows Event generator.
func (g *WinevtGenerator) Start(writer output.Writer) error {
	g.logger.Info("Starting Windows Event generator",
		zap.Int("workers", g.workers),
		zap.Duration("rate", g.rate),
	)

	g.winevtActiveWorkers.Record(context.Background(), int64(g.workers),
		metric.WithAttributeSet(attribute.NewSet(attribute.String("component", "generator_winevt"))),
	)

	for i := 0; i < g.workers; i++ {
		g.wg.Add(1)
		go g.worker(i, writer)
	}
	return nil
}

// Stop stops the generator.
func (g *WinevtGenerator) Stop(ctx context.Context) error {
	g.logger.Info("Stopping Windows Event generator")

	g.winevtActiveWorkers.Record(ctx, 0,
		metric.WithAttributeSet(attribute.NewSet(attribute.String("component", "generator_winevt"))),
	)

	close(g.stopCh)

	done := make(chan struct{})
	go func() {
		g.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		g.logger.Info("All workers stopped gracefully")
		return nil
	case <-ctx.Done():
		return fmt.Errorf("stop cancelled due to context cancellation: %w", ctx.Err())
	}
}

func (g *WinevtGenerator) worker(workerID int, writer output.Writer) {
	defer g.wg.Done()
	g.logger.Debug("Starting worker", zap.Int("worker_id", workerID))

	backoffConfig := backoff.NewExponentialBackOff()
	backoffConfig.InitialInterval = g.rate
	backoffConfig.MaxInterval = 5 * time.Second
	backoffConfig.MaxElapsedTime = 0

	backoffTicker := backoff.NewTicker(backoffConfig)
	defer backoffTicker.Stop()

	for {
		select {
		case <-g.stopCh:
			g.logger.Debug("Worker stopping", zap.Int("worker_id", workerID))
			return
		case <-backoffTicker.C:
			if err := g.generateAndWrite(writer, workerID); err != nil {
				g.logger.Error("Failed to write log", zap.Int("worker_id", workerID), zap.Error(err))
				continue
			}
			backoffConfig.Reset()
		}
	}
}

func (g *WinevtGenerator) generateAndWrite(writer output.Writer, workerID int) error {
	data, err := templates.RenderTemplate(templates.RenderOptions{})
	if err != nil {
		g.recordWriteError("unknown", err)
		return fmt.Errorf("render template: %w", err)
	}

	g.winevtLogsGenerated.Add(context.Background(), 1,
		metric.WithAttributeSet(attribute.NewSet(attribute.String("component", "generator_winevt"))),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	logRecord := output.LogRecord{
		Message: data,
		Metadata: output.LogRecordMetadata{
			Severity: "WARN",
		},
	}

	if err := writer.Write(ctx, logRecord); err != nil {
		errorType := "unknown"
		if ctx.Err() == context.DeadlineExceeded {
			errorType = "timeout"
		}
		g.recordWriteError(errorType, err)
		return err
	}
	return nil
}

func (g *WinevtGenerator) recordWriteError(errorType string, err error) {
	ctx := context.Background()
	g.winevtWriteErrors.Add(ctx, 1,
		metric.WithAttributeSet(attribute.NewSet(
			attribute.String("component", "generator_winevt"),
			attribute.String("error_type", errorType),
		)),
	)
}
