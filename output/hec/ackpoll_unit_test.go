package hec

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

// TestACKPollLatencyUnitIsMillis confirms the HEC ACK-poll latency histogram is
// registered in milliseconds so recorded values spread across the default
// millisecond-scale buckets rather than collapsing into the first one.
func TestACKPollLatencyUnitIsMillis(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	m, err := NewMetrics(mp)
	require.NoError(t, err)

	m.blitzOutputHecAckPollLatencyHistogram.Record(context.Background(), 1)

	var rm metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(context.Background(), &rm))
	unit := ""
	for _, sm := range rm.ScopeMetrics {
		for _, mm := range sm.Metrics {
			if mm.Name == "blitz.output.hec.ack_poll_latency" {
				unit = mm.Unit
			}
		}
	}
	require.Equal(t, "ms", unit, "ack_poll_latency unit")
}
