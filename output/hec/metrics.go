package hec

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

var componentAttr = attribute.String("component", "output_hec")

// hecMetrics holds all OTel metrics for the HEC output
type hecMetrics struct {
	logsReceived     metric.Int64Counter
	activeWorkers    metric.Int64Gauge
	logRate          metric.Float64Counter
	requestSizeBytes metric.Int64Histogram
	requestLatency   metric.Float64Histogram
	sendErrors       metric.Int64Counter
	batchSize        metric.Int64Histogram
	ackPending       metric.Int64Gauge
	ackConfirmed     metric.Int64Counter
	ackExpired       metric.Int64Counter
	ackRetried       metric.Int64Counter
	ackDropped       metric.Int64Counter
	ackPollLatency   metric.Float64Histogram
}

func newHECMetrics() (*hecMetrics, error) {
	meter := otel.Meter("blitz-hec-output")

	logsReceived, err := meter.Int64Counter(
		"blitz.hec.logs.received",
		metric.WithDescription("Number of logs received from the write channel"),
	)
	if err != nil {
		return nil, fmt.Errorf("create logs received counter: %w", err)
	}

	activeWorkers, err := meter.Int64Gauge(
		"blitz.hec.workers.active",
		metric.WithDescription("Number of active worker goroutines"),
	)
	if err != nil {
		return nil, fmt.Errorf("create active workers gauge: %w", err)
	}

	logRate, err := meter.Float64Counter(
		"blitz.hec.log.rate",
		metric.WithDescription("Rate at which logs are successfully sent"),
	)
	if err != nil {
		return nil, fmt.Errorf("create log rate counter: %w", err)
	}

	requestSizeBytes, err := meter.Int64Histogram(
		"blitz.hec.request.size.bytes",
		metric.WithDescription("Size of HTTP request bodies in bytes"),
	)
	if err != nil {
		return nil, fmt.Errorf("create request size histogram: %w", err)
	}

	requestLatency, err := meter.Float64Histogram(
		"blitz.hec.request.latency",
		metric.WithDescription("HTTP POST latency in seconds"),
	)
	if err != nil {
		return nil, fmt.Errorf("create request latency histogram: %w", err)
	}

	sendErrors, err := meter.Int64Counter(
		"blitz.hec.send.errors",
		metric.WithDescription("Total number of send errors"),
	)
	if err != nil {
		return nil, fmt.Errorf("create send errors counter: %w", err)
	}

	batchSize, err := meter.Int64Histogram(
		"blitz.hec.batch.size",
		metric.WithDescription("Number of events per batch"),
	)
	if err != nil {
		return nil, fmt.Errorf("create batch size histogram: %w", err)
	}

	ackPending, err := meter.Int64Gauge(
		"blitz.hec.ack.pending",
		metric.WithDescription("Number of ackIds awaiting confirmation"),
	)
	if err != nil {
		return nil, fmt.Errorf("create ack pending gauge: %w", err)
	}

	ackConfirmed, err := meter.Int64Counter(
		"blitz.hec.ack.confirmed",
		metric.WithDescription("Number of ackIds confirmed by Splunk"),
	)
	if err != nil {
		return nil, fmt.Errorf("create ack confirmed counter: %w", err)
	}

	ackExpired, err := meter.Int64Counter(
		"blitz.hec.ack.expired",
		metric.WithDescription("Number of ackIds that timed out"),
	)
	if err != nil {
		return nil, fmt.Errorf("create ack expired counter: %w", err)
	}

	ackRetried, err := meter.Int64Counter(
		"blitz.hec.ack.retried",
		metric.WithDescription("Number of batches resent after ACK timeout"),
	)
	if err != nil {
		return nil, fmt.Errorf("create ack retried counter: %w", err)
	}

	ackDropped, err := meter.Int64Counter(
		"blitz.hec.ack.dropped",
		metric.WithDescription("Number of batches dropped after max retries"),
	)
	if err != nil {
		return nil, fmt.Errorf("create ack dropped counter: %w", err)
	}

	ackPollLatency, err := meter.Float64Histogram(
		"blitz.hec.ack.poll.latency",
		metric.WithDescription("ACK poll request latency in seconds"),
	)
	if err != nil {
		return nil, fmt.Errorf("create ack poll latency histogram: %w", err)
	}

	return &hecMetrics{
		logsReceived:     logsReceived,
		activeWorkers:    activeWorkers,
		logRate:          logRate,
		requestSizeBytes: requestSizeBytes,
		requestLatency:   requestLatency,
		sendErrors:       sendErrors,
		batchSize:        batchSize,
		ackPending:       ackPending,
		ackConfirmed:     ackConfirmed,
		ackExpired:       ackExpired,
		ackRetried:       ackRetried,
		ackDropped:       ackDropped,
		ackPollLatency:   ackPollLatency,
	}, nil
}

func (m *hecMetrics) recordLogsReceived(ctx context.Context, count int64) {
	m.logsReceived.Add(ctx, count, metric.WithAttributeSet(attribute.NewSet(componentAttr)))
}

func (m *hecMetrics) recordActiveWorkers(ctx context.Context, count int64) {
	m.activeWorkers.Record(ctx, count, metric.WithAttributeSet(attribute.NewSet(componentAttr)))
}

func (m *hecMetrics) recordLogRate(ctx context.Context, count float64) {
	m.logRate.Add(ctx, count, metric.WithAttributeSet(attribute.NewSet(componentAttr)))
}

func (m *hecMetrics) recordRequestSize(ctx context.Context, bytes int64) {
	m.requestSizeBytes.Record(ctx, bytes, metric.WithAttributeSet(attribute.NewSet(componentAttr)))
}

func (m *hecMetrics) recordRequestLatency(ctx context.Context, seconds float64) {
	m.requestLatency.Record(ctx, seconds, metric.WithAttributeSet(attribute.NewSet(componentAttr)))
}

func (m *hecMetrics) recordSendError(ctx context.Context, errorType string) {
	m.sendErrors.Add(ctx, 1, metric.WithAttributeSet(attribute.NewSet(
		componentAttr,
		attribute.String("error_type", errorType),
	)))
}

func (m *hecMetrics) recordBatchSize(ctx context.Context, size int64) {
	m.batchSize.Record(ctx, size, metric.WithAttributeSet(attribute.NewSet(componentAttr)))
}

func (m *hecMetrics) recordACKPending(ctx context.Context, count int64) {
	m.ackPending.Record(ctx, count, metric.WithAttributeSet(attribute.NewSet(componentAttr)))
}

func (m *hecMetrics) recordACKConfirmed(ctx context.Context, count int64) {
	m.ackConfirmed.Add(ctx, count, metric.WithAttributeSet(attribute.NewSet(componentAttr)))
}

func (m *hecMetrics) recordACKExpired(ctx context.Context, count int64) {
	m.ackExpired.Add(ctx, count, metric.WithAttributeSet(attribute.NewSet(componentAttr)))
}

func (m *hecMetrics) recordACKRetried(ctx context.Context, count int64) {
	m.ackRetried.Add(ctx, count, metric.WithAttributeSet(attribute.NewSet(componentAttr)))
}

func (m *hecMetrics) recordACKDropped(ctx context.Context, count int64) {
	m.ackDropped.Add(ctx, count, metric.WithAttributeSet(attribute.NewSet(componentAttr)))
}

func (m *hecMetrics) recordACKPollLatency(ctx context.Context, seconds float64) {
	m.ackPollLatency.Record(ctx, seconds, metric.WithAttributeSet(attribute.NewSet(componentAttr)))
}
