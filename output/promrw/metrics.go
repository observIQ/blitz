package promrw

import (
	"context"

	"github.com/observiq/blitz/embed"
	"github.com/observiq/blitz/output"
)

// outputType is the output_type attribute value for shared output metrics.
const outputType = "prometheus-remote-write"

// promrwMetrics delegates to the shared output metrics (output_type-tagged) and
// this package's generated remote-write instruments.
type promrwMetrics struct {
	out *output.Metrics
	rw  *Metrics
}

func newPromrwMetrics(tel embed.TelemetrySettings) (*promrwMetrics, error) {
	out, err := output.NewMetrics(tel.MeterProvider)
	if err != nil {
		return nil, err
	}
	rw, err := NewMetrics(tel.MeterProvider)
	if err != nil {
		return nil, err
	}
	return &promrwMetrics{out: out, rw: rw}, nil
}

func (m *promrwMetrics) recordSeriesReceived(ctx context.Context, n int64) {
	m.out.BlitzOutputEntriesReceivedCounter.Add(ctx, n, outputType, "metrics")
}

func (m *promrwMetrics) recordBatch(ctx context.Context, series int64, latencyMS float64) {
	m.rw.blitzOutputPrometheusRemoteWriteBatchSizeHistogram.Record(ctx, series)
	m.rw.blitzOutputPrometheusRemoteWritePostLatencyHistogram.Record(ctx, latencyMS)
	m.rw.blitzOutputPrometheusRemoteWriteSeriesSentCounter.Add(ctx, series)
}

func (m *promrwMetrics) recordPostFailed(ctx context.Context) {
	m.rw.blitzOutputPrometheusRemoteWritePostsFailedCounter.Add(ctx, 1)
}
