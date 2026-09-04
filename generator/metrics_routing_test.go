package generator

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

// TestNewMetrics_recordsToProvidedProvider is the core contract of the
// self-telemetry refactor: instruments built by NewMetrics record to the
// caller-supplied MeterProvider, not the process global.
func TestNewMetrics_recordsToProvidedProvider(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))

	m, err := NewMetrics(mp)
	require.NoError(t, err)

	m.BlitzGeneratorEntriesCounter.Add(context.Background(), 5, "json")

	var rm metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(context.Background(), &rm))

	var got int64
	found := false
	for _, sm := range rm.ScopeMetrics {
		for _, md := range sm.Metrics {
			if md.Name == "blitz.generator.entries" {
				sum, ok := md.Data.(metricdata.Sum[int64])
				require.True(t, ok, "blitz.generator.entries should be an int64 sum")
				require.Len(t, sum.DataPoints, 1)
				got = sum.DataPoints[0].Value
				found = true
			}
		}
	}
	require.True(t, found, "expected blitz.generator.entries in the provided provider's output")
	require.Equal(t, int64(5), got)
}

// TestNewMetrics_nilProviderFallsBackToGlobal documents that a nil provider is
// safe and uses the process global, preserving standalone behavior.
func TestNewMetrics_nilProviderFallsBackToGlobal(t *testing.T) {
	m, err := NewMetrics(nil)
	require.NoError(t, err)
	require.NotNil(t, m)

	// Recording must not panic against the global provider.
	require.NotPanics(t, func() {
		m.BlitzGeneratorEntriesCounter.Add(context.Background(), 1, "json")
	})
}
