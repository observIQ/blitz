package output_test

import (
	"context"
	"testing"
	"time"

	"github.com/observiq/blitz/output"
	"github.com/stretchr/testify/require"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

func TestDurationMillis(t *testing.T) {
	cases := []struct {
		name string
		in   time.Duration
		want float64
	}{
		{"whole milliseconds", 250 * time.Millisecond, 250},
		{"seconds scale", 2 * time.Second, 2000},
		{"sub-millisecond preserved", 500 * time.Microsecond, 0.5},
		{"zero", 0, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.InDelta(t, tc.want, output.DurationMillis(tc.in), 1e-9)
		})
	}
}

// TestRequestLatencyUnitIsMillis confirms the request-latency histogram is
// registered in milliseconds so recorded values land across the default
// millisecond-scale buckets rather than collapsing into the first one.
func TestRequestLatencyUnitIsMillis(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	m, err := output.NewMetrics(mp)
	require.NoError(t, err)

	m.BlitzOutputRequestLatencyHistogram.Record(context.Background(), 1, "test", "logs")

	var rm metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(context.Background(), &rm))
	unit := ""
	for _, sm := range rm.ScopeMetrics {
		for _, mm := range sm.Metrics {
			if mm.Name == "blitz.output.request_latency" {
				unit = mm.Unit
			}
		}
	}
	require.Equal(t, "ms", unit, "request_latency unit")
}
