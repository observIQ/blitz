package embed

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestNopTelemetry(t *testing.T) {
	tel := NopTelemetry()

	// Every provider and the logger must be non-nil so callers can use the
	// bundle without nil checks.
	require.NotNil(t, tel.Logger)
	require.NotNil(t, tel.MeterProvider)
	require.NotNil(t, tel.TracerProvider)

	// Per-batch spans default off.
	require.False(t, tel.PerBatchSpans)
}

func TestTelemetrySettings_zeroValueFieldsAreNil(t *testing.T) {
	// A zero-value bundle leaves providers nil; components and NewMetrics are
	// responsible for the nil->global fallback. This documents that contract.
	var tel TelemetrySettings

	require.Nil(t, tel.Logger)
	require.Nil(t, tel.MeterProvider)
	require.Nil(t, tel.TracerProvider)
}

func TestTelemetrySettings_Tracer_usesProvidedProvider(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	tel := TelemetrySettings{TracerProvider: tp}

	_, span := tel.Tracer("test").Start(context.Background(), "op")
	span.End()

	spans := exporter.GetSpans()
	require.Len(t, spans, 1)
	require.Equal(t, "op", spans[0].Name)
}

func TestTelemetrySettings_Tracer_nilFallsBackToGlobal(t *testing.T) {
	var tel TelemetrySettings

	require.NotPanics(t, func() {
		_, span := tel.Tracer("test").Start(context.Background(), "op")
		span.End()
	})
}
