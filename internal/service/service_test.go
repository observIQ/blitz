package service

import (
	"context"
	"testing"

	"github.com/observiq/blitz/embed"
	"github.com/observiq/blitz/output"
	"github.com/observiq/blitz/telemetry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// stubLogGen is a ProducerModule that records whether Start was called.
// Mirrors the shape every log generator package uses (apache, json,
// nginx, etc.) after the embed-seam migration: Name + Start(ctx) +
// Stop(ctx), with embed.ProducerMarker satisfying the marker method.
type stubLogGen struct {
	embed.ProducerMarker
	started bool
}

func (s *stubLogGen) Name() string                         { return "stub-log" }
func (s *stubLogGen) Start(_ context.Context) error        { s.started = true; return nil }
func (s *stubLogGen) Stop(_ context.Context) error         { return nil }
func (s *stubLogGen) SupportedTelemetry() []telemetry.Type { return []telemetry.Type{telemetry.Logs} }

type stubMetricGen struct{ started bool }

func (s *stubMetricGen) Start(_ output.MetricWriter) error { s.started = true; return nil }
func (s *stubMetricGen) Stop(_ context.Context) error      { return nil }
func (s *stubMetricGen) SupportedTelemetry() []telemetry.Type {
	return []telemetry.Type{telemetry.Metrics}
}

type stubTraceGen struct{ started bool }

func (s *stubTraceGen) Start(_ output.TraceWriter) error { s.started = true; return nil }
func (s *stubTraceGen) Stop(_ context.Context) error     { return nil }
func (s *stubTraceGen) SupportedTelemetry() []telemetry.Type {
	return []telemetry.Type{telemetry.Traces}
}

type stubOutput struct{}

func (s *stubOutput) Write(_ context.Context, _ output.LogRecord) error          { return nil }
func (s *stubOutput) WriteMetric(_ context.Context, _ output.MetricRecord) error { return nil }
func (s *stubOutput) WriteTrace(_ context.Context, _ output.TraceRecord) error   { return nil }
func (s *stubOutput) Stop(_ context.Context) error                               { return nil }
func (s *stubOutput) SupportedTelemetry() []telemetry.Type {
	return []telemetry.Type{telemetry.Logs, telemetry.Metrics, telemetry.Traces}
}

type stubLogsOnlyOutput struct{}

func (s *stubLogsOnlyOutput) Write(_ context.Context, _ output.LogRecord) error { return nil }
func (s *stubLogsOnlyOutput) Stop(_ context.Context) error                      { return nil }
func (s *stubLogsOnlyOutput) SupportedTelemetry() []telemetry.Type {
	return []telemetry.Type{telemetry.Logs}
}

func TestNew(t *testing.T) {
	logger := zaptest.NewLogger(t)

	t.Run("nil logger", func(t *testing.T) {
		_, err := New(nil, []any{&stubLogGen{}}, &stubOutput{})
		require.Error(t, err)
	})

	t.Run("nil generators", func(t *testing.T) {
		_, err := New(logger, nil, &stubOutput{})
		require.Error(t, err)
	})

	t.Run("empty generators", func(t *testing.T) {
		_, err := New(logger, []any{}, &stubOutput{})
		require.Error(t, err)
	})

	t.Run("nil output", func(t *testing.T) {
		_, err := New(logger, []any{&stubLogGen{}}, nil)
		require.Error(t, err)
	})

	t.Run("valid single log generator", func(t *testing.T) {
		svc, err := New(logger, []any{&stubLogGen{}}, &stubOutput{})
		require.NoError(t, err)
		assert.NotNil(t, svc)
	})

	t.Run("valid multi generator", func(t *testing.T) {
		gens := []any{&stubLogGen{}, &stubMetricGen{}, &stubTraceGen{}}
		svc, err := New(logger, gens, &stubOutput{})
		require.NoError(t, err)
		assert.NotNil(t, svc)
	})
}

func TestStartStop(t *testing.T) {
	logger := zaptest.NewLogger(t)

	logGen := &stubLogGen{}
	metricGen := &stubMetricGen{}
	traceGen := &stubTraceGen{}

	svc, err := New(logger, []any{logGen, metricGen, traceGen}, &stubOutput{})
	require.NoError(t, err)

	require.NoError(t, svc.Start())
	assert.True(t, logGen.started)
	assert.True(t, metricGen.started)
	assert.True(t, traceGen.started)

	require.NoError(t, svc.Stop())
}

func TestStart_MetricGenWithLogsOnlyOutput(t *testing.T) {
	// MetricGenerator paired with logs-only output should skip (not error)
	logger := zaptest.NewLogger(t)

	metricGen := &stubMetricGen{}
	svc, err := New(logger, []any{metricGen}, &stubLogsOnlyOutput{})
	require.NoError(t, err)

	require.NoError(t, svc.Start())
	assert.False(t, metricGen.started, "metric gen should be skipped when output doesn't support metrics")

	require.NoError(t, svc.Stop())
}
