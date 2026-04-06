package generator

import (
	"context"

	"github.com/observiq/blitz/output"
	"github.com/observiq/blitz/telemetry"
)

// Compile-time interface assertions for MetricGenerator
var _ MetricGenerator = (*testMetricGen)(nil)

type testMetricGen struct{}

func (t *testMetricGen) Start(_ output.MetricWriter) error { return nil }
func (t *testMetricGen) Stop(_ context.Context) error      { return nil }
func (t *testMetricGen) SupportedTelemetry() []telemetry.Type {
	return []telemetry.Type{telemetry.Metrics}
}

// Compile-time interface assertions for TraceGenerator
var _ TraceGenerator = (*testTraceGen)(nil)

type testTraceGen struct{}

func (t *testTraceGen) Start(_ output.TraceWriter) error { return nil }
func (t *testTraceGen) Stop(_ context.Context) error     { return nil }
func (t *testTraceGen) SupportedTelemetry() []telemetry.Type {
	return []telemetry.Type{telemetry.Traces}
}
