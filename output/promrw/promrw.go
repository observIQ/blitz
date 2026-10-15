package promrw

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/url"
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
	"go.uber.org/zap/zapcore"
)

// Retry policy for recoverable failures (network errors, 429, 5xx), using
// Prometheus's default backoff bounds.
const (
	minBackoff = 30 * time.Millisecond
	maxBackoff = 5 * time.Second
	// Prometheus retries recoverable errors forever and blocks the
	// shard; capped here so a dead endpoint can't wedge the sender. Make it
	// configurable if a caller needs longer.
	maxAttempts = 10
	// defaultMaxBuffered is Prometheus's default queue capacity per shard.
	defaultMaxBuffered = 10000
	// defaultBatchSize is the series per send when none is configured.
	defaultBatchSize = 500
)

// entry is one buffered family plus the target_info series for its resource,
// when the resource yields one.
type entry struct {
	fam    prommap.MetricFamily
	target *prommap.MetricFamily
}

// errStopped refuses a write after Stop, whose drain it would miss.
var errStopped = errors.New("promrw: output stopped")

// Output is the prometheus-remote-write output. It follows the Prometheus
// queue-manager model: WriteMetric only buffers, and a single sender goroutine
// owns every POST, sending when a batch fills or the batch deadline passes.
// One sender keeps each series' samples in order on the wire.
type Output struct {
	endpoint     string
	version      version
	batchN       int
	maxBuffered  int
	batchTimeout time.Duration
	headers      map[string]string
	client       *http.Client
	logger       *zap.Logger
	dropWarn     *zap.Logger // sampled: a slow endpoint drops on every write
	metrics      *promrwMetrics
	clock        clockwork.Clock

	mu      sync.Mutex
	buf     []entry
	series  int
	stopped bool // set by Stop; later writes are refused

	full     chan struct{} // capacity 1: batch-full signal to the sender
	stop     chan struct{}
	stopOnce sync.Once
	done     chan struct{}
	postCtx  context.Context // cancelled when Stop's context ends
	cancel   context.CancelFunc
}

// New builds a prometheus-remote-write output and starts its sender. ver is
// "1.0" or "2.0" (empty defaults to 1.0). batchN is the series count that
// triggers a send and maxBuffered caps the series waiting to be sent (0 takes
// each default); batchTimeout bounds how long a partial batch waits (0
// disables the deadline, so partial batches go out only on Stop). As config
// validation also requires, endpoint must be an http or https URL with a
// host, and maxBuffered must be at least batchN, since a larger batch never
// fills.
func New(endpoint, ver string, batchN, maxBuffered int, batchTimeout, timeout time.Duration, headers map[string]string, tel embed.TelemetrySettings, logger *zap.Logger) (*Output, error) {
	if u, err := url.Parse(endpoint); err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, fmt.Errorf("promrw: endpoint %q is not an http or https URL with a host", endpoint)
	}
	batch, buffered := batchN, maxBuffered
	if batch <= 0 {
		batch = defaultBatchSize
	}
	if buffered <= 0 {
		buffered = defaultMaxBuffered
	}
	if buffered < batch {
		return nil, fmt.Errorf("promrw: max buffered series (%d) must be at least the batch size (%d)", buffered, batch)
	}
	return newWithClock(endpoint, ver, batchN, maxBuffered, batchTimeout, timeout, headers, tel, logger, clockwork.NewRealClock())
}

func newWithClock(endpoint, ver string, batchN, maxBuffered int, batchTimeout, timeout time.Duration, headers map[string]string, tel embed.TelemetrySettings, logger *zap.Logger, clk clockwork.Clock) (*Output, error) {
	var v version
	switch ver {
	case "", "1", string(versionV1): // "1"/"2": an unquoted YAML 1.0/2.0
		v = versionV1
	case "2", string(versionV2):
		v = versionV2
	default:
		return nil, fmt.Errorf("promrw: unsupported remote-write version %q", ver)
	}
	if batchN <= 0 {
		batchN = defaultBatchSize
	}
	if maxBuffered <= 0 {
		maxBuffered = defaultMaxBuffered
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
		maxBuffered:  maxBuffered,
		batchTimeout: batchTimeout,
		headers:      maps.Clone(headers), // the sender reads it on every POST
		client:       &http.Client{Timeout: timeout},
		logger:       logger,
		dropWarn: logger.WithOptions(zap.WrapCore(func(c zapcore.Core) zapcore.Core {
			return zapcore.NewSamplerWithOptions(c, time.Second, 1, 100)
		})),
		clock: clk,
		full:  make(chan struct{}, 1),
		stop:  make(chan struct{}),
		done:  make(chan struct{}),
	}

	// Self-telemetry is best-effort: a build failure disables recording rather
	// than the output.
	if m, err := newPromrwMetrics(tel); err != nil {
		logger.Warn("prometheus-remote-write metrics disabled", zap.Error(err))
	} else {
		o.metrics = m
	}

	o.postCtx, o.cancel = context.WithCancel(context.Background())
	go o.run()
	return o, nil
}

