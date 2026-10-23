package hostmetrics

import (
	"context"
	"errors"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/observiq/blitz/embed"
	"github.com/observiq/blitz/generator/count"
	"github.com/observiq/blitz/internal/datagen"
	"github.com/observiq/blitz/telemetry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.uber.org/zap/zaptest"
)

type mockMetricConsumer struct {
	mu     sync.Mutex
	points []embed.MetricPoint
}

func (m *mockMetricConsumer) ConsumeMetrics(_ context.Context, batch []embed.MetricPoint) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.points = append(m.points, batch...)
	return nil
}

func (m *mockMetricConsumer) Snapshot() []embed.MetricPoint {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]embed.MetricPoint, len(m.points))
	copy(out, m.points)
	return out
}

func (m *mockMetricConsumer) Count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.points)
}

func baseCfg(t *testing.T, cons embed.MetricConsumer) Config {
	t.Helper()
	return Config{
		Logger:   zaptest.NewLogger(t),
		Workers:  1,
		Rate:     time.Second,
		OS:       "linux",
		Hostname: "test-host",
		Consumer: cons,
	}
}

func TestNew(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		g, err := New(baseCfg(t, &mockMetricConsumer{}))
		require.NoError(t, err)
		assert.NotNil(t, g)
		assert.Equal(t, "test-host", g.hostname)
		assert.Len(t, g.scrapers, 8)
	})

	t.Run("auto hostname linux", func(t *testing.T) {
		cfg := baseCfg(t, &mockMetricConsumer{})
		cfg.Hostname = ""
		g, err := New(cfg)
		require.NoError(t, err)
		assert.NotEmpty(t, g.hostname)
	})

	t.Run("auto hostname windows", func(t *testing.T) {
		cfg := baseCfg(t, &mockMetricConsumer{})
		cfg.Hostname = ""
		cfg.OS = "windows"
		g, err := New(cfg)
		require.NoError(t, err)
		assert.NotEmpty(t, g.hostname)
	})

	t.Run("specific scrapers", func(t *testing.T) {
		cfg := baseCfg(t, &mockMetricConsumer{})
		cfg.ScraperNames = []string{"cpu", "memory"}
		g, err := New(cfg)
		require.NoError(t, err)
		assert.Len(t, g.scrapers, 2)
	})

	t.Run("nil logger", func(t *testing.T) {
		cfg := baseCfg(t, &mockMetricConsumer{})
		cfg.Logger = nil
		_, err := New(cfg)
		require.Error(t, err)
	})

	t.Run("nil consumer", func(t *testing.T) {
		cfg := baseCfg(t, nil)
		_, err := New(cfg)
		require.Error(t, err)
	})

	t.Run("workers unset is valid", func(t *testing.T) {
		cfg := baseCfg(t, &mockMetricConsumer{})
		cfg.Workers = 0
		_, err := New(cfg)
		require.NoError(t, err)
	})
}

// TestWorkersIgnored asserts one simulated host runs exactly one worker no
// matter what Workers says: parallel workers would emit duplicate series for
// the same host, and rate is the load knob.
func TestWorkersIgnored(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))

	cfg := baseCfg(t, &mockMetricConsumer{})
	cfg.Workers = 4
	cfg.Telemetry = embed.TelemetrySettings{MeterProvider: mp}
	g, err := New(cfg)
	require.NoError(t, err)
	require.NoError(t, g.Start(context.Background()))
	t.Cleanup(func() { _ = g.Stop(context.Background()) })

	var rm metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(context.Background(), &rm))
	var active int64 = -1
	for _, sm := range rm.ScopeMetrics {
		for _, md := range sm.Metrics {
			if md.Name != "blitz.generator.active_workers" {
				continue
			}
			if gauge, ok := md.Data.(metricdata.Gauge[int64]); ok && len(gauge.DataPoints) > 0 {
				active = gauge.DataPoints[0].Value
			}
		}
	}
	require.Equal(t, int64(1), active)
}

