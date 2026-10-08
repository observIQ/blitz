package paloalto

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/observiq/blitz/embed"
	"github.com/observiq/blitz/generator"
	"github.com/observiq/blitz/generator/count"
	"github.com/observiq/blitz/generator/resource"
	"github.com/observiq/blitz/internal/datagen"
	"github.com/observiq/blitz/telemetry"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"
)

const componentName = "paloalto"

// Generator produces Palo Alto-style syslog lines.
type Generator struct {
	embed.ProducerMarker

	logger   *zap.Logger
	workers  int
	rate     time.Duration
	consumer embed.LogConsumer
	static   *resource.StaticResources

	wg      sync.WaitGroup
	stopCh  chan struct{}
	tracker *count.Tracker
	metrics *generator.Metrics
}

// New creates a new Palo Alto generator. The consumer receives each
// generated record as a size-1 batch via ConsumeLogs.
func New(logger *zap.Logger, workers int, rate time.Duration, consumer embed.LogConsumer, tel embed.TelemetrySettings) (*Generator, error) {
	if logger == nil {
		return nil, fmt.Errorf("logger cannot be nil")
	}
	if workers < 1 {
		return nil, fmt.Errorf("workers must be 1 or greater, got %d", workers)
	}
	if consumer == nil {
		return nil, fmt.Errorf("consumer cannot be nil")
	}

	metrics, err := generator.NewMetrics(tel.MeterProvider)
	if err != nil {
		return nil, fmt.Errorf("build generator metrics: %w", err)
	}

	return &Generator{
		logger:   logger,
		workers:  workers,
		rate:     rate,
		consumer: consumer,
		static:   resource.FromIdentity(nil, componentName),
		metrics:  metrics,
		stopCh:   make(chan struct{}),
	}, nil
}

// Name returns the module identifier.
func (g *Generator) Name() string { return componentName }

// SetHostIdentity sets the simulated host whose identity every emitted record
// carries (PIPE-1036). A nil identity keeps the process-hostname fallback. Must
// be called before Start; the resource it builds is read concurrently by
// workers thereafter.
func (g *Generator) SetHostIdentity(id *datagen.SystemIdentity) {
	g.static = resource.FromIdentity(id, componentName)
}

// Start launches the worker goroutines that push generated records to
// the configured consumer.
func (g *Generator) Start(_ context.Context) error {
	g.logger.Info("Starting Palo Alto generator",
		zap.Int("workers", g.workers),
		zap.Duration("rate", g.rate),
	)

	g.metrics.BlitzGeneratorActiveWorkersGauge.Record(context.Background(), int64(g.workers), componentName)

	for i := 0; i < g.workers; i++ {
		g.wg.Add(1)
		go g.worker(i)
	}
	return nil
}

// Stop stops the generator.
func (g *Generator) Stop(ctx context.Context) error {
	g.logger.Info("Stopping Palo Alto generator")

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
func (g *Generator) SetCountTracker(t *count.Tracker) {
	g.tracker = t
}

func (g *Generator) worker(workerID int) {
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
			if err := g.generateAndWrite(workerID); err != nil {
				g.logger.Error("Failed to write log", zap.Int("worker_id", workerID), zap.Error(err))
				timer.Reset(backoffConfig.NextBackOff())
				continue
			}
			backoffConfig.Reset()
			timer.Reset(backoffConfig.NextBackOff())
		}
	}
}

func (g *Generator) generateAndWrite(_ int) error {
	line := generatePaloAltoLog()

	g.metrics.BlitzGeneratorEntriesCounter.Add(context.Background(), 1, componentName)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	logRecord := embed.LogRecord{
		Message: line,
		Metadata: embed.LogRecordMetadata{
			Severity: "INFO",
			Resource: g.static.Record(),
		},
	}

	if err := g.consumer.ConsumeLogs(ctx, []embed.LogRecord{logRecord}); err != nil {
		errorType := "unknown"
		if ctx.Err() == context.DeadlineExceeded {
			errorType = "timeout"
		}
		g.recordWriteError(errorType, err)
		return err
	}
	return nil
}

func (g *Generator) recordWriteError(errorType string, _ error) {
	g.metrics.BlitzGeneratorWriteErrorsCounter.Add(context.Background(), 1, componentName,
		metric.WithAttributeSet(attribute.NewSet(attribute.String("error_type", errorType))),
	)
}

// ---- Palo Alto log synthesis ----

func generateRandomIP() string {
	ranges := []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"34.0.0.0/8",
		"134.0.0.0/8",
		"206.0.0.0/8",
	}

	rangeIndex := randInt(0, len(ranges)-1)
	ipRange := ranges[rangeIndex]

	if strings.Contains(ipRange, "10.") {
		return fmt.Sprintf("10.%d.%d.%d", randInt(0, 255), randInt(0, 255), randInt(1, 254))
	} else if strings.Contains(ipRange, "172.") {
		return fmt.Sprintf("172.%d.%d.%d", randInt(16, 31), randInt(0, 255), randInt(1, 254))
	} else if strings.Contains(ipRange, "192.168.") {
		return fmt.Sprintf("192.168.%d.%d", randInt(0, 255), randInt(1, 254))
	}
	return fmt.Sprintf("%d.%d.%d.%d", randInt(1, 254), randInt(0, 255), randInt(0, 255), randInt(1, 254))
}

func generateRandomPort() string {
	commonPorts := datagen.CommonPorts.All()
	if randInt(0, 10) < 7 {
		return strconv.Itoa(commonPorts[randInt(0, len(commonPorts)-1)])
	}
	return strconv.Itoa(randInt(1024, 65535))
}

func generateRandomSessionID() string {
	bytes := make([]byte, 6)
	_, _ = rand.Read(bytes)
	return strings.ToUpper(hex.EncodeToString(bytes))
}

func generateNumericSessionID() string {
	// Generate a numeric session ID like "01606001116" or "1606001116"
	// Length varies between 9-11 digits
	length := randInt(9, 11)
	var sessionID strings.Builder
	for i := range length {
		// First digit can be 0 for IDs with length > 9, otherwise 1-9
		if i == 0 && length > 9 {
			digit := randInt(0, 9)
			sessionID.WriteString(strconv.Itoa(digit))
		} else {
			digit := randInt(0, 9)
			sessionID.WriteString(strconv.Itoa(digit))
		}
	}
	return sessionID.String()
}

func randInt(min, max int) int {
	delta := max - min + 1
	n, _ := rand.Int(rand.Reader, big.NewInt(int64(delta)))
	return min + int(n.Int64())
}

// randInt64 is the int64 sibling of randInt. Required at call sites
// where min or max exceeds the int range on 32-bit platforms (notably
// linux/arm) — see the 10-digit session-id field in the THREAT log.
func randInt64(min, max int64) int64 {
	delta := max - min + 1
	n, _ := rand.Int(rand.Reader, big.NewInt(delta))
	return min + n.Int64()
}

// SupportedTelemetry returns the telemetry types this generator produces.
func (g *Generator) SupportedTelemetry() []telemetry.Type {
	return []telemetry.Type{telemetry.Logs}
}
