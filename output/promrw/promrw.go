package promrw

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/observiq/blitz/embed"
	"github.com/observiq/blitz/internal/prommap"
	"github.com/observiq/blitz/output"
	"github.com/observiq/blitz/telemetry"
	"go.uber.org/zap"
)

// Retry policy for recoverable failures (network errors, 429, 5xx), using
// Prometheus's default backoff bounds.
const (
	minBackoff = 30 * time.Millisecond
	maxBackoff = 5 * time.Second
	// ponytail: Prometheus retries recoverable errors forever and blocks the
	// shard; capped here so a dead endpoint can't wedge the sender. Make it
	// configurable if a caller needs longer.
	maxAttempts = 10
)

// entry is one buffered family plus the target_info series for its resource,
// when the resource yields one.
type entry struct {
	fam    prommap.MetricFamily
	target *prommap.MetricFamily
}

// Output is the prometheus-remote-write output. It follows the Prometheus
// queue-manager model: WriteMetric only buffers, and a single sender goroutine
// owns every POST, sending when a batch fills or the batch deadline passes.
// One sender keeps each series' samples in order on the wire.
type Output struct {
	endpoint     string
	version      version
	batchN       int
	batchTimeout time.Duration
	headers      map[string]string
	client       *http.Client
	logger       *zap.Logger
	metrics      *promrwMetrics
	clock        clockwork.Clock

	mu     sync.Mutex
	buf    []entry
	series int

	full     chan struct{} // capacity 1: batch-full signal to the sender
	stop     chan struct{}
	stopOnce sync.Once
	done     chan struct{}
	drainErr error // set by the sender before done closes
}

// New builds a prometheus-remote-write output and starts its sender. ver is
// "1.0" or "2.0" (empty defaults to 1.0). batchN is the series count that
// triggers a send; batchTimeout bounds how long a partial batch waits (0
// disables the deadline, so partial batches go out only on Stop).
func New(endpoint, ver string, batchN int, batchTimeout, timeout time.Duration, headers map[string]string, tel embed.TelemetrySettings, logger *zap.Logger) (*Output, error) {
	return newWithClock(endpoint, ver, batchN, batchTimeout, timeout, headers, tel, logger, clockwork.NewRealClock())
}

func newWithClock(endpoint, ver string, batchN int, batchTimeout, timeout time.Duration, headers map[string]string, tel embed.TelemetrySettings, logger *zap.Logger, clk clockwork.Clock) (*Output, error) {
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
		endpoint:     endpoint,
		version:      v,
		batchN:       batchN,
		batchTimeout: batchTimeout,
		headers:      headers,
		client:       &http.Client{Timeout: timeout},
		logger:       logger,
		clock:        clk,
		full:         make(chan struct{}, 1),
		stop:         make(chan struct{}),
		done:         make(chan struct{}),
	}

	// Self-telemetry is best-effort: a build failure disables recording rather
	// than the output.
	if m, err := newPromrwMetrics(tel); err != nil {
		logger.Warn("prometheus-remote-write metrics disabled", zap.Error(err))
	} else {
		o.metrics = m
	}

	go o.run()
	return o, nil
}

// Write reports that logs are unsupported: this output is metrics-only.
func (o *Output) Write(_ context.Context, _ output.LogRecord) error {
	return output.ErrUnsupportedTelemetryType
}

// WriteMetric maps a metric record and buffers its series. It never POSTs:
// when the batch fills it signals the sender and returns.
func (o *Output) WriteMetric(ctx context.Context, data output.MetricRecord) error {
	fam, err := prommap.Map(data)
	if err != nil {
		return fmt.Errorf("promrw: map metric: %w", err)
	}
	e := entry{fam: fam}
	if t, ok := prommap.TargetInfo(data); ok {
		e.target = &t
	}

	o.mu.Lock()
	o.buf = append(o.buf, e)
	o.series += len(fam.Samples)
	full := o.series >= o.batchN
	o.mu.Unlock()

	if o.metrics != nil {
		o.metrics.recordSeriesReceived(ctx, int64(len(fam.Samples)))
	}
	if full {
		select {
		case o.full <- struct{}{}:
		default: // a signal is already pending
		}
	}
	return nil
}

// SupportedTelemetry reports that only metrics are consumed.
func (o *Output) SupportedTelemetry() []telemetry.Type {
	return []telemetry.Type{telemetry.Metrics}
}

// Stop signals the sender to drain whatever is buffered and waits for it.
func (o *Output) Stop(ctx context.Context) error {
	o.stopOnce.Do(func() { close(o.stop) })
	select {
	case <-o.done:
		return o.drainErr
	case <-ctx.Done():
		return ctx.Err()
	}
}

// run is the single sender: every POST happens here, one at a time.
func (o *Output) run() {
	defer close(o.done)

	var deadline <-chan time.Time
	if o.batchTimeout > 0 {
		t := o.clock.NewTicker(o.batchTimeout)
		defer t.Stop()
		deadline = t.Chan()
	}

	for {
		select {
		case <-o.stop:
			o.drainErr = o.sendBatches(true)
			return
		case <-o.full:
			o.logSendErr(o.sendBatches(false))
		case <-deadline:
			o.logSendErr(o.sendBatches(true))
		}
	}
}