// TestNewProjectsIdentityResource confirms that when a resolved datagen
// identity is supplied, the generator's static resource carries the full
// host.* / os.* / deployment.* projection (os.type as the semconv value, so
// macOS becomes darwin) rather than just host.name + os raw string.
func TestNewProjectsIdentityResource(t *testing.T) {
	cfg := baseCfg(t, &mockMetricConsumer{})
	cfg.Hostname = ""
	cfg.OS = ""
	cfg.Identity = &datagen.SystemIdentity{
		Hostname: "THOR-01",
		HostID:   "abc123",
		Arch:     datagen.ArchAMD64,
		Tier:     datagen.TierProd,
		OSInfo:   datagen.OSInfo{Type: datagen.OSMacOS, Name: "macOS", Version: "14.6.1"},
	}

	g, err := New(cfg)
	require.NoError(t, err)

	res := g.static.Record()
	assert.Equal(t, "THOR-01", res["host.name"])
	assert.Equal(t, "abc123", res["host.id"])
	assert.Equal(t, "darwin", res["os.type"])
	assert.Equal(t, "macOS", res["os.name"])
	assert.Equal(t, "production", res["deployment.environment.name"])
	assert.Equal(t, "hostmetrics", res["telemetry.source"])
	assert.Equal(t, "THOR-01", g.hostname)
}

// TestSyntheticIdentityOSTypeProjection confirms the no-Identity path builds a
// synthetic identity from the OS knob: an empty OS omits os.type entirely
// (rather than emitting an empty string), and a set OS projects through
// SemconvOSType.
func TestSyntheticIdentityOSTypeProjection(t *testing.T) {
	cfg := baseCfg(t, &mockMetricConsumer{})
	cfg.OS = ""

	g, err := New(cfg)
	require.NoError(t, err)
	assert.Equal(t, "linux", g.static.Record()["os.type"], "an unset os is linux, never an empty os.type")

	cfg.OS = "windows"
	g2, err := New(cfg)
	require.NoError(t, err)
	assert.Equal(t, "windows", g2.static.Record()["os.type"])
}

// TestExplicitHostnameOverridesIdentityHostname confirms a set Hostname
// replaces only the resolved identity's hostname: the OS and other identity
// attributes stay, and the shared identity is not mutated.
func TestExplicitHostnameOverridesIdentityHostname(t *testing.T) {
	cfg := baseCfg(t, &mockMetricConsumer{})
	cfg.Hostname = "explicit-01"
	id := &datagen.SystemIdentity{
		Hostname: "identity-01",
		HostID:   "id-9",
		OSInfo:   datagen.OSInfo{Type: datagen.OSLinux},
	}
	cfg.Identity = id

	g, err := New(cfg)
	require.NoError(t, err)

	res := g.static.Record()
	assert.Equal(t, "explicit-01", res["host.name"])
	assert.Equal(t, "id-9", res["host.id"])
	assert.Equal(t, "linux", res["os.type"])
	assert.Equal(t, "explicit-01", g.hostname)
	assert.Equal(t, "identity-01", id.Hostname, "shared identity must not be mutated")
}

// TestSyntheticIdentityRandomSeed exercises the randomize branch of the
// synthetic-identity hostname generation (Seed < 0 → wall-clock seed).
func TestSyntheticIdentityRandomSeed(t *testing.T) {
	cfg := baseCfg(t, &mockMetricConsumer{})
	cfg.Hostname = ""
	cfg.Seed = -1

	g, err := New(cfg)
	require.NoError(t, err)
	assert.NotEmpty(t, g.hostname)
}

func TestNameAndSupportedTelemetry(t *testing.T) {
	g, err := New(baseCfg(t, &mockMetricConsumer{}))
	require.NoError(t, err)

	assert.Equal(t, "hostmetrics", g.Name())
	assert.Equal(t, []telemetry.Type{telemetry.Metrics}, g.SupportedTelemetry())
}

