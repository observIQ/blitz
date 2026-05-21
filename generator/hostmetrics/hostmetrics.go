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
	"github.com/observiq/blitz/internal/datagen"
	"github.com/observiq/blitz/output"
	"github.com/observiq/blitz/telemetry"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"
)

const generatorType = "hostmetrics"

// Generator implements generator.MetricGenerator for host metrics.
type Generator struct {
	embed.ProducerMarker

	logger   *zap.Logger
	workers  int
	rate     time.Duration
	os       string
	hostname string
	scrapers []Scraper
	wg       sync.WaitGroup
	stopCh   chan struct{}
	tracker  *count.Tracker
}

// Compile-time assertion that Generator implements MetricGenerator.
var _ generator.MetricGenerator = (*Generator)(nil)

// New creates a new host metrics generator.
func New(logger *zap.Logger, workers int, rate time.Duration, os string, hostname string, scraperNames []string) (*Generator, error) {
	if logger == nil {
		return nil, fmt.Errorf("logger cannot be nil")
	}
	if workers < 1 {
		return nil, fmt.Errorf("workers must be 1 or greater, got %d", workers)
	}
	if rate <= 0 {
		return nil, fmt.Errorf("rate must be greater than 0, got %s", rate)
	}

	// Generate a hostname if not provided
	if hostname == "" {
		style := datagen.StyleLinux
		if os == "windows" {
			style = datagen.StyleWindows
		}
		hostname = datagen.GenerateHostname(
			rand.New(rand.NewSource(time.Now().UnixNano())), // #nosec G404
			style,
			datagen.AllMythologyNames,
		)
	}

	scrapers := buildScrapers(scraperNames)

	return &Generator{
		logger:   logger.Named("generator-hostmetrics"),
		workers:  workers,
		rate:     rate,
		os:       os,
		hostname: hostname,
		scrapers: scrapers,
		stopCh:   make(chan struct{}),
	}, nil
}

// SetCountTracker sets the count tracker for finite generation.
func (g *Generator) SetCountTracker(tracker *count.Tracker) {
	g.tracker = tracker
}

// Start starts the host metrics generator.
func (g *Generator) Start(writer output.MetricWriter) error {
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
		go g.worker(i, writer)
	}

	return nil
}

// Stop stops the host metrics generator.
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

func (g *Generator) worker(id int, writer output.MetricWriter) {
	defer g.wg.Done()

	r := rand.New(rand.NewSource(time.Now().UnixNano() + int64(id))) // #nosec G404

	resource := map[string]string{
		"host.name": g.hostname,
		"os.type":   g.os,
	}

	ticker := time.NewTicker(g.rate)
	defer ticker.Stop()

	for {
		select {
		case <-g.stopCh:
			return
		case <-ticker.C:
			g.scrape(r, writer, resource)
		}
	}
}

func (g *Generator) scrape(r *rand.Rand, writer output.MetricWriter, resource map[string]string) {
	// Check count tracker
	if g.tracker != nil {
		if !g.tracker.Acquire() {
			return
		}
	}

	ctx := context.Background()

	for _, scraper := range g.scrapers {
		records := scraper.Scrape(r, g.hostname, resource)
		for _, rec := range records {
			if err := writer.WriteMetric(ctx, rec); err != nil {
				g.logger.Debug("Write metric error",
					zap.String("scraper", scraper.Name()),
					zap.Error(err),
				)
				generator.BlitzGeneratorWriteErrorsCounter.Add(ctx, 1, generatorType,
					metric.WithAttributeSet(attribute.NewSet(attribute.String("scraper", scraper.Name()))),
				)
				continue
			}
			generator.BlitzGeneratorEntriesCounter.Add(ctx, 1, generatorType)
		}
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
	}
}
