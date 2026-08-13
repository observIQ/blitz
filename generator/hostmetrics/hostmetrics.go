package hostmetrics

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"

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

const generatorType = "hostmetrics"

// Config configures the host-metrics generator.
type Config struct {
	// Logger is the zap logger used for diagnostic output. Required.
	Logger *zap.Logger
	// Workers is the number of worker goroutines. Required, >= 1.
	Workers int
	// Rate is the scrape interval per worker. Required, > 0.
	Rate time.Duration
	// OS is the simulated operating system ("linux" or "windows").
	OS string
	// Hostname is the simulated hostname. If empty, a random hostname is
	// generated per the OS style.
	Hostname string
	// ScraperNames restricts emission to a named subset. Empty = all.
	ScraperNames []string
	// Consumer receives every scraped batch. Required.
	Consumer embed.MetricConsumer
	// Seed controls per-worker RNG seeding for scrape values.
	// Negative → randomize (worker N gets time.Now().UnixNano()+N).
	// 0 or positive → deterministic (worker N gets seed Seed+N).
	//
	// Programmatic Go callers see the literal value here. The YAML
	// path additionally translates `seed: 0` (or omitted) into
	// randomize at the dispatch layer so YAML users get stochastic
	// data by default; pass a positive value explicitly for
	// reproducibility.
	Seed int64
}

// Generator implements embed.ProducerModule for host metrics.
type Generator struct {
	embed.ProducerMarker

	logger   *zap.Logger
	workers  int
	rate     time.Duration
	os       string
	hostname string
	scrapers []Scraper
	consumer embed.MetricConsumer
	seed     int64
	wg       sync.WaitGroup
	stopCh   chan struct{}
	tracker  *count.Tracker
}

// Compile-time assertion that *Generator implements embed.ProducerModule.
var _ embed.ProducerModule = (*Generator)(nil)

// New creates a new host-metrics generator. The supplied
// embed.MetricConsumer receives every scraped batch — callers wrap
// existing output.MetricWriter values with output.WriterAsMetricConsumer
// for standalone CLI use.
func New(cfg Config) (*Generator, error) {
	if cfg.Logger == nil {
		return nil, fmt.Errorf("logger cannot be nil")
	}
	if cfg.Consumer == nil {
		return nil, fmt.Errorf("MetricConsumer cannot be nil")
	}
	if cfg.Workers < 1 {
		return nil, fmt.Errorf("workers must be 1 or greater, got %d", cfg.Workers)
	}
	if cfg.Rate <= 0 {
		return nil, fmt.Errorf("rate must be greater than 0, got %s", cfg.Rate)
	}

	hostname := cfg.Hostname
	if hostname == "" {
		// Hostname-only RNG; intentionally seeded once at construction
		// since hostname is fixed for the lifetime of the generator.
		style := datagen.StyleLinux
		if cfg.OS == "windows" {
			style = datagen.StyleWindows
		}
		seed := cfg.Seed
		if seed < 0 {
			seed = time.Now().UnixNano()
		}
		hostname = datagen.GenerateHostname(
			rand.New(rand.NewSource(seed)), // #nosec G404 -- seeded for determinism contract
			style,
			datagen.AllMythologyNames,
		)
	}

	return &Generator{
		logger:   cfg.Logger.Named("generator-hostmetrics"),
		workers:  cfg.Workers,
		rate:     cfg.Rate,
		os:       cfg.OS,
		hostname: hostname,
		scrapers: buildScrapers(cfg.ScraperNames),
		consumer: cfg.Consumer,
		seed:     cfg.Seed,
		stopCh:   make(chan struct{}),
	}, nil
}

// Name returns the module identifier for ProducerModule.
func (g *Generator) Name() string { return generatorType }

// SetCountTracker sets the count tracker for finite generation.
func (g *Generator) SetCountTracker(tracker *count.Tracker) {
	g.tracker = tracker
}

