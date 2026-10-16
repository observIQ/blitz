package paloalto

import (
	"cmp"
	"context"
	"crypto/rand"
	"fmt"
	"math"
	"math/big"
	mrand "math/rand"
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
	// dev is the one firewall every line names; read-only after New.
	dev device
}

// New creates a new Palo Alto generator. The consumer receives each
// generated record as a size-1 batch via ConsumeLogs.
func New(logger *zap.Logger, workers int, rate time.Duration, consumer embed.LogConsumer, tel embed.TelemetrySettings, opts ...generator.Option) (*Generator, error) {
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

	// The environment appliance names the device; without one the generator
	// still describes a single firewall for its lifetime. A hand-built
	// appliance missing a name or serial takes the fallback's.
	appliance := generator.NewOptions(opts...).Appliance
	dev := fallbackDevice()
	static := resource.NewStaticResources(resource.WithHost(dev.name, componentName))
	if appliance != nil {
		dev = newDevice(cmp.Or(appliance.Hostname, dev.name), cmp.Or(appliance.Serial, dev.serial))
		if appliance.FQDN != "" {
			dev.setFQDN(appliance.FQDN)
		}
		named := *appliance
		named.Hostname = dev.name // host.name matches the line's device name
		static = resource.FromAppliance(&named, componentName)
	}
	return &Generator{
		logger:   logger,
		workers:  workers,
		rate:     rate,
		consumer: consumer,
		static:   static,
		metrics:  metrics,
		stopCh:   make(chan struct{}),
		dev:      dev,
	}, nil
}

// Name returns the module identifier.
func (g *Generator) Name() string { return componentName }

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
	line, severity := generatePaloAltoLog(g.dev)

	g.metrics.BlitzGeneratorEntriesCounter.Add(context.Background(), 1, componentName)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	logRecord := embed.LogRecord{
		Message: line,
		Metadata: embed.LogRecordMetadata{
			Severity: severity,
			Resource: g.static.Record(),
			// The line carries the firewall's own BSD syslog header.
			SyslogFramed: true,
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

// addrRand is the one source behind the address draws, seeded from
// crypto/rand once. A *rand.Rand is not safe for concurrent use, so every
// draw holds mu.
var addrRand = struct {
	mu sync.Mutex
	r  *mrand.Rand
}{r: mrand.New(mrand.NewSource(randInt64(0, math.MaxInt64-1)))} // #nosec G404 -- synthetic addresses, not security

// drawAddr runs draw on the shared address source.
func drawAddr(draw func(*mrand.Rand) string) string {
	addrRand.mu.Lock()
	defer addrRand.mu.Unlock()
	return draw(addrRand.r)
}

// generateRandomIP is a private (RFC 1918) or public unicast address, never a
// reserved one (loopback, multicast, Class E, CGNAT, documentation).
func generateRandomIP() string {
	return drawAddr(func(r *mrand.Rand) string {
		if r.Intn(2) == 0 {
			return datagen.RandomPrivateIPv4(r)
		}
		return datagen.RandomPublicIPv4(r)
	})
}

// generatePublicIP is a routable public IPv4 address.
func generatePublicIP() string { return drawAddr(datagen.RandomPublicIPv4) }

// generatePrivateIP is an RFC 1918 address.
func generatePrivateIP() string { return drawAddr(datagen.RandomPrivateIPv4) }

func randInt(min, max int) int {
	delta := max - min + 1
	n, _ := rand.Int(rand.Reader, big.NewInt(int64(delta)))
	return min + int(n.Int64())
}

// randInt64 is the int64 sibling of randInt, for ranges past the int range on
// 32-bit platforms (notably linux/arm): 64-bit sequence numbers, serials, and
// IDs, and the seed of the address source.
func randInt64(min, max int64) int64 {
	delta := max - min + 1
	n, _ := rand.Int(rand.Reader, big.NewInt(delta))
	return min + n.Int64()
}

// SupportedTelemetry returns the telemetry types this generator produces.
func (g *Generator) SupportedTelemetry() []telemetry.Type {
	return []telemetry.Type{telemetry.Logs}
}
