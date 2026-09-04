package wel

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/observiq/blitz/embed"
	"github.com/observiq/blitz/generator"
	"github.com/observiq/blitz/generator/resource"
	"github.com/observiq/blitz/generator/wel/catalog"
	"github.com/observiq/blitz/telemetry"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"
)

const componentName = "wel"

// Compile-time assertion: WEL XML mode is a Producer module.
var _ embed.ProducerModule = (*Generator)(nil)

// Generator generates Windows Event Log entries (XML mode) and yields
// them as embed.LogRecord batches through a host-supplied consumer.
type Generator struct {
	embed.ProducerMarker

	logger   *zap.Logger
	workers  int
	rate     time.Duration
	computer string
	domain   string
	role     catalog.MachineRole
	channels []string
	consumer embed.LogConsumer

	registry *catalog.Registry
	state    *catalog.StateTracker
	opts     *catalog.GenerateOpts

	wg     sync.WaitGroup
	stopCh chan struct{}

	recordID atomic.Int64
}

// Config holds the WEL generator configuration. Consumer is mandatory:
// it is the destination for every record this generator produces, the
// same way the consumer wiring works for every other Producer module.
type Config struct {
	Logger   *zap.Logger
	Workers  int
	Rate     time.Duration
	Computer string
	Domain   string
	Role     catalog.MachineRole
	Channels []string
	Consumer embed.LogConsumer

	// Environment data — caller-owned slices. The generator stores the
	// reference; callers must not mutate the underlying arrays after
	// passing them in.
	Usernames []string
	IPs       []string
	Hostnames []string
}

// New creates a new WEL generator.
func New(cfg Config) (*Generator, error) {
	if cfg.Logger == nil {
		return nil, fmt.Errorf("logger cannot be nil")
	}
	if cfg.Consumer == nil {
		return nil, fmt.Errorf("consumer cannot be nil")
	}
	if cfg.Workers < 1 {
		return nil, fmt.Errorf("workers must be 1 or greater, got %d", cfg.Workers)
	}
	if cfg.Role == "" {
		cfg.Role = catalog.RoleMember
	}
	if cfg.Domain == "" {
		cfg.Domain = "WORKGROUP"
	}
	if cfg.Computer == "" {
		cfg.Computer = "BLITZ-WEL"
	}

	// Build registry filtered by role and channels
	reg := catalog.DefaultRegistry(cfg.Role)
	if len(cfg.Channels) > 0 {
		reg = reg.FilterChannels(cfg.Channels)
	}

	channels := reg.Channels()
	if len(channels) == 0 {
		return nil, fmt.Errorf("no events available for role %q with channels %v", cfg.Role, cfg.Channels)
	}

	state := catalog.NewStateTracker(1000)

	// Defensive clone of the caller-supplied slices. Workers read from
	// these concurrently; a caller that retains its own reference and
	// later mutates the slice would race with the workers otherwise.
	opts := &catalog.GenerateOpts{
		Computer:   cfg.Computer,
		DomainName: cfg.Domain,
		Role:       cfg.Role,
		Usernames:  cloneStrings(cfg.Usernames),
		IPs:        cloneStrings(cfg.IPs),
		Hostnames:  cloneStrings(cfg.Hostnames),
		State:      state,
	}

	return &Generator{
		logger:   cfg.Logger,
		workers:  cfg.Workers,
		rate:     cfg.Rate,
		computer: cfg.Computer,
		domain:   cfg.Domain,
		role:     cfg.Role,
		channels: channels,
		consumer: cfg.Consumer,
		registry: reg,
		state:    state,
		opts:     opts,
		stopCh:   make(chan struct{}),
	}, nil
}

// Name returns the module identifier.
func (g *Generator) Name() string { return componentName }

// Start launches the worker goroutines that yield generated records
// to the configured consumer. Start returns once workers are running.
func (g *Generator) Start(_ context.Context) error {
	g.logger.Info("Starting WEL generator",
		zap.Int("workers", g.workers),
		zap.Duration("rate", g.rate),
		zap.String("role", string(g.role)),
		zap.Strings("channels", g.channels),
	)

	generator.BlitzGeneratorActiveWorkersGauge.Record(context.Background(), int64(g.workers), componentName)

	for i := 0; i < g.workers; i++ {
		g.wg.Add(1)
		go g.worker(i)
	}
	return nil
}

// Stop stops the generator.
func (g *Generator) Stop(ctx context.Context) error {
	g.logger.Info("Stopping WEL generator")

	generator.BlitzGeneratorActiveWorkersGauge.Record(ctx, 0, componentName)

	close(g.stopCh)

	done := make(chan struct{})
	go func() {
		g.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		g.logger.Info("All WEL workers stopped gracefully")
		return nil
	case <-ctx.Done():
		return fmt.Errorf("stop cancelled due to context cancellation: %w", ctx.Err())
	}
}

// SupportedTelemetry returns the telemetry types this generator produces.
func (g *Generator) SupportedTelemetry() []telemetry.Type {
	return []telemetry.Type{telemetry.Logs}
}

func (g *Generator) worker(workerID int) {
	defer g.wg.Done()
	g.logger.Debug("Starting WEL worker", zap.Int("worker_id", workerID))

	rng := rand.New(rand.NewSource(time.Now().UnixNano() + int64(workerID))) // #nosec G404

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
			g.logger.Debug("WEL worker stopping", zap.Int("worker_id", workerID))
			return
		case <-timer.C:
			if err := g.generateAndWrite(rng); err != nil {
				g.logger.Error("Failed to write WEL event",
					zap.Int("worker_id", workerID),
					zap.Error(err),
				)
				timer.Reset(backoffConfig.NextBackOff())
				continue
			}
			backoffConfig.Reset()
			timer.Reset(backoffConfig.NextBackOff())
		}
	}
}

func (g *Generator) generateAndWrite(rng *rand.Rand) error {
	// Pick a random event definition
	def := g.registry.RandomEvent(rng)
	if def == nil {
		return fmt.Errorf("no event definitions available")
	}

	// Generate the event record
	recordID := g.recordID.Add(1)
	record := GenerateRecord(rng, def, g.opts, recordID)

	// Render as XML for the consumer
	xml := record.ToXML()

	generator.BlitzGeneratorEntriesCounter.Add(context.Background(), 1, componentName,
		metric.WithAttributeSet(attribute.NewSet(attribute.String("channel", record.Channel))),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	logRecord := embed.LogRecord{
		Message: xml,
		Metadata: embed.LogRecordMetadata{
			Severity: record.LevelName,
			Resource: resource.Default(componentName,
				"wel.channel", record.Channel,
				"wel.computer", g.computer,
				"wel.domain", g.domain,
				"wel.role", string(g.role),
			),
		},
	}

	if err := g.consumer.ConsumeLogs(ctx, []embed.LogRecord{logRecord}); err != nil {
		errorType := "unknown"
		if ctx.Err() == context.DeadlineExceeded {
			errorType = "timeout"
		}
		generator.BlitzGeneratorWriteErrorsCounter.Add(context.Background(), 1, componentName,
			metric.WithAttributeSet(attribute.NewSet(attribute.String("error_type", errorType))),
		)
		return err
	}

	return nil
}

// cloneStrings returns a defensive copy of s. Constructor uses it so a
// caller that retains a reference to a passed-in slice and later mutates
// it cannot race with worker goroutines reading from opts.
func cloneStrings(s []string) []string {
	if s == nil {
		return nil
	}
	out := make([]string, len(s))
	copy(out, s)
	return out
}
