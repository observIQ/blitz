// Package f5 is the multi-product F5 log generator. It emits a weighted
// mix of syslog-shaped log lines across the F5 product portfolio
// (BIG-IP LTM/ASM/AFM/APM/DNS/audit, NGINX-on-F5 Plus + App Protect,
// iRules, F5OS/TMOS), each product modeled in its own subpackage under
// generator/f5/products and self-registered into generator/f5/catalog.
//
// Architecture mirrors the FIX generator: a catalog of registered
// products, per-product subpackages, and this top-level Generator that
// spawns workers each running a deterministic emit loop seeded from the
// user's Seed and pushing records into an embed.LogConsumer.
//
// Determinism: same Seed + same product set = same output stream per
// worker.
package f5

import (
	"context"
	"fmt"
	"math/rand"
	"sort"
	"sync"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"

	"github.com/observiq/blitz/embed"
	"github.com/observiq/blitz/generator"
	"github.com/observiq/blitz/generator/f5/catalog"
	"github.com/observiq/blitz/generator/resource"
	"github.com/observiq/blitz/internal/datagen"
	"github.com/observiq/blitz/telemetry"

	// Register every F5 product.
	_ "github.com/observiq/blitz/generator/f5/products/afm"
	_ "github.com/observiq/blitz/generator/f5/products/apm"
	_ "github.com/observiq/blitz/generator/f5/products/appprotect"
	_ "github.com/observiq/blitz/generator/f5/products/asm"
	_ "github.com/observiq/blitz/generator/f5/products/audit"
	_ "github.com/observiq/blitz/generator/f5/products/dns"
	_ "github.com/observiq/blitz/generator/f5/products/f5os"
	_ "github.com/observiq/blitz/generator/f5/products/irules"
	_ "github.com/observiq/blitz/generator/f5/products/ltm"
	_ "github.com/observiq/blitz/generator/f5/products/nginxplus"
)

const componentName = "f5"

// defaultHostname is the simulated F5 device hostname when none is configured.
const defaultHostname = "bigip1"

// Config configures the F5 generator.
type Config struct {
	// Workers spawned for parallel emission. Each worker has its own RNG.
	Workers int
	// Rate is the per-worker emission interval (one line per Rate).
	Rate time.Duration
	// Hostname is the simulated F5 device hostname in the syslog header.
	Hostname string
	// EnabledProducts restricts emission to a subset of product names
	// (e.g. "ltm", "asm"). Empty = all registered products.
	EnabledProducts []string
	// Weights sets the relative mix ratio per product name. A product
	// absent from the map (or the whole map empty) defaults to weight 1.
	Weights map[string]float64
	// Seed is the base RNG seed. Negative = randomize per worker; 0+ =
	// deterministic (worker N gets Seed+N).
	Seed int64
}

// DefaultConfig returns a Config with sensible defaults: one worker, 1s
// rate, all products at equal weight, randomized seed.
func DefaultConfig() Config {
	return Config{Workers: 1, Rate: time.Second, Hostname: defaultHostname, Seed: -1}
}

// Generator emits F5 log lines at the configured rate.
type Generator struct {
	embed.ProducerMarker

	logger   *zap.Logger
	cfg      Config
	consumer embed.LogConsumer
	static   *resource.StaticResources
	metrics  *generator.Metrics

	products []catalog.Product
	cumWeit  []float64 // cumulative weights, parallel to products
	total    float64

	wg     sync.WaitGroup
	stopCh chan struct{}
}

// New constructs an F5 Generator. Returns an error for invalid inputs
// (nil logger/consumer, workers < 1, non-positive rate) or an
// EnabledProducts entry that names no registered product.
func New(logger *zap.Logger, cfg Config, consumer embed.LogConsumer, tel embed.TelemetrySettings) (*Generator, error) {
	if logger == nil {
		return nil, fmt.Errorf("logger cannot be nil")
	}
	if consumer == nil {
		return nil, fmt.Errorf("consumer cannot be nil")
	}
	if cfg.Workers < 1 {
		return nil, fmt.Errorf("workers must be 1 or greater, got %d", cfg.Workers)
	}
	if cfg.Rate <= 0 {
		return nil, fmt.Errorf("rate must be positive, got %v", cfg.Rate)
	}
	if cfg.Hostname == "" {
		cfg.Hostname = defaultHostname
	}

	products, err := selectProducts(cfg.EnabledProducts)
	if err != nil {
		return nil, err
	}

	metrics, err := generator.NewMetrics(tel.MeterProvider)
	if err != nil {
		return nil, fmt.Errorf("build generator metrics: %w", err)
	}

	g := &Generator{
		logger:   logger,
		cfg:      cfg,
		consumer: consumer,
		static:   resource.FromIdentity(nil, componentName),
		metrics:  metrics,
		products: products,
		stopCh:   make(chan struct{}),
	}
	g.buildWeights()
	return g, nil
}

