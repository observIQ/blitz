// Package f5 is the multi-product F5 log generator. It emits a weighted
// mix of syslog-shaped log lines across the F5 product portfolio
// (BIG-IP ASM/AFM/APM/DNS/audit, NGINX-on-F5 Plus + App Protect,
// iRules, F5OS), each product modeled in its own subpackage under
// generator/f5/products and self-registered into generator/f5/catalog.
//
// Architecture mirrors the FIX generator: a catalog of registered
// products, per-product subpackages, and this top-level Generator that
// spawns workers each running a deterministic emit loop seeded from the
// user's Seed and pushing records into an embed.LogConsumer.
//
// All workers speak for one simulated unit: a single catalog.Device fixes
// the management IP and daemon PIDs, while each worker keeps its own
// product state (open APM sessions, pending DNS queries).
//
// Determinism: same Seed + same product set + same start time = same output
// stream per worker. Lines carry the emission time, so the clock is part of
// the output.
package f5

import (
	"context"
	"fmt"
	"maps"
	"math"
	"math/rand"
	"slices"
	"sort"
	"sync"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/observiq/blitz/embed"
	"github.com/observiq/blitz/generator"
	"github.com/observiq/blitz/generator/count"
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
	// (e.g. "asm", "afm"). Empty = all registered products. Each name may
	// appear once.
	EnabledProducts []string
	// Weights sets the relative mix ratio per product name. Each weight must
	// be positive and name a registered product; a product absent from the
	// map (or the whole map empty) defaults to weight 1.
	Weights map[string]float64
	// Seed is the base RNG seed. Negative = pick a random seed once (logged
	// at Start so the run can be reproduced); 0+ = deterministic. Worker N
	// gets Seed+N.
	Seed int64
}

// DefaultConfig returns a Config with sensible defaults: one worker, 1s
// rate, all products at equal weight, randomized seed.
func DefaultConfig() Config {
	return Config{Workers: 1, Rate: time.Second, Seed: -1}
}

// Generator emits F5 log lines at the configured rate.
type Generator struct {
	embed.ProducerMarker

	logger *zap.Logger
	// warn reports write failures, sampled so a dead output cannot flood the
	// log at the line rate.
	warn     *zap.Logger
	cfg      Config
	consumer embed.LogConsumer
	metrics  *generator.Metrics
	// hosts is each product host's one name and resource; read-only after New.
	hosts map[catalog.Host]host

	products  []catalog.Product
	cumWeight []float64 // cumulative weights, parallel to products
	total     float64

	// device is the simulated unit, shared read-only by every worker.
	device *catalog.Device
	// now is the clock stamped on each line; tests replace it.
	now func() time.Time
	// tracker, when set, caps the total number of lines (generator.count).
	tracker *count.Tracker

	wg       sync.WaitGroup
	stopCh   chan struct{}
	stopOnce sync.Once
}

// New constructs an F5 Generator. Returns an error for invalid inputs
// (nil logger/consumer, workers < 1, non-positive rate), an EnabledProducts
// entry that names no registered product or repeats one, or a weight that
// is not positive or names no registered product.
func New(logger *zap.Logger, cfg Config, consumer embed.LogConsumer, tel embed.TelemetrySettings, opts ...generator.Option) (*Generator, error) {
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
	explicitHostname := cfg.Hostname != ""
	if err := validateWeights(cfg.Weights, cfg.EnabledProducts); err != nil {
		return nil, err
	}

	products, err := selectProducts(cfg.EnabledProducts)
	if err != nil {
		return nil, err
	}

	metrics, err := generator.NewMetrics(tel.MeterProvider)
	if err != nil {
		return nil, fmt.Errorf("build generator metrics: %w", err)
	}

	// Resolve a random seed once, so the logged value reproduces the run:
	// it seeds the device and, as Seed+N, worker N.
	if cfg.Seed < 0 {
		cfg.Seed = time.Now().UnixNano()
	}
	o := generator.NewOptions(opts...)
	// seed-1: workers use seed+N (N >= 0), so the device has its own stream.
	device := catalog.NewDevice(rand.New(rand.NewSource(cfg.Seed - 1))) // #nosec G404 -- simulated identity, not security
	if a := o.Appliance; a != nil && a.ManagementInterface != nil && a.ManagementInterface.IPv4 != "" {
		device.MgmtIP = a.ManagementInterface.IPv4
	}
	g := &Generator{
		logger: logger,
		warn: logger.WithOptions(zap.WrapCore(func(core zapcore.Core) zapcore.Core {
			return zapcore.NewSamplerWithOptions(core, warnWindow, 1, warnThereafter)
		})),
		cfg:      cfg,
		consumer: consumer,
		metrics:  metrics,
		hosts:    resolveHosts(cfg.Hostname, explicitHostname, o),
		products: products,
		device:   device,
		now:      time.Now,
		stopCh:   make(chan struct{}),
	}
	g.buildWeights()
	return g, nil
}

