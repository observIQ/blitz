package metrics

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/observiq/blitz/output"
	"github.com/observiq/blitz/telemetry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

type mockWriter struct {
	mu      sync.Mutex
	metrics []output.MetricRecord
}

func (m *mockWriter) Write(_ context.Context, _ output.LogRecord) error {
	return output.ErrUnsupportedTelemetryType
}

func (m *mockWriter) WriteMetric(_ context.Context, data output.MetricRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.metrics = append(m.metrics, data)
	return nil
}

func (m *mockWriter) getMetrics() []output.MetricRecord {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]output.MetricRecord(nil), m.metrics...)
}

func sampleDefs() []MetricDefinition {
	return []MetricDefinition{
		{
			Name:        "system.cpu.utilization",
			Type:        output.MetricTypeGauge,
			Description: "CPU utilization",
			Unit:        "1",
			Attributes:  map[string]string{"host.name": "test-host"},
			ValueMin:    0,
			ValueMax:    100,
		},
	}
}

func TestNew(t *testing.T) {
	logger := zaptest.NewLogger(t)

	t.Run("success", func(t *testing.T) {
		g, err := New(logger, 1, time.Second, nil, sampleDefs())
		require.NoError(t, err)
		assert.NotNil(t, g)
	})

	t.Run("nil logger", func(t *testing.T) {
		_, err := New(nil, 1, time.Second, nil, sampleDefs())
		require.Error(t, err)
	})

	t.Run("zero workers", func(t *testing.T) {
		_, err := New(logger, 0, time.Second, nil, sampleDefs())
		require.Error(t, err)
	})

	t.Run("no metric definitions", func(t *testing.T) {
		_, err := New(logger, 1, time.Second, nil, nil)
		require.Error(t, err)
	})
}

func TestSupportedTelemetry(t *testing.T) {
	logger := zaptest.NewLogger(t)
	g, err := New(logger, 1, time.Second, nil, sampleDefs())
	require.NoError(t, err)
	assert.Equal(t, []telemetry.Type{telemetry.Metrics}, g.SupportedTelemetry())
}

func TestStartStop(t *testing.T) {
	logger := zaptest.NewLogger(t)
	writer := &mockWriter{}

	g, err := New(logger, 2, 50*time.Millisecond, nil, sampleDefs())
	require.NoError(t, err)

	require.NoError(t, g.Start(writer))

	time.Sleep(300 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, g.Stop(ctx))

	metrics := writer.getMetrics()
	assert.Greater(t, len(metrics), 0, "Expected at least one metric")

	m := metrics[0]
	assert.Equal(t, "system.cpu.utilization", m.Name)
	assert.Equal(t, output.MetricTypeGauge, m.Type)
	assert.Equal(t, "CPU utilization", m.Description)
	assert.Equal(t, "1", m.Unit)
	assert.NotNil(t, m.DoubleValue)
	assert.GreaterOrEqual(t, *m.DoubleValue, 0.0)
	assert.LessOrEqual(t, *m.DoubleValue, 100.0)
	assert.False(t, m.Timestamp.IsZero())
}

func TestResourceAttributeCombinations(t *testing.T) {
	logger := zaptest.NewLogger(t)
	writer := &mockWriter{}

	resAttrs := map[string][]string{
		"service.name": {"svc-a", "svc-b"},
	}

	defs := []MetricDefinition{
		{
			Name:     "cpu",
			Type:     output.MetricTypeGauge,
			ValueMin: 0,
			ValueMax: 1,
		},
	}

	g, err := New(logger, 1, 50*time.Millisecond, resAttrs, defs)
	require.NoError(t, err)

	require.NoError(t, g.Start(writer))
	time.Sleep(200 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, g.Stop(ctx))

	metrics := writer.getMetrics()
	// Each tick produces 2 data points (one per resource attr value).
	resValues := make(map[string]bool)
	for _, m := range metrics {
		if v, ok := m.ResourceAttributes["service.name"]; ok {
			resValues[v] = true
		}
	}
	assert.True(t, resValues["svc-a"], "Expected svc-a")
	assert.True(t, resValues["svc-b"], "Expected svc-b")
}

func TestMultipleMetricDefinitions(t *testing.T) {
	logger := zaptest.NewLogger(t)
	writer := &mockWriter{}

	defs := []MetricDefinition{
		{
			Name:     "system.cpu.utilization",
			Type:     output.MetricTypeGauge,
			ValueMin: 0,
			ValueMax: 100,
		},
		{
			Name:     "system.memory.usage",
			Type:     output.MetricTypeGauge,
			Unit:     "By",
			ValueMin: 1000,
			ValueMax: 8000,
		},
		{
			Name:     "http.server.request.count",
			Type:     output.MetricTypeSum,
			ValueMin: 1,
			ValueMax: 50,
		},
	}

	g, err := New(logger, 1, 50*time.Millisecond, nil, defs)
	require.NoError(t, err)

	require.NoError(t, g.Start(writer))
	time.Sleep(200 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, g.Stop(ctx))

	metrics := writer.getMetrics()
	assert.GreaterOrEqual(t, len(metrics), 3)

	names := make(map[string]bool)
	for _, m := range metrics {
		names[m.Name] = true
	}
	assert.True(t, names["system.cpu.utilization"])
	assert.True(t, names["system.memory.usage"])
	assert.True(t, names["http.server.request.count"])
}

func TestCartesianProduct(t *testing.T) {
	t.Run("nil input", func(t *testing.T) {
		result := cartesianProduct(nil)
		assert.Len(t, result, 1)
		assert.Empty(t, result[0])
	})

	t.Run("single key single value", func(t *testing.T) {
		result := cartesianProduct(map[string][]string{"a": {"1"}})
		assert.Len(t, result, 1)
		assert.Equal(t, "1", result[0]["a"])
	})

	t.Run("single key multiple values", func(t *testing.T) {
		result := cartesianProduct(map[string][]string{"a": {"1", "2", "3"}})
		assert.Len(t, result, 3)
	})

	t.Run("two keys", func(t *testing.T) {
		result := cartesianProduct(map[string][]string{
			"a": {"1", "2"},
			"b": {"x", "y"},
		})
		assert.Len(t, result, 4)

		combos := make(map[string]bool)
		for _, m := range result {
			combos[m["a"]+"-"+m["b"]] = true
		}
		assert.True(t, combos["1-x"])
		assert.True(t, combos["1-y"])
		assert.True(t, combos["2-x"])
		assert.True(t, combos["2-y"])
	})
}
