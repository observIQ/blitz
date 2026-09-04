// Package traces provides an OTLP gRPC exporter and TracerProvider for
// blitz's own self-telemetry spans. It mirrors the Prometheus metrics
// setup in internal/telemetry/metrics: a thin wrapper around the OTel SDK
// that the standalone CLI wires up when trace export is configured.
package traces

import (
	"context"
	"fmt"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"go.opentelemetry.io/otel/trace"
)

const serviceName = "blitz"

// osHostname and newTraceExporter are indirections over os.Hostname and the
// OTLP trace exporter constructor, so tests can exercise the hostname-fallback
// and exporter-error paths deterministically.
var (
	osHostname       = os.Hostname
	newTraceExporter = otlptracegrpc.New
)

// OTLP owns an OTLP gRPC trace exporter and the TracerProvider built on it.
type OTLP struct {
	provider *sdktrace.TracerProvider
}

// NewOTLP builds a batching TracerProvider that exports blitz's self-telemetry
// spans over OTLP gRPC to endpoint (host:port). insecure sends over plaintext.
// It also installs the provider as the process global so any otel.Tracer user
// picks it up. The exporter connects lazily, so a nil error does not imply the
// collector is reachable.
func NewOTLP(ctx context.Context, endpoint string, insecure bool) (*OTLP, error) {
	hostname, err := osHostname()
	if err != nil {
		hostname = "unknown"
	}
	res := resource.NewWithAttributes(semconv.SchemaURL,
		semconv.ServiceNameKey.String(serviceName),
		semconv.HostNameKey.String(hostname),
	)

	opts := []otlptracegrpc.Option{otlptracegrpc.WithEndpoint(endpoint)}
	if insecure {
		opts = append(opts, otlptracegrpc.WithInsecure())
	}
	exporter, err := newTraceExporter(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("create otlp trace exporter: %w", err)
	}

	provider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(provider)

	return &OTLP{provider: provider}, nil
}

// Provider returns the TracerProvider for wiring into a TelemetrySettings.
func (o *OTLP) Provider() trace.TracerProvider { return o.provider }

// Shutdown flushes buffered spans and stops the exporter.
func (o *OTLP) Shutdown(ctx context.Context) error {
	if o.provider != nil {
		return o.provider.Shutdown(ctx)
	}
	return nil
}