// validateWeights rejects a weight that is not positive, names no registered
// product (so a typo cannot silently fall back to weight 1), or names a
// product outside a non-empty enabled list (it would be silently ignored), and
// weights whose sum overflows, which would always pick the last product.
func validateWeights(weights map[string]float64, enabled []string) error {
	var sum float64
	for _, name := range slices.Sorted(maps.Keys(weights)) {
		w := weights[name]
		if _, ok := catalog.Get(name); !ok {
			return fmt.Errorf("weights: unknown f5 product %q", name)
		}
		if len(enabled) > 0 && !slices.Contains(enabled, name) {
			return fmt.Errorf("weights: f5 product %q is not in enabledProducts", name)
		}
		if !(w > 0) || math.IsInf(w, 1) { // !(w > 0) also catches NaN
			return fmt.Errorf("weights: %q must be positive and finite, got %v", name, w)
		}
		sum += w
	}
	if math.IsInf(sum, 1) {
		return fmt.Errorf("weights: sum must be finite, got %v", sum)
	}
	return nil
}

// selectProducts returns the enabled products (all when enabled is empty),
// erroring on any name that is not registered or is listed twice.
func selectProducts(enabled []string) ([]catalog.Product, error) {
	if len(enabled) == 0 {
		return catalog.AllProducts(), nil
	}
	out := make([]catalog.Product, 0, len(enabled))
	seen := make(map[string]bool, len(enabled))
	for _, name := range enabled {
		if seen[name] {
			return nil, fmt.Errorf("f5 product %q listed more than once", name)
		}
		seen[name] = true
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
// selection. A product missing from Weights defaults to 1.
func (g *Generator) buildWeights() {
	g.cumWeight = make([]float64, len(g.products))
	var acc float64
	for i, p := range g.products {
		w := 1.0
		if wv, ok := g.cfg.Weights[p.Name]; ok {
			w = wv
		}
		acc += w
		g.cumWeight[i] = acc
	}
	g.total = acc
}

// pickProduct selects a product by weighted random draw from r.
func (g *Generator) pickProduct(r *rand.Rand) catalog.Product {
	x := r.Float64() * g.total
	for i, cum := range g.cumWeight {
		if x < cum {
			return g.products[i]
		}
	}
	return g.products[len(g.products)-1]
}

// Name returns the module identifier.
func (g *Generator) Name() string { return componentName }

// SetCountTracker sets the finite generation count tracker. Must be called
// before Start.
func (g *Generator) SetCountTracker(t *count.Tracker) { g.tracker = t }

// Start launches the worker goroutines.
func (g *Generator) Start(_ context.Context) error {
	g.logger.Info("Starting F5 generator",
		zap.Int("workers", g.cfg.Workers),
		zap.Duration("rate", g.cfg.Rate),
		zap.Int("products", len(g.products)),
		zap.Int64("seed", g.cfg.Seed),
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
	g.stopOnce.Do(func() { close(g.stopCh) })

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

	r := rand.New(rand.NewSource(g.cfg.Seed + int64(workerIdx))) // #nosec G404 -- seeded for determinism contract
	c := &catalog.Ctx{Device: g.device}

	ticker := time.NewTicker(g.cfg.Rate)
	defer ticker.Stop()

	for {
		select {
		case <-g.stopCh:
			return
		case <-ticker.C:
			if g.tracker != nil {
				// Take the resume channel before Acquire, so a Reset that lands
				// between a failed Acquire and the wait still wakes the worker.
				resume := g.tracker.ResumeC()
				if !g.tracker.Acquire() {
					g.logger.Debug("F5 worker waiting for a count reset", zap.Int("worker_id", workerIdx))
					select {
					case <-g.stopCh:
						return
					case <-resume:
						continue
					}
				}
			}
			c.Now = g.now()
			g.emit(g.build(r, c))
		}
	}
}

// Write-failure warnings are sampled: the first in each window logs at once,
// then one in every warnThereafter until the window ends.
const (
	warnWindow     = time.Second
	warnThereafter = 100
)

// emit counts one line, then pushes it to the consumer, its severity read
// from the line's PRI.
func (g *Generator) emit(line string, static *resource.StaticResources) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	rec := embed.LogRecord{
		Message: line,
		Metadata: embed.LogRecordMetadata{
			Severity: catalog.SeverityFromPRI(line),
			Resource: static.Record(),
			// Every product's line carries the device's own <PRI> and header.
			SyslogFramed: true,
		},
	}
	g.metrics.BlitzGeneratorEntriesCounter.Add(context.Background(), 1, componentName)
	if err := g.consumer.ConsumeLogs(ctx, []embed.LogRecord{rec}); err != nil {
		g.warn.Warn("F5 write failed", zap.Error(err))
		g.metrics.BlitzGeneratorWriteErrorsCounter.Add(context.Background(), 1, componentName,
			metric.WithAttributeSet(attribute.NewSet(attribute.String("error_type", "consume"))),
		)
	}
}

// build picks a weighted product and builds one line from it on the
// product's host, returning the line and that host's resource.
func (g *Generator) build(r *rand.Rand, c *catalog.Ctx) (string, *resource.StaticResources) {
	p := g.pickProduct(r)
	h := g.hosts[p.Host]
	c.Hostname, c.FQDN = h.name, h.fqdn
	return p.Build(r, c), h.static
}

// SupportedTelemetry reports the telemetry types this generator emits.
func (g *Generator) SupportedTelemetry() []telemetry.Type {
	return []telemetry.Type{telemetry.Logs}
}

// platformHostname is the F5OS platform's default system hostname, as rSeries
// audit logs show it.
const platformHostname = "appliance-1"

// host is one machine a product logs from: its name, its fully qualified
// name (empty when it has none), and its resource.
type host struct {
	name, fqdn string
	static     *resource.StaticResources
}

// resolveHosts gives each product host one name, used in the line and in
// host.name. The BIG-IP unit is the environment's F5 appliance (an explicit
// hostname replaces only its name); NGINX hosts are the environment server
// host; the F5OS platform is appliance-1. Without an environment, the BIG-IP
// unit is the configured hostname (bigip1) and NGINX the running host. An
// appliance without a name is named bigip1.
func resolveHosts(hostname string, explicit bool, o generator.Options) map[catalog.Host]host {
	bigip := o.Appliance
	if bigip != nil && explicit {
		c := *bigip
		c.Hostname, c.FQDN = hostname, "" // an explicit hostname has no domain
		bigip = &c
	} else if bigip != nil && bigip.Hostname == "" {
		c := *bigip
		c.Hostname = defaultHostname // a hand-built appliance missing a name
		bigip = &c
	}
	bigipHost := host{name: hostname, static: resource.NewStaticResources(resource.WithHost(hostname, componentName))}
	if hostname == "" {
		bigipHost.name = defaultHostname
		bigipHost.static = resource.NewStaticResources(resource.WithHost(defaultHostname, componentName))
	}
	if bigip != nil {
		bigipHost = host{name: bigip.Hostname, fqdn: bigip.FQDN, static: resource.FromAppliance(bigip, componentName)}
	}

	id := o.Identity
	if id != nil && id.Hostname == "" {
		id = nil // an identity without a name describes no host: use the process host
	}
	nginx := resource.FromIdentity(id, componentName)
	platform := &datagen.NetworkSystemIdentity{Hostname: platformHostname, Vendor: datagen.VendorF5}
	return map[catalog.Host]host{
		catalog.HostBIGIP:    bigipHost,
		catalog.HostNGINX:    {name: nginx.Record()["host.name"].(string), static: nginx},
		catalog.HostPlatform: {name: platformHostname, static: resource.FromAppliance(platform, componentName)},
	}
}
