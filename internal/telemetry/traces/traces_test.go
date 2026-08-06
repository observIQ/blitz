package traces

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
)

// TestNewOTLP_buildsProviderAndShutsDown confirms the OTLP tracer provider
// constructs without a live collector (the exporter connects lazily) and shuts
// down cleanly. It does not assert export, which would require a collector.
func TestNewOTLP_buildsProviderAndShutsDown(t *testing.T) {
	o, err := NewOTLP(context.Background(), "localhost:4317", true)
	require.NoError(t, err)
	require.NotNil(t, o)
	require.NotNil(t, o.Provider())

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, o.Shutdown(ctx))
}

// TestNewOTLP_hostnameFallback covers the os.Hostname error path: the provider
// still builds, falling back to an "unknown" hostname resource attribute.
func TestNewOTLP_hostnameFallback(t *testing.T) {
	orig := osHostname
	osHostname = func() (string, error) { return "", errors.New("no hostname") }
	defer func() { osHostname = orig }()

	o, err := NewOTLP(context.Background(), "localhost:4317", true)
	require.NoError(t, err)
	require.NotNil(t, o)
	require.NoError(t, o.Shutdown(context.Background()))
}

// TestNewOTLP_exporterError covers the exporter-construction error path.
func TestNewOTLP_exporterError(t *testing.T) {
	orig := newTraceExporter
	newTraceExporter = func(context.Context, ...otlptracegrpc.Option) (*otlptrace.Exporter, error) {
		return nil, errors.New("boom")
	}
	defer func() { newTraceExporter = orig }()

	_, err := NewOTLP(context.Background(), "localhost:4317", true)
	require.Error(t, err)
	require.Contains(t, err.Error(), "create otlp trace exporter")
}

// TestShutdown_nilProvider confirms Shutdown is safe on a zero-value OTLP.
func TestShutdown_nilProvider(t *testing.T) {
	var o OTLP
	require.NoError(t, o.Shutdown(context.Background()))
}