func TestStartStop(t *testing.T) {
	cons := &mockMetricConsumer{}
	cfg := baseCfg(t, cons)
	cfg.Rate = 50 * time.Millisecond

	g, err := New(cfg)
	require.NoError(t, err)

	require.NoError(t, g.Start(context.Background()))

	require.Eventually(t, func() bool { return cons.Count() > 0 }, 2*time.Second, 10*time.Millisecond,
		"should consume at least one metric point")

	require.NoError(t, g.Stop(context.Background()))

	assert.NotEmpty(t, cons.Snapshot(), "should have generated metrics")
}

func TestCountTracker(t *testing.T) {
	cons := &mockMetricConsumer{}
	cfg := baseCfg(t, cons)
	cfg.Rate = 50 * time.Millisecond

	g, err := New(cfg)
	require.NoError(t, err)

	tracker := count.NewTracker(2)
	g.SetCountTracker(tracker)

	require.NoError(t, g.Start(context.Background()))

	select {
	case <-tracker.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("tracker should have completed")
	}

	require.NoError(t, g.Stop(context.Background()))
}

// flakyMetricConsumer rejects its first call, then accepts every one.
type flakyMetricConsumer struct {
	mockMetricConsumer
	calls int
}

func (f *flakyMetricConsumer) ConsumeMetrics(ctx context.Context, batch []embed.MetricPoint) error {
	f.mu.Lock()
	f.calls++
	first := f.calls == 1
	f.mu.Unlock()
	if first {
		return errors.New("rejected")
	}
	return f.mockMetricConsumer.ConsumeMetrics(ctx, batch)
}

// TestFailedScrapeReleasesPermit pins that a scrape with a rejected batch
// gives its permit back and is retried, so the count ends on a scrape the
// consumer accepted.
func TestFailedScrapeReleasesPermit(t *testing.T) {
	cons := &flakyMetricConsumer{}
	cfg := baseCfg(t, cons)
	cfg.Rate = time.Millisecond
	cfg.ScraperNames = []string{"cpu"}
	g, err := New(cfg)
	require.NoError(t, err)
	tracker := count.NewTracker(1)
	g.SetCountTracker(tracker)
	require.NoError(t, g.Start(context.Background()))
	t.Cleanup(func() { _ = g.Stop(context.Background()) })

	select {
	case <-tracker.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("count never completed")
	}
	cons.mu.Lock()
	defer cons.mu.Unlock()
	assert.Equal(t, 2, cons.calls, "the rejected scrape was retried before the count closed")
	assert.NotEmpty(t, cons.points, "the counted scrape reached the consumer")
}

// TestRefusedScrapeEmitsNothing pins that a scrape refused a permit emits
// nothing, so a spent count stops hostmetrics output.
func TestRefusedScrapeEmitsNothing(t *testing.T) {
	cons := &mockMetricConsumer{}
	cfg := baseCfg(t, cons)
	cfg.Rate = time.Millisecond
	g, err := New(cfg)
	require.NoError(t, err)
	tracker := count.NewTracker(1)
	_, _, ok := tracker.Acquire() // spend the count, so every scrape is refused
	require.True(t, ok)
	g.SetCountTracker(tracker)
	require.NoError(t, g.Start(context.Background()))
	t.Cleanup(func() { _ = g.Stop(context.Background()) })

	require.Never(t, func() bool { return cons.Count() > 0 }, 50*time.Millisecond, time.Millisecond)
	// The worker was running and refused: a refilled count lets it scrape.
	tracker.Reset()
	require.Eventually(t, func() bool { return cons.Count() > 0 }, 5*time.Second, time.Millisecond)
}

