// Package logs provides an OTLP gRPC exporter and LoggerProvider for blitz's
// own self-telemetry logs, plus a zap bridge that tees blitz's internal zap
// logging into an OTel LoggerProvider. It mirrors the trace setup in
// internal/telemetry/traces: a thin wrapper around the OTel SDK that the
// standalone CLI wires up when log export is configured.
package logs

import (
	"context"
	"fmt"
	"os"

	"go.opentelemetry.io/contrib/bridges/otelzap"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/log"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const (
	serviceName = "blitz"
	// scopeName is the instrumentation scope for blitz's internal logs,
	// matching the module path used by the traces self-telemetry.
	scopeName = "github.com/observiq/blitz"
)

// osHostname and newLogExporter are indirections over os.Hostname and the OTLP
// log exporter constructor, so tests can exercise the hostname-fallback and
// exporter-error paths deterministically.
var (
	osHostname     = os.Hostname
	newLogExporter = otlploggrpc.New
)

// OTLP owns an OTLP gRPC log exporter and the LoggerProvider built on it.
type OTLP struct {
	provider *sdklog.LoggerProvider
}

// NewOTLP builds a batching LoggerProvider that exports blitz's self-telemetry
// logs over OTLP gRPC to endpoint (host:port). insecure sends over plaintext.
// The exporter connects lazily, so a nil error does not imply the collector is
// reachable. Unlike the trace provider, this is not installed as a process
// global: there is no global LoggerProvider to set, so the returned provider is
// threaded explicitly via BridgeZap.
func NewOTLP(ctx context.Context, endpoint string, insecure bool) (*OTLP, error) {
	hostname, err := osHostname()
	if err != nil {
		hostname = "unknown"
	}
	res := resource.NewWithAttributes(semconv.SchemaURL,
		semconv.ServiceNameKey.String(serviceName),
		semconv.HostNameKey.String(hostname),
	)

	opts := []otlploggrpc.Option{otlploggrpc.WithEndpoint(endpoint)}
	if insecure {
		opts = append(opts, otlploggrpc.WithInsecure())
	}
	exporter, err := newLogExporter(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("create otlp log exporter: %w", err)
	}

	provider := sdklog.NewLoggerProvider(
		sdklog.WithProcessor(sdklog.NewBatchProcessor(exporter)),
		sdklog.WithResource(res),
	)

	return &OTLP{provider: provider}, nil
}

// Provider returns the LoggerProvider for wiring into a TelemetrySettings and
// bridging via BridgeZap.
func (o *OTLP) Provider() log.LoggerProvider { return o.provider }

// Shutdown flushes buffered log records and stops the exporter.
func (o *OTLP) Shutdown(ctx context.Context) error {
	if o.provider != nil {
		return o.provider.Shutdown(ctx)
	}
	return nil
}

// BridgeZap tees logger into lp so blitz's internal zap logs are also emitted as
// OTel log records. A nil lp returns logger unchanged (zap only, today's
// behavior). The returned logger writes to both the original zap core and an
// otelzap core backed by lp.
func BridgeZap(logger *zap.Logger, lp log.LoggerProvider) *zap.Logger {
	if lp == nil {
		return logger
	}
	otelCore := otelzap.NewCore(scopeName, otelzap.WithLoggerProvider(lp))
	return logger.WithOptions(zap.WrapCore(func(existing zapcore.Core) zapcore.Core {
		return zapcore.NewTee(existing, otelCore)
	}))
}
