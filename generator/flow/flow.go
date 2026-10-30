// Package flow generates network FlowRecords (the 5-tuple + counters) shaped
// by a canned traffic scenario. It is protocol-agnostic: the wire format
// (NetFlow v5/v9, IPFIX, sFlow) is chosen by the flow output that consumes
// these records, so one generator drives every exporter format.
package flow

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"sync"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/jonboulle/clockwork"

	"github.com/observiq/blitz/embed"
	"github.com/observiq/blitz/generator"
	"github.com/observiq/blitz/generator/count"
	"github.com/observiq/blitz/generator/resource"
	"github.com/observiq/blitz/internal/datagen"
	"github.com/observiq/blitz/internal/generator/pacing"
	"github.com/observiq/blitz/telemetry"
	"go.uber.org/zap"
)

const componentName = "flow"

// Generator produces FlowRecords and pushes them to a FlowConsumer.
type Generator struct {
	embed.ProducerMarker

	logger   *zap.Logger
	workers  int
	rate     time.Duration
	scenario Scenario
	seed     int64
	consumer embed.FlowConsumer
	metrics  *generator.Metrics
	static   *resource.StaticResources
	// exporter is the address of the one device the generator's flows come
	// from: the appliance's management address, or drawn at Start from the
	// resolved seed. Workers only read it.
	exporter net.IP
	// victim is the ddos-target scenario's destination, drawn at Start.
	victim host

	wg       sync.WaitGroup
	stopCh   chan struct{}
	stopOnce sync.Once
	tracker  *count.Tracker
	clock    clockwork.Clock // paces workers and stamps records; tests fake it
}

// New builds a flow generator. seed < 0 picks a random seed at Start (logged
// so the run can be reproduced); 0+ is a deterministic seed.
//
// generator.WithAppliance sets the routing device the flows come from: its
// hostname and model are the resource, and its management address is every
// record's ExporterIP. Without one the resource is the running host and the
// exporter address is drawn from the seed.
func New(logger *zap.Logger, workers int, rate time.Duration, scenario Scenario, seed int64, consumer embed.FlowConsumer, tel embed.TelemetrySettings, opts ...generator.Option) (*Generator, error) {
	if logger == nil {
		return nil, fmt.Errorf("logger cannot be nil")
	}
	if workers < 1 {
		return nil, fmt.Errorf("workers must be 1 or greater, got %d", workers)
	}
	if rate <= 0 {
		return nil, fmt.Errorf("rate must be positive, got %v", rate)
	}
	if consumer == nil {
		return nil, fmt.Errorf("consumer cannot be nil")
	}
	if !ValidScenario(scenario) {
		return nil, fmt.Errorf("unknown flow scenario %q", scenario)
	}
	metrics, err := generator.NewMetrics(tel.MeterProvider)
	if err != nil {
		return nil, fmt.Errorf("build generator metrics: %w", err)
	}
	if scenario == "" {
		scenario = ScenarioDefault
	}
	dev := generator.NewOptions(opts...).Appliance
	var exporter net.IP
	if dev != nil && dev.ManagementInterface != nil {
		exporter = net.ParseIP(dev.ManagementInterface.IPv4)
	}
	return &Generator{
		logger:   logger,
		workers:  workers,
		rate:     rate,
		scenario: scenario,
		seed:     seed,
		consumer: consumer,
		metrics:  metrics,
		static:   resource.FromAppliance(dev, componentName),
		exporter: exporter,
		stopCh:   make(chan struct{}),
		clock:    clockwork.NewRealClock(),
	}, nil
}

// Name returns the module identifier.
func (g *Generator) Name() string { return componentName }

// SupportedTelemetry reports that this generator produces flow records.
func (g *Generator) SupportedTelemetry() []telemetry.Type {
	return []telemetry.Type{telemetry.Flows}
}

// SetCountTracker sets the finite generation count tracker.
func (g *Generator) SetCountTracker(t *count.Tracker) { g.tracker = t }