// selectProducts returns the enabled products (all when enabled is empty),
// erroring on any name that is not registered.
func selectProducts(enabled []string) ([]catalog.Product, error) {
	if len(enabled) == 0 {
		return catalog.AllProducts(), nil
	}
	out := make([]catalog.Product, 0, len(enabled))
	for _, name := range enabled {
		p, ok := catalog.Get(name)
		if !ok {
			return nil, fmt.Errorf("unknown f5 product %q", name)
		}
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// buildWeights precomputes the cumulative-weight table for weighted product
// selection. A product missing from Weights (or a non-positive weight)
// defaults to 1.
func (g *Generator) buildWeights() {
	g.cumWeit = make([]float64, len(g.products))
	var acc float64
	for i, p := range g.products {
		w := 1.0
		if wv, ok := g.cfg.Weights[p.Name]; ok && wv > 0 {
			w = wv
		}
		acc += w
		g.cumWeit[i] = acc
	}
	g.total = acc
}

// pickProduct selects a product by weighted random draw from r.
func (g *Generator) pickProduct(r *rand.Rand) catalog.Product {
	x := r.Float64() * g.total
	for i, cum := range g.cumWeit {
		if x < cum {
			return g.products[i]
		}
	}
	return g.products[len(g.products)-1]
}

// Name returns the module identifier.
func (g *Generator) Name() string { return componentName }

// SetHostIdentity sets the simulated host whose identity every emitted record
// carries (PIPE-1036). A nil identity keeps the process-hostname fallback.
func (g *Generator) SetHostIdentity(id *datagen.SystemIdentity) {
	g.static = resource.FromIdentity(id, componentName)
}

// Start launches the worker goroutines.
func (g *Generator) Start(_ context.Context) error {
	g.logger.Info("Starting F5 generator",
		zap.Int("workers", g.cfg.Workers),
		zap.Duration("rate", g.cfg.Rate),
		zap.Int("products", len(g.products)),
	)
	g.metrics.BlitzGeneratorActiveWorkersGauge.Record(context.Background(), int64(g.cfg.Workers), componentName)
	for i := 0; i < g.cfg.Workers; i++ {
		g.wg.Add(1)
		go g.runWorker(i) // #nosec G118 -- workers bounded by Stop() and the WaitGroup
	}
	return nil
}

// Stop signals workers to drain and waits for them to exit.
func (g *Generator) Stop(ctx context.Context) error {
	g.logger.Info("Stopping F5 generator")
	g.metrics.BlitzGeneratorActiveWorkersGauge.Record(context.Background(), 0, componentName)
	close(g.stopCh)

	done := make(chan struct{})
	go func() {
		g.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("stop cancelled due to context cancellation: %w", ctx.Err())
	}
}

func (g *Generator) runWorker(workerIdx int) {
	defer g.wg.Done()

	seed := g.cfg.Seed
	if seed < 0 {
		seed = time.Now().UnixNano() + int64(workerIdx)
	} else {
		seed += int64(workerIdx)
	}
	r := rand.New(rand.NewSource(seed)) // #nosec G404 -- seeded for determinism contract

	ticker := time.NewTicker(g.cfg.Rate)
	defer ticker.Stop()

	for {
		select {
		case <-g.stopCh:
			return
		case <-ticker.C:
			line := g.buildLine(r)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			rec := embed.LogRecord{
				Message: line,
				Metadata: embed.LogRecordMetadata{
					Severity: "INFO",
					Resource: g.static.Record(),
				},
			}
			if err := g.consumer.ConsumeLogs(ctx, []embed.LogRecord{rec}); err != nil {
				g.logger.Debug("F5 emit failed", zap.Error(err))
				g.metrics.BlitzGeneratorWriteErrorsCounter.Add(context.Background(), 1, componentName,
					metric.WithAttributeSet(attribute.NewSet(attribute.String("error_type", "consume"))),
				)
			}
			g.metrics.BlitzGeneratorEntriesCounter.Add(context.Background(), 1, componentName)
			cancel()
		}
	}
}

// buildLine picks a weighted product and builds one log line from it.
func (g *Generator) buildLine(r *rand.Rand) string {
	p := g.pickProduct(r)
	return p.Build(r, &catalog.Ctx{Now: time.Now(), Hostname: g.cfg.Hostname})
}

// SupportedTelemetry reports that this generator produces logs.
func (g *Generator) SupportedTelemetry() []telemetry.Type {
	return []telemetry.Type{telemetry.Logs}
}