func (o *Output) logSendErr(err error) {
	if err != nil {
		o.logger.Warn("prometheus-remote-write send failed", zap.Error(err))
	}
}

// sendBatches sends every full batch, and when partial is true also the
// remainder, in batches of at most batchN series. It returns the last error.
func (o *Output) sendBatches(partial bool) error {
	var last error
	for {
		batch, series := o.take(partial)
		if len(batch) == 0 {
			return last
		}
		if err := o.send(batch, series); err != nil {
			last = err
		}
	}
}

// take removes up to batchN series from the buffer. Without partial it takes
// nothing unless a full batch is buffered.
func (o *Output) take(partial bool) ([]entry, int64) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.series == 0 || (!partial && o.series < o.batchN) {
		return nil, 0
	}
	n, i := 0, 0
	for i < len(o.buf) && n < o.batchN {
		n += len(o.buf[i].fam.Samples)
		i++
	}
	batch := make([]entry, i)
	copy(batch, o.buf[:i])
	o.buf = append([]entry(nil), o.buf[i:]...)
	o.series -= n
	return batch, int64(n)
}

// send encodes one batch, adding each distinct target_info once (a repeat
// would be a duplicate sample), and POSTs it with retry.
func (o *Output) send(batch []entry, series int64) error {
	families := make([]prommap.MetricFamily, 0, len(batch)+1)
	targets := map[string]int{}
	for _, e := range batch {
		families = append(families, e.fam)
		if e.target == nil {
			continue
		}
		key := labelKey(e.target.Samples[0].Labels)
		if idx, ok := targets[key]; ok {
			// Keep the newest timestamp for the target.
			if e.target.Samples[0].TimestampMS > families[idx].Samples[0].TimestampMS {
				families[idx] = *e.target
			}
			continue
		}
		targets[key] = len(families)
		families = append(families, *e.target)
	}

	body, err := encode(o.version, families)
	if err != nil {
		return err
	}

	start := o.clock.Now()
	if err := o.postWithRetry(body); err != nil {
		if o.metrics != nil {
			o.metrics.recordPostFailed(context.Background())
		}
		return err
	}
	if o.metrics != nil {
		o.metrics.recordBatch(context.Background(), series, float64(o.clock.Since(start).Milliseconds()))
	}
	return nil
}

func labelKey(labels []prommap.Label) string {
	var b strings.Builder
	for _, l := range labels {
		b.WriteString(l.Name)
		b.WriteByte('\x00')
		b.WriteString(l.Value)
		b.WriteByte('\x00')
	}
	return b.String()
}

// postWithRetry POSTs body, retrying recoverable failures with exponential
// backoff (or the server's Retry-After) up to maxAttempts. Stopping cuts the
// wait short so shutdown is never held by a backoff.
func (o *Output) postWithRetry(body []byte) error {
	backoff := minBackoff
	for attempt := 1; ; attempt++ {
		err := o.post(body)
		if err == nil {
			return nil
		}
		var se *statusError
		recoverable := !errors.As(err, &se) || se.recoverable()
		if !recoverable || attempt >= maxAttempts {
			return err
		}
		wait := backoff
		if se != nil && se.retryAfter > 0 {
			wait = se.retryAfter
		}
		select {
		case <-o.clock.After(wait):
		case <-o.stop:
			return err
		}
		backoff = min(backoff*2, maxBackoff)
	}
}

// statusError is a non-2xx remote-write response.
type statusError struct {
	code       int
	body       string
	retryAfter time.Duration
}

func (e *statusError) Error() string {
	return fmt.Sprintf("promrw: endpoint returned %d: %s", e.code, e.body)
}

// recoverable reports whether the remote-write spec allows a retry: 5xx and
// 429 are retried; other 4xx are dropped.
func (e *statusError) recoverable() bool {
	return e.code == http.StatusTooManyRequests || e.code >= 500
}

func (o *Output) post(body []byte) error {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, o.endpoint, bytes.NewReader(body))
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
	if resp.StatusCode/100 == 2 {
		return nil
	}
	snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
	return &statusError{
		code:       resp.StatusCode,
		body:       string(bytes.TrimSpace(snippet)),
		retryAfter: parseRetryAfter(resp.Header.Get("Retry-After"), o.clock.Now()),
	}
}

// parseRetryAfter reads a Retry-After value as delay-seconds or an HTTP-date.
func parseRetryAfter(v string, now time.Time) time.Duration {
	if v == "" {
		return 0
	}
	if secs, err := strconv.Atoi(v); err == nil && secs > 0 {
		return time.Duration(secs) * time.Second
	}
	if t, err := http.ParseTime(v); err == nil {
		if d := t.Sub(now); d > 0 {
			return d
		}
	}
	return 0
}
