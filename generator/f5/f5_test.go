package f5

import (
	"context"
	"errors"
	"math"
	"math/rand"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/metric"
	metricnoop "go.opentelemetry.io/otel/metric/noop"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/observiq/blitz/embed"
	"github.com/observiq/blitz/generator/count"
	"github.com/observiq/blitz/generator/f5/catalog"
	"github.com/observiq/blitz/telemetry"
)

func rngFor(seed int64) *rand.Rand { return rand.New(rand.NewSource(seed)) }

type captureConsumer struct {
	mu   sync.Mutex
	recs []embed.LogRecord
	err  error
}

func (c *captureConsumer) ConsumeLogs(_ context.Context, records []embed.LogRecord) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.recs = append(c.recs, records...)
	return c.err
}

func (c *captureConsumer) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.recs)
}

func (c *captureConsumer) messages(n int) []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]string, 0, n)
	for _, r := range c.recs[:n] {
		out = append(out, r.Message)
	}
	return out
}

func newGen(t *testing.T, cfg Config) (*Generator, *captureConsumer) {
	t.Helper()
	cc := &captureConsumer{}
	g, err := New(zap.NewNop(), cfg, cc, embed.TelemetrySettings{})
	require.NoError(t, err)
	return g, cc
}

// newCtx is a worker context for direct buildLine calls.
func newCtx(g *Generator) *catalog.Ctx {
	return &catalog.Ctx{Now: time.Date(2026, 9, 24, 15, 4, 5, 0, time.UTC), Hostname: g.cfg.Hostname, Device: g.device}
}

func TestNewValidates(t *testing.T) {
	cc := &captureConsumer{}
	_, err := New(nil, DefaultConfig(), cc, embed.TelemetrySettings{})
	require.Error(t, err)
	_, err = New(zap.NewNop(), DefaultConfig(), nil, embed.TelemetrySettings{})
	require.Error(t, err)
	_, err = New(zap.NewNop(), Config{Workers: 0, Rate: time.Second}, cc, embed.TelemetrySettings{})
	require.Error(t, err)
	_, err = New(zap.NewNop(), Config{Workers: 1, Rate: 0}, cc, embed.TelemetrySettings{})
	require.Error(t, err)
}

func TestNewReportsAMetricsBuildError(t *testing.T) {
	_, err := New(zap.NewNop(), DefaultConfig(), &captureConsumer{}, embed.TelemetrySettings{MeterProvider: failingMeterProvider{}})
	require.ErrorContains(t, err, "build generator metrics")
}

// failingMeterProvider's meters fail the first instrument the generator
// metrics build.
type failingMeterProvider struct{ metric.MeterProvider }

func (failingMeterProvider) Meter(string, ...metric.MeterOption) metric.Meter {
	return failingMeter{Meter: metricnoop.NewMeterProvider().Meter("test")}
}

type failingMeter struct{ metric.Meter }

func (failingMeter) Int64Gauge(string, ...metric.Int64GaugeOption) (metric.Int64Gauge, error) {
	return nil, errors.New("instrument error")
}

func TestNewRejectsBadProductsAndWeights(t *testing.T) {
	cc := &captureConsumer{}
	cases := map[string]Config{
		"unknown product":   {EnabledProducts: []string{"nope"}},
		"duplicate product": {EnabledProducts: []string{"afm", "asm", "afm"}},
		"zero weight":       {Weights: map[string]float64{"afm": 0}},
		"negative weight":   {Weights: map[string]float64{"asm": -2}},
		"unknown weight":    {Weights: map[string]float64{"nope": 1}},
		"NaN weight":        {Weights: map[string]float64{"asm": math.NaN()}},
		"infinite weight":   {Weights: map[string]float64{"asm": math.Inf(1)}},
		"disabled weight":   {EnabledProducts: []string{"asm"}, Weights: map[string]float64{"afm": 5}},
	}
	for name, cfg := range cases {
		cfg.Workers, cfg.Rate = 1, time.Second
		_, err := New(zap.NewNop(), cfg, cc, embed.TelemetrySettings{})
		require.Error(t, err, name)
	}
}

// TestSourcedProductsRegistered pins the product set: only products whose
// wire format is sourced from real samples ship (LTM request logs are
// operator-defined, so there is no real format to match).
func TestSourcedProductsRegistered(t *testing.T) {
	g, _ := newGen(t, DefaultConfig())
	var names []string
	for _, p := range g.products {
		names = append(names, p.Name)
	}
	require.ElementsMatch(t, []string{"afm", "apm", "asm", "audit", "dns", "f5os", "irules", "nginx-app-protect", "nginx-plus"}, names)
}

