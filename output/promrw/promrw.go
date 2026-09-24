package promrw

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/observiq/blitz/embed"
	"github.com/observiq/blitz/internal/prommap"
	"github.com/observiq/blitz/output"
	"github.com/observiq/blitz/telemetry"
	"go.uber.org/zap"
)

// Output is the prometheus-remote-write output: it buffers mapped metric
// series and POSTs snappy-compressed remote-write payloads to the endpoint.
type Output struct {
	endpoint string
	version  version
	batchN   int
	timeout  time.Duration
	headers  map[string]string
	client   *http.Client
	logger   *zap.Logger
	metrics  *promrwMetrics

	mu     sync.Mutex
	buf    []prommap.MetricFamily
	series int

	stopFlusher context.CancelFunc
	flusherDone chan struct{}
}

// New builds a prometheus-remote-write output. ver is "1.0" or "2.0" (empty
// defaults to 1.0). batchN is the series count that triggers a flush;
// batchTimeout bounds how long a partial batch waits before flushing.
func New(endpoint, ver string, batchN int, batchTimeout, timeout time.Duration, headers map[string]string, tel embed.TelemetrySettings, logger *zap.Logger) (*Output, error) {
	v := versionV1
	switch ver {
	case "", string(versionV1):
		v = versionV1
	case string(versionV2):
		v = versionV2
	default:
		return nil, fmt.Errorf("promrw: unsupported remote-write version %q", ver)
	}
	if batchN <= 0 {
		batchN = 1
	}
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	if logger == nil {
		logger = zap.NewNop()
	}

	o := &Output{
		endpoint: endpoint,
		version:  v,
		batchN:   batchN,
		timeout:  timeout,
		headers:  headers,
		client:   &http.Client{Timeout: timeout},
		logger:   logger,
	}

	// Self-telemetry is best-effort: a build failure disables recording rather
	// than the output.
	if m, err := newPromrwMetrics(tel); err != nil {
		logger.Warn("prometheus-remote-write metrics disabled", zap.Error(err))
	} else {
		o.metrics = m
	}

	if batchTimeout > 0 {
		ctx, cancel := context.WithCancel(context.Background())
		o.stopFlusher = cancel
		o.flusherDone = make(chan struct{})
		go o.flushLoop(ctx, batchTimeout)
	}
	return o, nil
}

// Write reports that logs are unsupported: this output is metrics-only.
func (o *Output) Write(_ context.Context, _ output.LogRecord) error {
	return output.ErrUnsupportedTelemetryType
}

// WriteMetric maps a metric record and buffers its series, flushing when the
// batch fills.
func (o *Output) WriteMetric(ctx context.Context, data output.MetricRecord) error {
	fam, err := prommap.Map(data)
	if err != nil {
		return fmt.Errorf("promrw: map metric: %w", err)
	}

	o.mu.Lock()
	o.buf = append(o.buf, fam)
	o.series += len(fam.Samples)
	full := o.series >= o.batchN
	o.mu.Unlock()

	if o.metrics != nil {
		o.metrics.recordSeriesReceived(ctx, int64(len(fam.Samples)))
	}
	if full {
		return o.flush(ctx)
	}
	return nil
}

// SupportedTelemetry reports that only metrics are consumed.
func (o *Output) SupportedTelemetry() []telemetry.Type {
	return []telemetry.Type{telemetry.Metrics}
}

// Stop stops the background flusher and flushes any buffered series.
func (o *Output) Stop(ctx context.Context) error {
	if o.stopFlusher != nil {
		o.stopFlusher()
		<-o.flusherDone
	}
	return o.flush(ctx)
}

// flushLoop flushes partial batches on an interval until the context is done.
func (o *Output) flushLoop(ctx context.Context, interval time.Duration) {
	defer close(o.flusherDone)
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if err := o.flush(ctx); err != nil {
				o.logger.Warn("prometheus-remote-write flush failed", zap.Error(err))
			}
		}
	}
}

// flush snapshots the buffered series under the lock, then encodes and POSTs
// them outside the lock so the HTTP round-trip never blocks writers.
func (o *Output) flush(ctx context.Context) error {
	o.mu.Lock()
	families := o.buf
	o.buf = nil
	o.series = 0
	o.mu.Unlock()

	if len(families) == 0 {
		return nil
	}

	var series int64
	for _, fam := range families {
		series += int64(len(fam.Samples))
	}

	body, err := encode(o.version, families)
	if err != nil {
		return err
	}

	start := time.Now()
	if err := o.post(ctx, body); err != nil {
		if o.metrics != nil {
			o.metrics.recordPostFailed(ctx)
		}
		return err
	}
	if o.metrics != nil {
		o.metrics.recordBatch(ctx, series, float64(time.Since(start).Milliseconds()))
	}
	return nil
}

func (o *Output) post(ctx context.Context, body []byte) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("promrw: build request: %w", err)
	}
	req.Header.Set("Content-Encoding", "snappy")
	req.Header.Set("User-Agent", "blitz")
	switch o.version {
	case versionV2:
		req.Header.Set("Content-Type", "application/x-protobuf;proto=io.prometheus.write.v2.Request")
		req.Header.Set("X-Prometheus-Remote-Write-Version", "2.0.0")
	default:
		req.Header.Set("Content-Type", "application/x-protobuf")
		req.Header.Set("X-Prometheus-Remote-Write-Version", "0.1.0")
	}
	for k, v := range o.headers {
		req.Header.Set(k, v)
	}

	resp, err := o.client.Do(req)
	if err != nil {
		return fmt.Errorf("promrw: post to %s: %w", o.endpoint, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("promrw: endpoint returned %d: %s", resp.StatusCode, bytes.TrimSpace(snippet))
	}
	return nil
}