// Start launches the worker goroutines.
func (g *Generator) Start(_ context.Context) error {
	g.logger.Info("Starting host metrics generator",
		zap.Int("workers", g.workers),
		zap.Duration("rate", g.rate),
		zap.String("os", g.os),
		zap.String("hostname", g.hostname),
		zap.Int("scrapers", len(g.scrapers)),
	)

	generator.BlitzGeneratorActiveWorkersGauge.Record(context.Background(), int64(g.workers), generatorType)

	for i := range g.workers {
		g.wg.Add(1)
		go g.worker(i)
	}

	return nil
}

// Stop signals workers to drain and waits for them to exit.
func (g *Generator) Stop(ctx context.Context) error {
	g.logger.Info("Stopping host metrics generator")

	close(g.stopCh)

	generator.BlitzGeneratorActiveWorkersGauge.Record(ctx, 0, generatorType)

	done := make(chan struct{})
	go func() {
		g.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		g.logger.Info("Host metrics generator stopped")
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// SupportedTelemetry returns the telemetry types this generator produces.
func (g *Generator) SupportedTelemetry() []telemetry.Type {
	return []telemetry.Type{telemetry.Metrics}
}

func (g *Generator) worker(id int) {
	defer g.wg.Done()

	seed := g.seed
	if seed < 0 {
		seed = time.Now().UnixNano() + int64(id)
	} else {
		seed += int64(id)
	}
	r := rand.New(rand.NewSource(seed)) // #nosec G404 -- seeded for determinism contract

	ticker := time.NewTicker(g.rate)
	defer ticker.Stop()

	for {
		select {
		case <-g.stopCh:
			return
		case <-ticker.C:
			g.scrape(r)
		}
	}
}

func (g *Generator) scrape(r *rand.Rand) {
	// Check count tracker
	if g.tracker != nil {
		if !g.tracker.Acquire() {
			return
		}
	}

	ctx := context.Background()

	// Build a fresh resource map per scrape. Future distributed-blitz
	// simulation may derive resource from the simulated host's
	// Environment record at scrape time — keep the allocation local so
	// no scrape-to-scrape mutation can bleed state. resource.Default
	// supplies telemetry.source + host.name (real process hostname); we
	// override host.name with the datagen-generated simulated hostname
	// because hostmetrics describes a simulated machine, not the host
	// blitz is running on.
	res := resource.Default(generatorType)
	res["host.name"] = g.hostname
	res["os.type"] = g.os

	for _, scraper := range g.scrapers {
		points := scraper.Scrape(r, g.hostname, res)
		if len(points) == 0 {
			continue
		}
		if err := g.consumer.ConsumeMetrics(ctx, points); err != nil {
			g.logger.Debug("Consume metrics error",
				zap.String("scraper", scraper.Name()),
				zap.Error(err),
			)
			generator.BlitzGeneratorWriteErrorsCounter.Add(ctx, 1, generatorType,
				metric.WithAttributeSet(attribute.NewSet(attribute.String("scraper", scraper.Name()))),
			)
			continue
		}
		generator.BlitzGeneratorEntriesCounter.Add(ctx, int64(len(points)), generatorType)
	}
}

// buildScrapers creates the scrapers based on the provided names.
// If names is empty, all scrapers are enabled.
func buildScrapers(names []string) []Scraper {
	all := allScrapers()

	if len(names) == 0 {
		return all
	}

	nameSet := make(map[string]struct{}, len(names))
	for _, n := range names {
		nameSet[n] = struct{}{}
	}

	var result []Scraper
	for _, s := range all {
		if _, ok := nameSet[s.Name()]; ok {
			result = append(result, s)
		}
	}
	return result
}

func allScrapers() []Scraper {
	return []Scraper{
		&cpuScraper{},
		&memoryScraper{},
		&diskScraper{},
		&networkScraper{},
		&filesystemScraper{},
		&loadScraper{},
		&pagingScraper{},
		&processesScraper{},
		&processScraper{},
	}
}
