package hec

import (
	"context"

	"github.com/observiq/blitz/output"
)

// outputType is the output_type attribute value for HEC metrics.
const outputType = "hec"

// hecMetrics provides convenience methods that delegate to generated metric wrappers.
// Shared output metrics use the output package-level wrappers (with output_type="hec").
// HEC-specific metrics use the package-level variables from monitoring.go.
type hecMetrics struct{}

func newHECMetrics() (*hecMetrics, error) {
	return &hecMetrics{}, nil
}

func (m *hecMetrics) recordLogsReceived(ctx context.Context, count int64) {
	output.BlitzOutputEntriesReceivedCounter.Add(ctx, count, outputType)
}

func (m *hecMetrics) recordActiveWorkers(ctx context.Context, count int64) {
	output.BlitzOutputActiveWorkersGauge.Record(ctx, count, outputType)
}

func (m *hecMetrics) recordLogRate(ctx context.Context, count float64) {
	output.BlitzOutputEntryRateCounter.Add(ctx, count, outputType)
}

func (m *hecMetrics) recordRequestSize(ctx context.Context, bytes int64) {
	output.BlitzOutputRequestSizeHistogram.Record(ctx, bytes, outputType)
}

func (m *hecMetrics) recordRequestLatency(ctx context.Context, seconds float64) {
	output.BlitzOutputRequestLatencyHistogram.Record(ctx, seconds, outputType)
}

func (m *hecMetrics) recordSendError(ctx context.Context, _ string) {
	output.BlitzOutputSendErrorsCounter.Add(ctx, 1, outputType)
}

func (m *hecMetrics) recordBatchSize(ctx context.Context, size int64) {
	blitzOutputHecBatchSizeHistogram.Record(ctx, size)
}

func (m *hecMetrics) recordACKPending(ctx context.Context, count int64) {
	blitzOutputHecAckPendingGauge.Record(ctx, count)
}

func (m *hecMetrics) recordACKConfirmed(ctx context.Context, count int64) {
	blitzOutputHecAckConfirmedCounter.Add(ctx, count)
}

func (m *hecMetrics) recordACKExpired(ctx context.Context, count int64) {
	blitzOutputHecAckExpiredCounter.Add(ctx, count)
}

func (m *hecMetrics) recordACKRetried(ctx context.Context, count int64) {
	blitzOutputHecAckRetriedCounter.Add(ctx, count)
}

func (m *hecMetrics) recordACKDropped(ctx context.Context, count int64) {
	blitzOutputHecAckDroppedCounter.Add(ctx, count)
}

func (m *hecMetrics) recordACKPollLatency(ctx context.Context, seconds float64) {
	blitzOutputHecAckPollLatencyHistogram.Record(ctx, seconds)
}
