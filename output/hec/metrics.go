package hec

import (
	"context"

	"github.com/observiq/blitz/embed"
	"github.com/observiq/blitz/output"
)

// outputType is the output_type attribute value for HEC metrics.
const outputType = "hec"

// hecMetrics provides convenience methods that delegate to the per-instance
// metric structs. Shared output metrics record through *output.Metrics (with
// output_type="hec"); HEC-specific metrics record through this package's
// generated *Metrics.
type hecMetrics struct {
	out *output.Metrics
	hec *Metrics
}

func newHECMetrics(tel embed.TelemetrySettings) (*hecMetrics, error) {
	out, err := output.NewMetrics(tel.MeterProvider)
	if err != nil {
		return nil, err
	}
	hecM, err := NewMetrics(tel.MeterProvider)
	if err != nil {
		return nil, err
	}
	return &hecMetrics{out: out, hec: hecM}, nil
}

func (m *hecMetrics) recordLogsReceived(ctx context.Context, count int64) {
	m.out.BlitzOutputEntriesReceivedCounter.Add(ctx, count, outputType, "logs")
}

func (m *hecMetrics) recordActiveWorkers(ctx context.Context, count int64) {
	m.out.BlitzOutputActiveWorkersGauge.Record(ctx, count, outputType)
}

func (m *hecMetrics) recordLogRate(ctx context.Context, count float64) {
	m.out.BlitzOutputEntryRateCounter.Add(ctx, count, outputType, "logs")
}

func (m *hecMetrics) recordRequestSize(ctx context.Context, bytes int64) {
	m.out.BlitzOutputRequestSizeHistogram.Record(ctx, bytes, outputType, "logs")
}

func (m *hecMetrics) recordRequestLatency(ctx context.Context, seconds float64) {
	m.out.BlitzOutputRequestLatencyHistogram.Record(ctx, seconds, outputType, "logs")
}

func (m *hecMetrics) recordSendError(ctx context.Context, _ string) {
	m.out.BlitzOutputSendErrorsCounter.Add(ctx, 1, outputType, "logs")
}

func (m *hecMetrics) recordBatchSize(ctx context.Context, size int64) {
	m.hec.blitzOutputHecBatchSizeHistogram.Record(ctx, size)
}

func (m *hecMetrics) recordACKPending(ctx context.Context, count int64) {
	m.hec.blitzOutputHecAckPendingGauge.Record(ctx, count)
}

func (m *hecMetrics) recordACKConfirmed(ctx context.Context, count int64) {
	m.hec.blitzOutputHecAckConfirmedCounter.Add(ctx, count)
}

func (m *hecMetrics) recordACKExpired(ctx context.Context, count int64) {
	m.hec.blitzOutputHecAckExpiredCounter.Add(ctx, count)
}

func (m *hecMetrics) recordACKRetried(ctx context.Context, count int64) {
	m.hec.blitzOutputHecAckRetriedCounter.Add(ctx, count)
}

func (m *hecMetrics) recordACKDropped(ctx context.Context, count int64) {
	m.hec.blitzOutputHecAckDroppedCounter.Add(ctx, count)
}

func (m *hecMetrics) recordACKPollLatency(ctx context.Context, seconds float64) {
	m.hec.blitzOutputHecAckPollLatencyHistogram.Record(ctx, seconds)
}
