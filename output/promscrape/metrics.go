package promscrape

import (
	"context"

	"github.com/observiq/blitz/embed"
	"github.com/observiq/blitz/output"
)

// outputType is the output_type attribute value for shared output metrics.
const outputType = "prometheus-scrape"

// promscrapeMetrics delegates to the shared output metrics (output_type-tagged)
// and this package's generated scrape instruments.
type promscrapeMetrics struct {
	out *output.Metrics
	sc  *Metrics
}

func newPromscrapeMetrics(tel embed.TelemetrySettings) (*promscrapeMetrics, error) {
	out, err := output.NewMetrics(tel.MeterProvider)
	if err != nil {
		return nil, err
	}
	sc, err := NewMetrics(tel.MeterProvider)
	if err != nil {
		return nil, err
	}
	return &promscrapeMetrics{out: out, sc: sc}, nil
}

func (m *promscrapeMetrics) recordSeriesReceived(ctx context.Context, n int64) {
	m.out.BlitzOutputEntriesReceivedCounter.Add(ctx, n, outputType, "metrics")
}

func (m *promscrapeMetrics) recordScrape(ctx context.Context, seriesExposed, bytes int64, latencyMS float64) {
	m.sc.blitzOutputPrometheusScrapeScrapesCounter.Add(ctx, 1)
	m.sc.blitzOutputPrometheusScrapeSeriesExposedGauge.Record(ctx, seriesExposed)
	m.sc.blitzOutputPrometheusScrapeExpositionBytesHistogram.Record(ctx, bytes)
	m.sc.blitzOutputPrometheusScrapeScrapeLatencyHistogram.Record(ctx, latencyMS)
}
