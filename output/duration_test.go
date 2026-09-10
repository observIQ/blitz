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

// TestRequestLatencyRecordsMillisAcrossBuckets asserts that a spread of real
// send durations, converted the way the output call sites convert them
// (output.DurationMillis), lands across multiple histogram buckets instead of
// collapsing into the first one — the defect PIPE-1404 fixed.
//
// Asserting the unit is "ms" is NOT sufficient: the unit string does not change
// the histogram's bucket boundaries, so a call site that recorded
// time.Since(start).Seconds() would keep unit="ms" while every value collapsed
// into le=5. The magnitude assertion below is what actually guards the scale.
// The call-site guard against a .Seconds() regression lives in the output
// packages (see output/tcp: TestSendDataRecordsLatencyInMillis).
func TestRequestLatencyRecordsMillisAcrossBuckets(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	m, err := output.NewMetrics(mp)
	require.NoError(t, err)

	// In seconds these would all be <= 0.6 and pile into le=5.
	for _, d := range []time.Duration{
		500 * time.Microsecond, // ~le=5
		8 * time.Millisecond,   // ~le=10
		30 * time.Millisecond,  // ~le=50
		120 * time.Millisecond, // ~le=250
		600 * time.Millisecond, // ~le=750
	} {
		m.BlitzOutputRequestLatencyHistogram.Record(context.Background(), output.DurationMillis(d), "test", "logs")
	}

	var rm metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(context.Background(), &rm))
	dp, unit, ok := histogramDataPoint(&rm, "blitz.output.request_latency")
	require.True(t, ok, "request_latency histogram not found")

	require.Equal(t, "ms", unit, "request_latency unit")
	require.GreaterOrEqualf(t, populatedBuckets(dp), 3, "expected >=3 populated buckets, got %d", populatedBuckets(dp))
	require.Greaterf(t, dp.Sum, 5.0, "values must be ms-scale; a .Seconds() regression would keep Sum well under 5 (got %v)", dp.Sum)
}

// histogramDataPoint returns the first data point and unit of the named
// float64 histogram from a manual-reader collection.
func histogramDataPoint(rm *metricdata.ResourceMetrics, name string) (metricdata.HistogramDataPoint[float64], string, bool) {
	for _, sm := range rm.ScopeMetrics {
		for _, mm := range sm.Metrics {
			if mm.Name != name {
				continue
			}
			if h, ok := mm.Data.(metricdata.Histogram[float64]); ok && len(h.DataPoints) > 0 {
				return h.DataPoints[0], mm.Unit, true
			}
		}
	}
	return metricdata.HistogramDataPoint[float64]{}, "", false
}

// populatedBuckets counts how many histogram buckets received at least one
// sample.
func populatedBuckets(dp metricdata.HistogramDataPoint[float64]) int {
	n := 0
	for _, c := range dp.BucketCounts {
		if c > 0 {
			n++
		}
	}
	return n
}