// Write reports that logs are unsupported: this output is metrics-only.
func (o *Output) Write(_ context.Context, _ output.LogRecord) error {
	return output.ErrUnsupportedTelemetryType
}

// WriteMetric maps a metric record and buffers its series. It never POSTs or
// blocks: when the batch fills it signals the sender and returns. When the
// buffer is over its cap, the oldest series are dropped, as Prometheus loses
// its oldest unsent samples when the WAL is truncated.
func (o *Output) WriteMetric(ctx context.Context, data output.MetricRecord) error {
	if o.metrics != nil {
		o.metrics.recordEntryReceived(ctx)
	}
	fam, err := prommap.Map(data)
	if err != nil {
		return fmt.Errorf("promrw: map metric: %w", err)
	}
	e := entry{fam: fam}
	if t, ok := prommap.TargetInfo(data); ok {
		e.target = &t
	}

	o.mu.Lock()
	if o.stopped {
		o.mu.Unlock()
		return errStopped
	}
	o.buf = append(o.buf, e)
	o.series += len(fam.Samples)
	dropped := 0
	for o.series > o.maxBuffered && len(o.buf) > 1 {
		n := len(o.buf[0].fam.Samples)
		o.buf[0] = entry{} // free the dropped series; the backing array outlives the reslice
		o.buf = o.buf[1:]
		o.series -= n
		dropped += n
	}
	full := o.series >= o.batchN
	o.mu.Unlock()

	if dropped > 0 {
		o.dropWarn.Warn("prometheus-remote-write buffer full: dropped the oldest series",
			zap.Int("series", dropped), zap.Int("max_buffered_series", o.maxBuffered))
		if o.metrics != nil {
			o.metrics.recordSeriesDropped(ctx, int64(dropped))
		}
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

// Stop signals the sender to drain whatever is buffered and waits for it,
// retrying as any send does. A failed drain is logged, as any send failure
// is. When ctx ends first, the in-flight POST or retry wait is cancelled.
func (o *Output) Stop(ctx context.Context) error {
	o.stopOnce.Do(func() {
		o.mu.Lock()
		o.stopped = true
		o.mu.Unlock()
		close(o.stop)
	})
	select {
	case <-o.done:
		return nil
	case <-ctx.Done():
		o.cancel()
		return ctx.Err()
	}
}

// run is the single sender: every POST happens here, one at a time.
func (o *Output) run() {
	defer close(o.done)
	defer o.cancel()

	var deadline <-chan time.Time
	if o.batchTimeout > 0 {
		t := o.clock.NewTicker(o.batchTimeout)
		defer t.Stop()
		deadline = t.Chan()
	}

	for {
		select {
		case <-o.stop:
			o.logSendErr(o.sendBatches(true))
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
// remainder, in batches of about batchN series (see take). It returns the
// last error.
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

// take removes whole families from the buffer until it has batchN series, so
// a histogram's family can carry a batch past batchN; each target's
// target_info series is added when the batch is sent, uncounted. Without
// partial it takes nothing unless a full batch is buffered.
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
		// The batch is lost without a POST, so it counts as one failed POST.
		if o.metrics != nil {
			o.metrics.recordPostFailed(context.Background())
			o.metrics.recordSeriesFailed(context.Background(), series)
		}
		return err
	}
	if err := o.postWithRetry(body); err != nil {
		if o.metrics != nil {
			o.metrics.recordSeriesFailed(context.Background(), series)
		}
		return err
	}
	if o.metrics != nil {
		o.metrics.recordBatch(context.Background(), series)
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
// backoff (or the server's Retry-After) up to maxAttempts. The drain at Stop
// retries too; the end of Stop's context cuts the wait short.
func (o *Output) postWithRetry(body []byte) error {
	backoff := minBackoff
	for attempt := 1; ; attempt++ {
		start := o.clock.Now()
		err := o.post(body)
		if o.metrics != nil {
			o.metrics.recordPostLatency(context.Background(), output.DurationMillis(o.clock.Since(start)))
			if err != nil {
				o.metrics.recordPostFailed(context.Background())
			}
		}
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
		case <-o.postCtx.Done():
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

// maxDrain bounds how much of a response body post reads to reuse its
// connection; a larger body is not worth reading, so the connection closes.
const maxDrain = 64 << 10

func (o *Output) post(body []byte) error {
	req, err := http.NewRequestWithContext(o.postCtx, http.MethodPost, o.endpoint, bytes.NewReader(body))
	if err != nil {
		// Safety net, unreachable through New: it rejects an endpoint no
		// request can be built for.
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
	defer func() {
		// Drain what is left (bounded) so the connection goes back to the
		// pool; a body closed unread costs a new connection per POST.
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxDrain))
		_ = resp.Body.Close()
	}()
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
