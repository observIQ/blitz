package generator

import (
	"context"

	"github.com/observiq/blitz/output"
	"github.com/observiq/blitz/telemetry"
)

// Compile-time interface assertion for TraceGenerator (the last
// surviving legacy interface — MetricGenerator was removed alongside
// the hostmetrics migration to embed.MetricConsumer in this PR).
var _ TraceGenerator = (*testTraceGen)(nil)

type testTraceGen struct{}

func (t *testTraceGen) Start(_ output.TraceWriter) error { return nil }
func (t *testTraceGen) Stop(_ context.Context) error     { return nil }
func (t *testTraceGen) SupportedTelemetry() []telemetry.Type {
	return []telemetry.Type{telemetry.Traces}
}
