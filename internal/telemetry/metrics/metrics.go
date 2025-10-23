// Package metrics provides a Prometheus exporter
// for serving metrics.
package metrics

import (
	"context"
	"fmt"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/prometheus"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
)

// TODO(jsirianni): These could be configurable in the future.
const (
	serviceName = "blitz"
)

// Prometheus is an OpenTelemetry Prometheus exporter.
type Prometheus struct {
	resources *resource.Resource
	provider  *sdkmetric.MeterProvider
}

// NewPrometheus creates a new Prometheus provider.
func NewPrometheus() (*Prometheus, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return nil, fmt.Errorf("get hostname: %w", err)
	}

	r := []attribute.KeyValue{
		semconv.ServiceNameKey.String(serviceName),
		semconv.HostNameKey.String(hostname),
	}

	return &Prometheus{
		resources: resource.NewWithAttributes(semconv.SchemaURL, r...),
	}, nil
}

// Start starts the Prometheus exporter
func (p *Prometheus) Start(_ context.Context) error {
	exporter, err := prometheus.New(prometheus.WithNamespace(serviceName))
	if err != nil {
		return err
	}

	p.provider = sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(exporter),
		sdkmetric.WithResource(p.resources),
	)

	otel.SetMeterProvider(p.provider)

	return nil
}

// Shutdown stops the Prometheus exporter
func (p *Prometheus) Shutdown(ctx context.Context) error {
	if p.provider != nil {
		return p.provider.Shutdown(ctx)
	}
	return nil
}
