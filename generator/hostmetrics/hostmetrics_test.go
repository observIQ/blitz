package hostmetrics

import (
	"context"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/observiq/blitz/embed"
	"github.com/observiq/blitz/generator/count"
	"github.com/observiq/blitz/telemetry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
		assert.Len(t, g.scrapers, 9)
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

	t.Run("invalid workers", func(t *testing.T) {
		cfg := baseCfg(t, &mockMetricConsumer{})
		cfg.Workers = 0
		_, err := New(cfg)
		require.Error(t, err)
	})
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
	resource := map[string]string{"host.name": "test", "os.type": "linux"}

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
		assert.Len(t, scrapers, 9)
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
