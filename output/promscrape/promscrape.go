package promscrape

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/observiq/blitz/embed"
	"github.com/observiq/blitz/internal/prommap"
	"github.com/observiq/blitz/output"
	"github.com/observiq/blitz/telemetry"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

// Output is the prometheus-scrape output: it keeps the latest point per
// series and serves them as the collector prometheusexporter would, on an
// HTTP endpoint that a Prometheus server or the collector prometheusreceiver
// scrapes. Metrics-only.
type Output struct {
	st      *store
	col     *collector
	handler http.Handler
	clock   clockwork.Clock
	srv     *http.Server
	ln      net.Listener
	logger  *zap.Logger
	metrics *promscrapeMetrics
	stopped atomic.Bool
}

// New builds a prometheus-scrape output and starts serving. It binds the
// listener eagerly so a port conflict surfaces here rather than at first
// scrape. metricsPath is the path the exposition is served on (e.g. /metrics).
// metricExpiration drops a series not updated within it (0 keeps series
// forever), matching the collector prometheusexporter's metric_expiration.
func New(listenAddr, metricsPath string, emitTimestamps bool, metricExpiration time.Duration, tel embed.TelemetrySettings, logger *zap.Logger) (*Output, error) {
	return newWithClock(listenAddr, metricsPath, emitTimestamps, metricExpiration, tel, logger, clockwork.NewRealClock())
}

func newWithClock(listenAddr, metricsPath string, emitTimestamps bool, metricExpiration time.Duration, tel embed.TelemetrySettings, logger *zap.Logger, clk clockwork.Clock) (*Output, error) {
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

	st := newStore(clk, metricExpiration)
	reg := prometheus.NewRegistry()
	col := newCollector(st, emitTimestamps, clk, metricExpiration, logger)
	reg.MustRegister(col)
	o := &Output{
		st:  st,
		col: col,
		// The exporter's handler options: serve what converted, log the rest.
		handler: promhttp.HandlerFor(reg, promhttp.HandlerOpts{ErrorHandling: promhttp.ContinueOnError, ErrorLog: promLogger{logger}}),
		clock:   clk,
		ln:      ln,
		logger:  logger,
	}

	// Self-telemetry is best-effort: a build failure disables recording rather
	// than the output.
	if m, err := newPromscrapeMetrics(tel); err != nil {
		logger.Warn("prometheus-scrape metrics disabled", zap.Error(err))
	} else {
		o.metrics = m
		col.onCollect = func(exposed int64) { m.recordExposed(context.Background(), exposed) }
	}

	// Match the path exactly: a ServeMux pattern would read "{" or a trailing
	// "/" as syntax, and panics on a malformed one.
	o.srv = &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != metricsPath {
			http.NotFound(w, r)
			return
		}
		o.handleScrape(w, r)
	}), ReadHeaderTimeout: 5 * time.Second}

	go func() {
		if err := o.srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			logger.Warn("prometheus-scrape server stopped", zap.Error(err))
		}
	}()

	return o, nil
}

// handleScrape serves the exposition and records the scrape.
func (o *Output) handleScrape(w http.ResponseWriter, r *http.Request) {
	start := o.clock.Now()
	cw := &countingWriter{ResponseWriter: w}
	o.handler.ServeHTTP(cw, r)
	if o.metrics != nil {
		o.metrics.recordScrape(context.Background(), cw.n, output.DurationMillis(o.clock.Since(start)))
	}
}

// countingWriter counts the body bytes written.
type countingWriter struct {
	http.ResponseWriter
	n int64
}

func (w *countingWriter) Write(b []byte) (int, error) {
	n, err := w.ResponseWriter.Write(b)
	w.n += int64(n)
	return n, err
}

// promLogger routes promhttp's errors to zap at Error, as the exporter does.
type promLogger struct{ l *zap.Logger }

func (p promLogger) Println(v ...any) { p.l.Error(fmt.Sprint(v...)) }

// errStopped refuses a write after Stop: the endpoint no longer serves it.
var errStopped = errors.New("promscrape: output stopped")

// WriteMetric stores the point as its series' latest value.
func (o *Output) WriteMetric(ctx context.Context, data output.MetricRecord) error {
	if o.metrics != nil {
		o.metrics.recordEntryReceived(ctx)
	}
	if o.stopped.Load() {
		return errStopped
	}
	if _, err := prommap.MetricName(data); err != nil {
		return fmt.Errorf("promscrape: map metric: %w", err)
	}
	switch data.Type {
	case embed.MetricTypeGauge, embed.MetricTypeSum, embed.MetricTypeCounter, embed.MetricTypeHistogram:
	default:
		return fmt.Errorf("promscrape: unsupported metric type %q for %q", data.Type, data.Name)
	}
	if data.Type == embed.MetricTypeHistogram && len(data.HistogramBucketCounts) != len(data.HistogramBucketBounds)+1 {
		return fmt.Errorf("promscrape: histogram %q has %d bucket counts, want len(bounds)+1 = %d", data.Name, len(data.HistogramBucketCounts), len(data.HistogramBucketBounds)+1)
	}
	o.st.put(data)
	return nil
}

// SupportedTelemetry reports that only metrics are consumed.
func (o *Output) SupportedTelemetry() []telemetry.Type {
	return []telemetry.Type{telemetry.Metrics}
}

// Stop shuts down the HTTP server; later writes are refused.
func (o *Output) Stop(ctx context.Context) error {
	o.stopped.Store(true)
	return o.srv.Shutdown(ctx)
}

// Addr returns the actual listen address (useful when the port was ephemeral).
func (o *Output) Addr() string {
	return o.ln.Addr().String()
}