// Start launches the worker goroutines.
func (g *Generator) Start(_ context.Context) error {
	// Resolve a random seed once so the logged value reproduces the run:
	// worker N uses seed+N.
	if g.seed < 0 {
		g.seed = time.Now().UnixNano()
	}
	// seed-1: workers use seed+N (N >= 0), so the exporter has its own stream.
	if g.exporter == nil {
		g.exporter = net.ParseIP(datagen.RandomPrivateIPv4(rand.New(rand.NewSource(g.seed - 1)))) // #nosec G404 -- synthetic identity, not security
	}
	// seed-2: the ddos-target victim, a public host with its own stream too.
	g.victim = endpoint(rand.New(rand.NewSource(g.seed-2)), false) // #nosec G404 -- synthetic identity, not security
	g.logger.Info("Starting flow generator",
		zap.Int("workers", g.workers), zap.Duration("rate", g.rate), zap.String("scenario", string(g.scenario)),
		zap.Int64("seed", g.seed), zap.Stringer("exporter", g.exporter))
	g.metrics.BlitzGeneratorActiveWorkersGauge.Record(context.Background(), int64(g.workers), componentName)
	for i := 0; i < g.workers; i++ {
		g.wg.Add(1)
		go g.worker(i) // #nosec G118 -- workers are bounded by Stop() and the WaitGroup, not the Start context
	}
	return nil
}

// Stop stops the generator and waits for workers to drain.
func (g *Generator) Stop(ctx context.Context) error {
	g.logger.Info("Stopping flow generator")
	g.metrics.BlitzGeneratorActiveWorkersGauge.Record(ctx, 0, componentName)
	g.stopOnce.Do(func() { close(g.stopCh) })
	done := make(chan struct{})
	go func() { g.wg.Wait(); close(done) }()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("stop cancelled due to context cancellation: %w", ctx.Err())
	}
}

func (g *Generator) worker(workerID int) {
	defer g.wg.Done()
	// Per-worker RNG: the seed (resolved at Start) offset by worker, so one
	// logged seed reproduces the whole run while workers stay independent.
	seed := g.seed + int64(workerID)
	r := rand.New(rand.NewSource(seed)) // #nosec G404 -- synthetic load data, not security-sensitive

	// Failed writes back off exponentially, as in the log generators. The
	// timer is driven from this goroutine only: ExponentialBackOff is not safe
	// for concurrent use.
	bo := newWriteBackoff(g.rate)
	timer := g.clock.NewTimer(bo.NextBackOff())
	defer timer.Stop()
	for {
		select {
		case <-g.stopCh:
			return
		case <-timer.Chan():
			permit, wait, ok := g.tracker.Acquire()
			if !ok {
				g.logger.Debug("Flow worker at the count limit, waiting", zap.Int("worker_id", workerID))
				if !pacing.AwaitWake(g.stopCh, wait, bo, timer) {
					return
				}
				continue
			}
			if err := g.generateAndWrite(count.WithPermit(context.Background(), permit), r); err != nil {
				g.logger.Error("Failed to write flow", zap.Int("worker_id", workerID), zap.Error(err))
				permit.Release()
				timer.Reset(bo.NextBackOff())
				continue
			}
			permit.Commit()
			bo.Reset()
			timer.Reset(bo.NextBackOff())
		}
	}
}

func (g *Generator) generateAndWrite(ctx context.Context, r *rand.Rand) error {
	rec := generateFlow(r, g.scenario, g.clock.Now(), g.victim)
	rec.ExporterIP = g.exporter
	rec.Metadata.Resource = g.static.Record()
	g.metrics.BlitzGeneratorEntriesCounter.Add(context.Background(), 1, componentName)
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := g.consumer.ConsumeFlows(ctx, []embed.FlowRecord{rec}); err != nil {
		g.metrics.BlitzGeneratorWriteErrorsCounter.Add(context.Background(), 1, componentName)
		return err
	}
	return nil
}

// newWriteBackoff paces a worker: rate between sends, growing exponentially
// to 5 s (or rate, when longer) while writes fail.
func newWriteBackoff(rate time.Duration) *backoff.ExponentialBackOff {
	bo := backoff.NewExponentialBackOff()
	bo.InitialInterval = rate
	bo.MaxInterval = max(5*time.Second, rate)
	bo.MaxElapsedTime = 0      // never stop retrying
	bo.RandomizationFactor = 0 // healthy sends keep exactly rate apart
	bo.Reset()                 // start from rate, not the library's 500 ms default
	return bo
}
