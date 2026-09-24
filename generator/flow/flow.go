// Package flow generates network FlowRecords (the 5-tuple + counters) shaped
// by a canned traffic scenario. It is protocol-agnostic: the wire format
// (NetFlow v5/v9, IPFIX, sFlow) is chosen by the flow output that consumes
// these records, so one generator drives every exporter format.
package flow

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/observiq/blitz/embed"
	"github.com/observiq/blitz/generator"
	"github.com/observiq/blitz/generator/count"
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

	wg      sync.WaitGroup
	stopCh  chan struct{}
	tracker *count.Tracker
}

// New builds a flow generator. seed < 0 randomizes; 0+ is a deterministic seed.
func New(logger *zap.Logger, workers int, rate time.Duration, scenario Scenario, seed int64, consumer embed.FlowConsumer, tel embed.TelemetrySettings) (*Generator, error) {
	if logger == nil {
		return nil, fmt.Errorf("logger cannot be nil")
	}
	if workers < 1 {
		return nil, fmt.Errorf("workers must be 1 or greater, got %d", workers)
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
	return &Generator{
		logger:   logger,
		workers:  workers,
		rate:     rate,
		scenario: scenario,
		seed:     seed,
		consumer: consumer,
		metrics:  metrics,
		stopCh:   make(chan struct{}),
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
	g.logger.Info("Starting flow generator",
		zap.Int("workers", g.workers), zap.Duration("rate", g.rate), zap.String("scenario", string(g.scenario)))
	g.metrics.BlitzGeneratorActiveWorkersGauge.Record(context.Background(), int64(g.workers), componentName)
	for i := 0; i < g.workers; i++ {
		g.wg.Add(1)
		go g.worker(i)
	}
	return nil
}

// Stop stops the generator and waits for workers to drain.
func (g *Generator) Stop(ctx context.Context) error {
	g.logger.Info("Stopping flow generator")
	g.metrics.BlitzGeneratorActiveWorkersGauge.Record(ctx, 0, componentName)
	close(g.stopCh)
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
	// Per-worker RNG: deterministic seed offsets by worker so a fixed seed
	// reproduces the whole run while workers stay independent. seed < 0
	// randomizes from wall clock.
	seed := g.seed + int64(workerID)
	if g.seed < 0 {
		seed = time.Now().UnixNano() + int64(workerID)
	}
	r := rand.New(rand.NewSource(seed)) // #nosec G404 -- synthetic load data, not security-sensitive

	ticker := time.NewTicker(g.rate)
	defer ticker.Stop()
	for {
		select {
		case <-g.stopCh:
			return
		case <-ticker.C:
			if g.tracker != nil && !g.tracker.Acquire() {
				select {
				case <-g.stopCh:
					return
				case <-g.tracker.ResumeC():
					continue
				}
			}
			if err := g.generateAndWrite(r); err != nil {
				g.logger.Error("Failed to write flow", zap.Int("worker_id", workerID), zap.Error(err))
			}
		}
	}
}

func (g *Generator) generateAndWrite(r *rand.Rand) error {
	rec := generateFlow(r, g.scenario, time.Now())
	g.metrics.BlitzGeneratorEntriesCounter.Add(context.Background(), 1, componentName)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := g.consumer.ConsumeFlows(ctx, []embed.FlowRecord{rec}); err != nil {
		g.metrics.BlitzGeneratorWriteErrorsCounter.Add(context.Background(), 1, componentName)
		return err
	}
	return nil
}