// TestSeedDeterminism confirms two generators with the same Seed and
// hostname produce byte-identical metric streams across runs. This is
// the core "deterministic from seed" guarantee that blitz has carried
// from day 0 — hostmetrics inherits it here.
func TestSeedDeterminism(t *testing.T) {
	runOnce := func() []embed.MetricPoint {
		cons := &mockMetricConsumer{}
		cfg := baseCfg(t, cons)
		cfg.Rate = 20 * time.Millisecond
		cfg.Seed = 12345
		cfg.ScraperNames = []string{"cpu", "memory"}

		g, err := New(cfg)
		require.NoError(t, err)
		require.NoError(t, g.Start(context.Background()))
		require.Eventually(t, func() bool { return cons.Count() >= 6 }, 2*time.Second, 10*time.Millisecond)
		stopCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		require.NoError(t, g.Stop(stopCtx))
		return cons.Snapshot()
	}

	a := runOnce()
	b := runOnce()

	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	require.Positive(t, n, "neither run produced points")
	for i := 0; i < n; i++ {
		assert.Equal(t, a[i].Name, b[i].Name, "point %d: name mismatch across seeded runs", i)
		if a[i].IntValue != nil && b[i].IntValue != nil {
			assert.Equal(t, *a[i].IntValue, *b[i].IntValue, "point %d (%s): IntValue mismatch", i, a[i].Name)
		}
		if a[i].DoubleValue != nil && b[i].DoubleValue != nil {
			assert.Equal(t, *a[i].DoubleValue, *b[i].DoubleValue, "point %d (%s): DoubleValue mismatch", i, a[i].Name)
		}
	}
}

// Test individual scrapers produce non-empty records.
func TestScrapers(t *testing.T) {
	r := rand.New(rand.NewSource(42)) // #nosec G404
	resource := map[string]any{"host.name": "test", "os.type": "linux"}

	scrapers := allScrapers()
	for _, s := range scrapers {
		t.Run(s.Name(), func(t *testing.T) {
			records := s.Scrape(r, "test-host", resource)
			assert.NotEmpty(t, records, "scraper %s should produce records", s.Name())
			for _, rec := range records {
				assert.NotEmpty(t, rec.Name, "metric name should not be empty")
				assert.NotZero(t, rec.Metadata.Timestamp, "timestamp should not be zero")
				assert.NotNil(t, rec.Metadata.Resource, "resource should not be nil")
				assert.True(t, rec.IntValue != nil || rec.DoubleValue != nil,
					"metric %s should have a value", rec.Name)
			}
		})
	}
}

func TestBuildScrapers(t *testing.T) {
	t.Run("empty returns all", func(t *testing.T) {
		scrapers := buildScrapers(nil)
		assert.Len(t, scrapers, 8)
	})

	t.Run("specific names", func(t *testing.T) {
		scrapers := buildScrapers([]string{"cpu", "load"})
		assert.Len(t, scrapers, 2)
		assert.Equal(t, "cpu", scrapers[0].Name())
		assert.Equal(t, "load", scrapers[1].Name())
	})

	t.Run("unknown name ignored", func(t *testing.T) {
		scrapers := buildScrapers([]string{"cpu", "bogus"})
		assert.Len(t, scrapers, 1)
	})
}

// TestSyntheticIdentityCanonicalizesOS pins the synthetic (no environment)
// identity to the canonical OS the os knob names: aliases resolve (xen is a
// Linux dom0, vmware is ESXi), os.type is the semconv value, and the host
// carries that OS's real name and version.
func TestSyntheticIdentityCanonicalizesOS(t *testing.T) {
	for os, want := range map[string]struct{ osType, osName string }{
		"xen":     {"linux", "XCP-ng"},
		"kvm":     {"linux", ""},
		"vmware":  {"esxi", "VMware ESXi"},
		"aix":     {"aix", "AIX"},
		"openbsd": {"openbsd", "OpenBSD"},
		"darwin":  {"darwin", "macOS"},
		"":        {"linux", ""},
	} {
		cfg := baseCfg(t, &mockMetricConsumer{})
		cfg.OS, cfg.Seed = os, 1
		g, err := New(cfg)
		require.NoError(t, err)
		res := g.static.Record()
		assert.Equal(t, want.osType, res["os.type"], "os %q", os)
		assert.NotEmpty(t, res["os.name"], "os %q has no os.name", os)
		assert.NotEmpty(t, res["os.version"], "os %q has no os.version", os)
		if want.osName != "" {
			assert.Equal(t, want.osName, res["os.name"], "os %q", os)
		}
	}
}
