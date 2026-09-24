package promscrape

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/observiq/blitz/embed"
	"github.com/observiq/blitz/internal/prommap"
	"github.com/observiq/blitz/output"
	"github.com/observiq/blitz/telemetry"
	"go.uber.org/zap"
)

// Output is the prometheus-scrape output: it upserts mapped metric series into
// an in-memory registry and serves them as Prometheus text exposition on an
// HTTP endpoint that a Prometheus server or the collector prometheusreceiver
// scrapes. Metrics-only.
type Output struct {
	emitTimestamps bool
	reg            *registry
	srv            *http.Server
	ln             net.Listener
	logger         *zap.Logger
	metrics        *promscrapeMetrics
}

// New builds a prometheus-scrape output and starts serving. It binds the
// listener eagerly so a port conflict surfaces here rather than at first
// scrape. metricsPath is the path the exposition is served on (e.g. /metrics).
func New(listenAddr, metricsPath string, emitTimestamps bool, tel embed.TelemetrySettings, logger *zap.Logger) (*Output, error) {
	if logger == nil {
		logger = zap.NewNop()
	}
	if metricsPath == "" {
		metricsPath = "/metrics"
	}

	ln, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return nil, fmt.Errorf("promscrape: listen on %s: %w", listenAddr, err)
	}

	o := &Output{
		emitTimestamps: emitTimestamps,
		reg:            newRegistry(),
		ln:             ln,
		logger:         logger,
	}

	// Self-telemetry is best-effort: a build failure disables recording rather
	// than the output.
	if m, err := newPromscrapeMetrics(tel); err != nil {
		logger.Warn("prometheus-scrape metrics disabled", zap.Error(err))
	} else {
		o.metrics = m
	}

	mux := http.NewServeMux()
	mux.HandleFunc(metricsPath, o.handleScrape)
	o.srv = &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}

	go func() {
		if err := o.srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			logger.Warn("prometheus-scrape server stopped", zap.Error(err))
		}
	}()

	return o, nil
}

// handleScrape renders the current registry snapshot as text exposition.
func (o *Output) handleScrape(w http.ResponseWriter, _ *http.Request) {
	start := time.Now()
	body := encode(o.reg.snapshot(), o.emitTimestamps)
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	n, _ := w.Write(body)
	if o.metrics != nil {
		o.metrics.recordScrape(context.Background(), int64(o.reg.len()), int64(n), float64(time.Since(start).Milliseconds()))
	}
}

// Write reports that logs are unsupported: this output is metrics-only.
func (o *Output) Write(_ context.Context, _ output.LogRecord) error {
	return output.ErrUnsupportedTelemetryType
}

// WriteMetric maps a metric record and upserts its series into the registry.
func (o *Output) WriteMetric(ctx context.Context, data output.MetricRecord) error {
	fam, err := prommap.Map(data)
	if err != nil {
		return fmt.Errorf("promscrape: map metric: %w", err)
	}
	o.reg.upsert(fam)
	if o.metrics != nil {
		o.metrics.recordSeriesReceived(ctx, int64(len(fam.Samples)))
	}
	return nil
}

// SupportedTelemetry reports that only metrics are consumed.
func (o *Output) SupportedTelemetry() []telemetry.Type {
	return []telemetry.Type{telemetry.Metrics}
}

// Stop shuts down the HTTP server.
func (o *Output) Stop(ctx context.Context) error {
	return o.srv.Shutdown(ctx)
}

// Addr returns the actual listen address (useful when the port was ephemeral).
func (o *Output) Addr() string {
	return o.ln.Addr().String()
}