func TestEnabledProductsFilter(t *testing.T) {
	g, _ := newGen(t, Config{Workers: 1, Rate: time.Second, EnabledProducts: []string{"asm"}})
	require.Len(t, g.products, 1)
	c := newCtx(g)
	for i := 0; i < 50; i++ {
		require.Contains(t, g.buildLine(rngFor(int64(i)), c), " ASM:unit_hostname=")
	}
}

func TestWeightsBiasSelection(t *testing.T) {
	g, _ := newGen(t, Config{Workers: 1, Rate: time.Second, Weights: map[string]float64{"asm": 1000}})
	c := newCtx(g)
	asm := 0
	for i := 0; i < 500; i++ {
		if strings.Contains(g.buildLine(rngFor(int64(i)), c), " ASM:unit_hostname=") {
			asm++
		}
	}
	require.Greater(t, asm, 450, "asm weight 1000 should dominate")
}

// TestPickProductFallsThroughToLast covers the guard for a draw at or past
// the last cumulative weight.
func TestPickProductFallsThroughToLast(t *testing.T) {
	g, _ := newGen(t, Config{Workers: 1, Rate: time.Second, EnabledProducts: []string{"asm", "afm"}})
	g.total = 10 // beyond the real cumulative total of 2
	r := rngFor(1)
	for i := 0; i < 100; i++ {
		if p := g.pickProduct(r); p.Name != g.products[0].Name {
			require.Equal(t, g.products[len(g.products)-1].Name, p.Name)
		}
	}
}

// TestSeverityFollowsPRI checks every record's Severity is the one its line's
// PRI encodes, including non-INFO lines.
func TestSeverityFollowsPRI(t *testing.T) {
	g, cc := newGen(t, Config{Workers: 1, Rate: time.Millisecond, Seed: 3, EnabledProducts: []string{"nginx-plus", "irules"}})
	require.NoError(t, g.Start(context.Background()))
	require.Eventually(t, func() bool { return cc.count() >= 300 }, 5*time.Second, 5*time.Millisecond)
	require.NoError(t, g.Stop(context.Background()))
	cc.mu.Lock()
	defer cc.mu.Unlock()
	nonInfo := 0
	for _, r := range cc.recs {
		require.Equal(t, catalog.SeverityFromPRI(r.Message), r.Metadata.Severity, r.Message)
		if r.Metadata.Severity != "INFO" {
			nonInfo++
		}
	}
	require.Positive(t, nonInfo)
}

// TestDeterministicThroughWorkers runs two generators with the same seed and
// a fixed clock through the real worker loop and expects identical streams.
func TestDeterministicThroughWorkers(t *testing.T) {
	fclock := func() time.Time { return time.Date(2026, 9, 24, 15, 4, 5, 0, time.UTC) }
	run := func() []string {
		g, cc := newGen(t, Config{Workers: 1, Rate: time.Millisecond, Seed: 42, Hostname: "bigip1"})
		g.now = fclock
		require.NoError(t, g.Start(context.Background()))
		require.Eventually(t, func() bool { return cc.count() >= 40 }, 5*time.Second, 5*time.Millisecond)
		require.NoError(t, g.Stop(context.Background()))
		return cc.messages(40)
	}
	require.Equal(t, run(), run())
}

func TestCountTrackerLimitsEmission(t *testing.T) {
	g, cc := newGen(t, Config{Workers: 3, Rate: time.Millisecond, Seed: 1})
	g.SetCountTracker(count.NewTracker(7))
	require.NoError(t, g.Start(context.Background()))
	require.Eventually(t, func() bool { return cc.count() == 7 }, 5*time.Second, 5*time.Millisecond)
	require.Never(t, func() bool { return cc.count() > 7 }, 100*time.Millisecond, 10*time.Millisecond)
	require.NoError(t, g.Stop(context.Background()))
}

func TestConsumeErrorKeepsEmitting(t *testing.T) {
	cc := &captureConsumer{err: errors.New("sink down")}
	g, err := New(zap.NewNop(), Config{Workers: 1, Rate: time.Millisecond, Seed: 1}, cc, embed.TelemetrySettings{})
	require.NoError(t, err)
	require.NoError(t, g.Start(context.Background()))
	require.Eventually(t, func() bool { return cc.count() >= 3 }, 5*time.Second, 5*time.Millisecond)
	require.NoError(t, g.Stop(context.Background()))
}

// blockingConsumer holds every write until released, ignoring the write's
// context, and signals each write it starts.
type blockingConsumer struct{ entered, release chan struct{} }

func (b blockingConsumer) ConsumeLogs(context.Context, []embed.LogRecord) error {
	select {
	case b.entered <- struct{}{}:
	default:
	}
	<-b.release
	return nil
}

