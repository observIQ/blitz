package logs

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/log/logtest"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// TestNewOTLP_buildsProviderAndShutsDown confirms the OTLP logger provider
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

// discardLogger returns a real zap logger at debug level that writes nowhere,
// so the tee has a live source core without polluting test output.
func discardLogger() *zap.Logger {
	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig()),
		zapcore.AddSync(io.Discard),
		zapcore.DebugLevel,
	)
	return zap.New(core)
}

// TestBridgeZap_teesRecordsToLoggerProvider confirms a bridged logger emits an
// OTel log record to the supplied LoggerProvider while still logging via zap.
func TestBridgeZap_teesRecordsToLoggerProvider(t *testing.T) {
	rec := logtest.NewRecorder()

	logger := BridgeZap(discardLogger(), rec)
	logger.Info("hello from blitz")

	got := rec.Result()
	require.NotEmpty(t, got, "expected at least one recorded scope")

	var bodies []string
	for scope, records := range got {
		require.Equal(t, scopeName, scope.Name)
		for _, r := range records {
			bodies = append(bodies, r.Body.AsString())
		}
	}
	require.Contains(t, bodies, "hello from blitz")
}

// TestBridgeZap_nilProviderReturnsLoggerUnchanged confirms a nil LoggerProvider
// leaves the zap logger untouched (zap-only, today's behavior).
func TestBridgeZap_nilProviderReturnsLoggerUnchanged(t *testing.T) {
	in := zap.NewNop()
	out := BridgeZap(in, nil)
	require.Same(t, in, out)
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
	orig := newLogExporter
	newLogExporter = func(context.Context, ...otlploggrpc.Option) (*otlploggrpc.Exporter, error) {
		return nil, errors.New("boom")
	}
	defer func() { newLogExporter = orig }()

	_, err := NewOTLP(context.Background(), "localhost:4317", true)
	require.Error(t, err)
	require.Contains(t, err.Error(), "create otlp log exporter")
}

// TestShutdown_nilProvider confirms Shutdown is safe on a zero-value OTLP.
func TestShutdown_nilProvider(t *testing.T) {
	var o OTLP
	require.NoError(t, o.Shutdown(context.Background()))
}
