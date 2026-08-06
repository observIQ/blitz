package winevt

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/observiq/blitz/embed"
	"github.com/observiq/blitz/generator"
	"github.com/observiq/blitz/generator/count"
	"github.com/observiq/blitz/internal/generators/winevt/templates"
	"github.com/observiq/blitz/output"
	"github.com/observiq/blitz/telemetry"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"
)

const componentName = "winevt"

// WinevtGenerator generates Windows Event XML logs using templates.
type WinevtGenerator struct {
	embed.ProducerMarker

	logger  *zap.Logger
	workers int
	rate    time.Duration

	wg      sync.WaitGroup
	stopCh  chan struct{}
	tracker *count.Tracker
	metrics *generator.Metrics
}

// New creates a new Windows Event generator.
func New(logger *zap.Logger, workers int, rate time.Duration, tel embed.TelemetrySettings) (*WinevtGenerator, error) {
	if logger == nil {
		return nil, fmt.Errorf("logger cannot be nil")
	}
	if workers < 1 {
		return nil, fmt.Errorf("workers must be 1 or greater, got %d", workers)
	}

	metrics, err := generator.NewMetrics(tel.MeterProvider)
	if err != nil {
		return nil, fmt.Errorf("build generator metrics: %w", err)
	}

	return &WinevtGenerator{
		logger:  logger,
		workers: workers,
		rate:    rate,
		metrics: metrics,
		stopCh:  make(chan struct{}),
	}, nil
}

// Start starts the Windows Event generator.
func (g *WinevtGenerator) Start(writer output.Writer) error {
	g.logger.Info("Starting Windows Event generator",
		zap.Int("workers", g.workers),
		zap.Duration("rate", g.rate),
	)

	g.metrics.BlitzGeneratorActiveWorkersGauge.Record(context.Background(), int64(g.workers), componentName)

	for i := 0; i < g.workers; i++ {
		g.wg.Add(1)
		go g.worker(i, writer)
	}
	return nil
}

// Stop stops the generator.
func (g *WinevtGenerator) Stop(ctx context.Context) error {
	g.logger.Info("Stopping Windows Event generator")

	g.metrics.BlitzGeneratorActiveWorkersGauge.Record(ctx, 0, componentName)

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

// SetCountTracker sets the finite generation count tracker.
func (g *WinevtGenerator) SetCountTracker(t *count.Tracker) {
	g.tracker = t
}

func (g *WinevtGenerator) worker(workerID int, writer output.Writer) {
	defer g.wg.Done()
	g.logger.Debug("Starting worker", zap.Int("worker_id", workerID))

	backoffConfig := backoff.NewExponentialBackOff()
	backoffConfig.InitialInterval = g.rate
	backoffConfig.MaxInterval = 5 * time.Second
	backoffConfig.MaxElapsedTime = 0

	// Drive the timer from this goroutine only. backoff.ExponentialBackOff is
	// not safe for concurrent use, so we never hand it to backoff.NewTicker's
	// internal goroutine; instead we own every NextBackOff/Reset call here.
	timer := time.NewTimer(backoffConfig.NextBackOff())
	defer timer.Stop()

	for {
		select {
		case <-g.stopCh:
			g.logger.Debug("Worker stopping", zap.Int("worker_id", workerID))
			return
		case <-timer.C:
			if g.tracker != nil && !g.tracker.Acquire() {
				select {
				case <-g.stopCh:
					return
				case <-g.tracker.ResumeC():
					timer.Reset(backoffConfig.NextBackOff())
					continue
				}
			}
			if err := g.generateAndWrite(writer, workerID); err != nil {
				g.logger.Error("Failed to write log", zap.Int("worker_id", workerID), zap.Error(err))
				timer.Reset(backoffConfig.NextBackOff())
				continue
			}
			backoffConfig.Reset()
			timer.Reset(backoffConfig.NextBackOff())
		}
	}
}

func (g *WinevtGenerator) generateAndWrite(writer output.Writer, workerID int) error {
	data, err := templates.RenderTemplate(templates.RenderOptions{})
	if err != nil {
		g.recordWriteError("unknown", err)
		return fmt.Errorf("render template: %w", err)
	}

	g.metrics.BlitzGeneratorEntriesCounter.Add(context.Background(), 1, componentName)

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

func (g *WinevtGenerator) recordWriteError(errorType string, _ error) {
	g.metrics.BlitzGeneratorWriteErrorsCounter.Add(context.Background(), 1, componentName,
		metric.WithAttributeSet(attribute.NewSet(attribute.String("error_type", errorType))),
	)
}

// SupportedTelemetry returns the telemetry types this generator produces.
func (g *WinevtGenerator) SupportedTelemetry() []telemetry.Type {
	return []telemetry.Type{telemetry.Logs}
}