// TestStopHonorsContextCancel holds the worker inside a write, so Stop's wait
// cannot finish and only the cancelled context can end it.
func TestStopHonorsContextCancel(t *testing.T) {
	bc := blockingConsumer{entered: make(chan struct{}, 1), release: make(chan struct{})}
	g, err := New(zap.NewNop(), Config{Workers: 1, Rate: time.Millisecond, Seed: 1}, bc, embed.TelemetrySettings{})
	require.NoError(t, err)
	require.NoError(t, g.Start(context.Background()))
	<-bc.entered
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	require.ErrorIs(t, g.Stop(ctx), context.Canceled)
	close(bc.release)
	require.NoError(t, g.Stop(context.Background()))
}

func TestSupportedTelemetry(t *testing.T) {
	g, _ := newGen(t, DefaultConfig())
	require.Equal(t, []telemetry.Type{telemetry.Logs}, g.SupportedTelemetry())
}

// TestCountTrackerResumeAndStop covers an exhausted worker: once parked it
// resumes after Reset, and Stop still returns while it waits.
func TestCountTrackerResumeAndStop(t *testing.T) {
	core, logs := observer.New(zap.DebugLevel)
	cc := &captureConsumer{}
	g, err := New(zap.New(core), Config{Workers: 1, Rate: time.Millisecond, Seed: 1}, cc, embed.TelemetrySettings{})
	require.NoError(t, err)
	tr := count.NewTracker(2)
	g.SetCountTracker(tr)
	require.NoError(t, g.Start(context.Background()))
	require.Eventually(t, func() bool { return cc.count() == 2 }, 5*time.Second, 5*time.Millisecond)
	require.Eventually(t, func() bool { return logs.FilterMessage("F5 worker waiting for a count reset").Len() == 1 }, 5*time.Second, time.Millisecond)
	tr.Reset()
	require.Eventually(t, func() bool { return cc.count() == 4 }, 5*time.Second, 5*time.Millisecond)
	require.NoError(t, g.Stop(context.Background()))
}

// TestNameAndFallbackHosts pins the module name and the hosts without an
// environment: the BIG-IP unit is bigip1 (or the set hostname) and F5OS the
// platform's appliance-1.
func TestNameAndFallbackHosts(t *testing.T) {
	g, _ := newGen(t, DefaultConfig())
	require.Equal(t, "f5", g.Name())
	require.Equal(t, "bigip1", g.hosts[catalog.HostBIGIP].name)
	require.Equal(t, "bigip1", g.hosts[catalog.HostBIGIP].static.Record()["host.name"])
	require.Equal(t, "appliance-1", g.hosts[catalog.HostPlatform].name)

	cfg := DefaultConfig()
	cfg.Hostname = "lb-prod-01"
	explicit, _ := newGen(t, cfg)
	require.Equal(t, "lb-prod-01", explicit.hosts[catalog.HostBIGIP].name)
}

// TestRandomSeedIsLoggedAndReproduces pins the seed contract through the real
// worker loop: a negative seed is resolved once, logged at Start, and rerunning
// with the logged seed reproduces the stream.
func TestRandomSeedIsLoggedAndReproduces(t *testing.T) {
	fclock := func() time.Time { return time.Date(2026, 9, 24, 15, 4, 5, 0, time.UTC) }
	run := func(seed int64) ([]string, int64) {
		core, logs := observer.New(zap.InfoLevel)
		cc := &captureConsumer{}
		g, err := New(zap.New(core), Config{Workers: 1, Rate: time.Millisecond, Seed: seed, Hostname: "bigip1"}, cc, embed.TelemetrySettings{})
		require.NoError(t, err)
		g.now = fclock
		require.NoError(t, g.Start(context.Background()))
		require.Eventually(t, func() bool { return cc.count() >= 40 }, 5*time.Second, 5*time.Millisecond)
		require.NoError(t, g.Stop(context.Background()))
		start := logs.FilterMessage("Starting F5 generator").All()
		require.Len(t, start, 1)
		logged, ok := start[0].ContextMap()["seed"].(int64)
		require.True(t, ok, "start log has no int64 seed field")
		return cc.messages(40), logged
	}
	first, seed := run(-1)
	require.Greater(t, seed, int64(math.MaxInt32), "the logged seed is the resolved one, from the full clock rather than 31 bits")
	again, logged := run(seed)
	require.Equal(t, seed, logged)
	require.Equal(t, first, again, "rerunning with the logged seed reproduces the stream")
}

