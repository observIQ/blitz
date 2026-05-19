package hostmetrics

import (
	"context"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/observiq/blitz/generator/count"
	"github.com/observiq/blitz/output"
	"github.com/observiq/blitz/telemetry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

type mockMetricWriter struct {
	mu      sync.Mutex
	records []output.MetricRecord
}

func (m *mockMetricWriter) WriteMetric(_ context.Context, rec output.MetricRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.records = append(m.records, rec)
	return nil
}

func (m *mockMetricWriter) Records() []output.MetricRecord {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]output.MetricRecord, len(m.records))
	copy(result, m.records)
	return result
}

func TestNew(t *testing.T) {
	logger := zaptest.NewLogger(t)

	t.Run("valid", func(t *testing.T) {
		g, err := New(logger, 1, time.Second, "linux", "test-host", nil)
		require.NoError(t, err)
		assert.NotNil(t, g)
		assert.Equal(t, "test-host", g.hostname)
		assert.Len(t, g.scrapers, 8) // all scrapers
	})

	t.Run("auto hostname", func(t *testing.T) {
		g, err := New(logger, 1, time.Second, "linux", "", nil)
		require.NoError(t, err)
		assert.NotEmpty(t, g.hostname)
	})

	t.Run("windows hostname", func(t *testing.T) {
		g, err := New(logger, 1, time.Second, "windows", "", nil)
		require.NoError(t, err)
		// Windows hostnames are uppercase
		assert.NotEmpty(t, g.hostname)
	})

	t.Run("specific scrapers", func(t *testing.T) {
		g, err := New(logger, 1, time.Second, "linux", "host", []string{"cpu", "memory"})
		require.NoError(t, err)
		assert.Len(t, g.scrapers, 2)
	})

	t.Run("nil logger", func(t *testing.T) {
		_, err := New(nil, 1, time.Second, "linux", "host", nil)
		require.Error(t, err)
	})

	t.Run("invalid workers", func(t *testing.T) {
		_, err := New(logger, 0, time.Second, "linux", "host", nil)
		require.Error(t, err)
	})
}

func TestSupportedTelemetry(t *testing.T) {
	logger := zaptest.NewLogger(t)
	g, err := New(logger, 1, time.Second, "linux", "host", nil)
	require.NoError(t, err)

	types := g.SupportedTelemetry()
	assert.Equal(t, []telemetry.Type{telemetry.Metrics}, types)
}

func TestStartStop(t *testing.T) {
	logger := zaptest.NewLogger(t)
	writer := &mockMetricWriter{}

	g, err := New(logger, 1, 50*time.Millisecond, "linux", "test-host", nil)
	require.NoError(t, err)

	require.NoError(t, g.Start(writer))

	// Wait for at least one scrape
	time.Sleep(150 * time.Millisecond)

	require.NoError(t, g.Stop(context.Background()))

	records := writer.Records()
	assert.NotEmpty(t, records, "should have generated metrics")
}

func TestCountTracker(t *testing.T) {
	logger := zaptest.NewLogger(t)
	writer := &mockMetricWriter{}

	g, err := New(logger, 1, 50*time.Millisecond, "linux", "test-host", nil)
	require.NoError(t, err)

	tracker := count.NewTracker(2)
	g.SetCountTracker(tracker)

	require.NoError(t, g.Start(writer))

	// Wait for tracker to exhaust
	select {
	case <-tracker.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("tracker should have completed")
	}

	require.NoError(t, g.Stop(context.Background()))
}

// Test individual scrapers produce non-empty records
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
				assert.NotZero(t, rec.Timestamp, "timestamp should not be zero")
				assert.NotNil(t, rec.Resource, "resource should not be nil")
				// Each record should have either IntValue or DoubleValue set
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
