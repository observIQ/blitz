package nop

import (
	"context"
	"errors"
	"testing"

	"github.com/observiq/blitz/embed"
	"github.com/observiq/blitz/output"
	"github.com/observiq/blitz/telemetry"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/metric"
	metricnoop "go.opentelemetry.io/otel/metric/noop"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.uber.org/zap"
)

func TestNew_NilLogger(t *testing.T) {
	_, err := New(nil, embed.NopTelemetry())
	require.Error(t, err)
}

func TestSupportedTelemetry(t *testing.T) {
	o, err := New(zap.NewNop(), embed.NopTelemetry())
	require.NoError(t, err)
	require.Equal(t, []telemetry.Type{telemetry.Logs}, o.SupportedTelemetry())
}

// failingMeter overrides the first instrument the output registry builds
// (an Int64Gauge) to return an error, so output.NewMetrics fails.
type failingMeter struct {
	metric.Meter
}

func (failingMeter) Int64Gauge(string, ...metric.Int64GaugeOption) (metric.Int64Gauge, error) {
	return nil, errors.New("instrument error")
}

type failingMeterProvider struct {
	metric.MeterProvider
}

func (failingMeterProvider) Meter(string, ...metric.MeterOption) metric.Meter {
	return failingMeter{Meter: metricnoop.NewMeterProvider().Meter("test")}
}

func TestNew_MetricsError(t *testing.T) {
	_, err := New(zap.NewNop(), embed.TelemetrySettings{MeterProvider: failingMeterProvider{}})
	require.Error(t, err)
	require.Contains(t, err.Error(), "build output metrics")
}

// TestWrite_countsAndDiscards confirms nop counts every record through the
// injected MeterProvider while discarding the data.
func TestWrite_countsAndDiscards(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))

	o, err := New(zap.NewNop(), embed.TelemetrySettings{MeterProvider: mp})
	require.NoError(t, err)

	require.NoError(t, o.Write(context.Background(), output.LogRecord{Message: "discarded"}))
	require.NoError(t, o.Stop(context.Background()))

	var rm metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(context.Background(), &rm))

	found := false
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name == "blitz.output.entries_received" {
				found = true
			}
		}
	}
	require.True(t, found, "expected blitz.output.entries_received counter")
}