// TestDeviceDrawIsIndependentOfWorkers pins that the device is drawn from its
// own stream, not worker 0's (seed+0), so its management IP never mirrors
// that worker's first private address.
func TestDeviceDrawIsIndependentOfWorkers(t *testing.T) {
	for seed := int64(0); seed < 20; seed++ {
		g, err := New(zap.NewNop(), Config{Workers: 1, Rate: time.Hour, Seed: seed}, &captureConsumer{}, embed.TelemetrySettings{})
		require.NoError(t, err)
		worker0 := catalog.NewDevice(rand.New(rand.NewSource(seed))) // #nosec G404 -- test
		require.NotEqual(t, worker0.MgmtIP, g.device.MgmtIP, "seed %d", seed)
	}
}

// TestWeightErrorsAreDeterministic pins that with several bad weights the
// error always names the same product (the first by name), run after run.
func TestWeightErrorsAreDeterministic(t *testing.T) {
	weights := map[string]float64{"zz-unknown": 1, "mm-unknown": 1, "aa-unknown": 1}
	for range 50 {
		require.ErrorContains(t, validateWeights(weights, nil), `"aa-unknown"`)
	}
}

// TestWeightsMustSumFinite pins that weights whose total overflows float64
// are rejected: an infinite total would always pick the last product.
func TestWeightsMustSumFinite(t *testing.T) {
	require.ErrorContains(t, validateWeights(map[string]float64{"afm": 1e308, "asm": 1e308}, nil), "sum")
}

// buildLine is build without the resource.
func (g *Generator) buildLine(r *rand.Rand, c *catalog.Ctx) string {
	line, _ := g.build(r, c)
	return line
}

// TestStopTwiceIsSafe pins that a second Stop returns instead of panicking on
// the already-closed stop channel.
func TestStopTwiceIsSafe(t *testing.T) {
	g, _ := newGen(t, Config{Workers: 1, Rate: time.Hour, Seed: 1})
	require.NoError(t, g.Start(context.Background()))
	require.NoError(t, g.Stop(context.Background()))
	require.NotPanics(t, func() { require.NoError(t, g.Stop(context.Background())) })
}

// countingFailConsumer fails every write and counts them.
type countingFailConsumer struct{ n atomic.Int64 }

func (c *countingFailConsumer) ConsumeLogs(context.Context, []embed.LogRecord) error {
	c.n.Add(1)
	return errors.New("sink down")
}

// TestWriteFailuresWarnSampled pins that a failing consumer is reported at
// Warn, sampled so a dead output cannot flood the log at the line rate.
func TestWriteFailuresWarnSampled(t *testing.T) {
	core, logs := observer.New(zap.WarnLevel)
	cc := &countingFailConsumer{}
	g, err := New(zap.New(core), Config{Workers: 1, Rate: time.Millisecond, Seed: 1}, cc, embed.TelemetrySettings{})
	require.NoError(t, err)
	require.NoError(t, g.Start(context.Background()))
	require.Eventually(t, func() bool { return cc.n.Load() >= 300 }, 10*time.Second, time.Millisecond)
	require.NoError(t, g.Stop(context.Background()))
	warned := logs.FilterMessage("F5 write failed").Len()
	require.Positive(t, warned, "a dead output is visible at Warn")
	require.Less(t, int64(warned), cc.n.Load()/10, "failures are sampled, not logged per line")
}

// signalConsumer reports each write on arrived, then holds it until release.
type signalConsumer struct{ arrived, release chan struct{} }

func (s signalConsumer) ConsumeLogs(ctx context.Context, _ []embed.LogRecord) error {
	select {
	case s.arrived <- struct{}{}:
	default:
	}
	select {
	case <-s.release:
	case <-ctx.Done():
	}
	return nil
}

// TestEntryCountedBeforeSend pins that F5 counts an entry before handing it to
// the consumer, as apache and nginx do.
func TestEntryCountedBeforeSend(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	tel := embed.TelemetrySettings{MeterProvider: sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))}
	sc := signalConsumer{arrived: make(chan struct{}, 1), release: make(chan struct{})}
	g, err := New(zap.NewNop(), Config{Workers: 1, Rate: time.Millisecond, Seed: 1}, sc, tel)
	require.NoError(t, err)
	require.NoError(t, g.Start(context.Background()))
	t.Cleanup(func() { close(sc.release); _ = g.Stop(context.Background()) })
	<-sc.arrived

	var rm metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(context.Background(), &rm))
	var entries int64
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name == "blitz.generator.entries" {
				for _, dp := range m.Data.(metricdata.Sum[int64]).DataPoints {
					entries += dp.Value
				}
			}
		}
	}
	require.Equal(t, int64(1), entries, "the entry in flight is already counted")
}
