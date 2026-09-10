package hec

import (
	"context"
	"testing"
	"time"

	"github.com/observiq/blitz/output"
	"github.com/stretchr/testify/require"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

// TestACKPollLatencyRecordsMillisAcrossBuckets asserts that a spread of real
// ACK-poll durations, converted the way ackPoller records them
// (output.DurationMillis), lands across multiple histogram buckets instead of
// collapsing into the first one — the defect PIPE-1404 fixed.
//
// Asserting the unit is "ms" alone is not sufficient: the unit string does not
// change the bucket boundaries, so a call site that recorded
// time.Since(start).Seconds() would keep unit="ms" while every value collapsed
// into le=5. The magnitude assertion below guards the scale.
func TestACKPollLatencyRecordsMillisAcrossBuckets(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	m, err := NewMetrics(mp)
	require.NoError(t, err)

	// In seconds these would all be <= 0.6 and pile into le=5.
	for _, d := range []time.Duration{
		500 * time.Microsecond,
		8 * time.Millisecond,
		30 * time.Millisecond,
		120 * time.Millisecond,
		600 * time.Millisecond,
	} {
		m.blitzOutputHecAckPollLatencyHistogram.Record(context.Background(), output.DurationMillis(d))
	}

	var rm metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(context.Background(), &rm))
	dp, unit, ok := ackPollHistogram(&rm)
	require.True(t, ok, "ack_poll_latency histogram not found")

	require.Equal(t, "ms", unit, "ack_poll_latency unit")
	require.GreaterOrEqualf(t, populatedBuckets(dp), 3, "expected >=3 populated buckets, got %d", populatedBuckets(dp))
	require.Greaterf(t, dp.Sum, 5.0, "values must be ms-scale; a .Seconds() regression would keep Sum well under 5 (got %v)", dp.Sum)
}

func ackPollHistogram(rm *metricdata.ResourceMetrics) (metricdata.HistogramDataPoint[float64], string, bool) {
	for _, sm := range rm.ScopeMetrics {
		for _, mm := range sm.Metrics {
			if mm.Name != "blitz.output.hec.ack_poll_latency" {
				continue
			}
			if h, ok := mm.Data.(metricdata.Histogram[float64]); ok && len(h.DataPoints) > 0 {
				return h.DataPoints[0], mm.Unit, true
			}
		}
	}
	return metricdata.HistogramDataPoint[float64]{}, "", false
}

func populatedBuckets(dp metricdata.HistogramDataPoint[float64]) int {
	n := 0
	for _, c := range dp.BucketCounts {
		if c > 0 {
			n++
		}
	}
	return n
}
