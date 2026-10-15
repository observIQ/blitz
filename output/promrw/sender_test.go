package promrw

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang/snappy"
	"github.com/jonboulle/clockwork"
	"github.com/observiq/blitz/embed"
	"github.com/observiq/blitz/internal/prommap"
	"github.com/observiq/blitz/output"
	"github.com/prometheus/prometheus/prompb"
	"github.com/stretchr/testify/require"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

const (
	waitFor = 2 * time.Second
	tick    = 5 * time.Millisecond
)

// scriptedReceiver answers each request with the next status in script
// (the last status repeats), and records bodies and peak concurrency.
type scriptedReceiver struct {
	mu         sync.Mutex
	script     []int
	retryAfter string
	bodies     [][]byte
	release    chan struct{} // when non-nil, the first request blocks until closed
	inflight   atomic.Int32
	peak       atomic.Int32
}

func (s *scriptedReceiver) handler(w http.ResponseWriter, r *http.Request) {
	n := s.inflight.Add(1)
	defer s.inflight.Add(-1)
	for {
		p := s.peak.Load()
		if n <= p || s.peak.CompareAndSwap(p, n) {
			break
		}
	}
	body, _ := io.ReadAll(r.Body)
	s.mu.Lock()
	idx := len(s.bodies)
	s.bodies = append(s.bodies, body)
	status := http.StatusNoContent
	if len(s.script) > 0 {
		status = s.script[min(idx, len(s.script)-1)]
	}
	release := s.release
	s.mu.Unlock()
	if idx == 0 && release != nil {
		<-release
	}
	if s.retryAfter != "" && status == http.StatusTooManyRequests {
		w.Header().Set("Retry-After", s.retryAfter)
	}
	w.WriteHeader(status)
}

func (s *scriptedReceiver) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.bodies)
}

func (s *scriptedReceiver) series(t *testing.T, i int) []prompb.TimeSeries {
	t.Helper()
	s.mu.Lock()
	body := s.bodies[i]
	s.mu.Unlock()
	raw, err := snappy.Decode(nil, body)
	require.NoError(t, err)
	var req prompb.WriteRequest
	require.NoError(t, req.Unmarshal(raw))
	return req.Timeseries
}

func newTestOutput(t *testing.T, url string, batchN int, batchTimeout time.Duration, clk clockwork.Clock) *Output {
	t.Helper()
	o, err := newWithClock(url, "1.0", batchN, 0, batchTimeout, 5*time.Second, nil, embed.TelemetrySettings{}, zap.NewNop(), clk)
	require.NoError(t, err)
	t.Cleanup(func() { stopNow(o) })
	return o
}

// stopNow stops o for test cleanup without draining: on a fake clock a drain
// retrying a failed POST would wait forever.
func stopNow(o *Output) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_ = o.Stop(ctx)
}

// blockUntil waits for n clock waiters, failing cleanly instead of hanging.
func blockUntil(t *testing.T, clk *clockwork.FakeClock, n int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), waitFor)
	defer cancel()
	require.NoError(t, clk.BlockUntilContext(ctx, n), "sender never waited on the clock")
}

// writeAsync calls WriteMetric and asserts it returns promptly: buffering must
// never block on a POST.
func writeAsync(t *testing.T, o *Output, rec output.MetricRecord) {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- o.WriteMetric(context.Background(), rec) }()
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(waitFor):
		t.Fatal("WriteMetric blocked on a POST")
	}
}

func gaugeAt(sec int64, res map[string]any) output.MetricRecord {
	v := float64(sec)
	return output.MetricRecord{
		Name:        "system.memory.utilization",
		Type:        output.MetricTypeGauge,
		DoubleValue: &v,
		Metadata: output.MetricPointMetadata{
			Timestamp: time.Unix(1700000000+sec, 0),
			Resource:  res,
		},
	}
}

// One sender owns every POST: a full batch signals the sender instead of
// POSTing from WriteMetric, so a second batch cannot race the first.
func TestSender_SingleInFlightPost(t *testing.T) {
	rcv := &scriptedReceiver{release: make(chan struct{})}
	srv := httptest.NewServer(http.HandlerFunc(rcv.handler))
	defer srv.Close()
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { close(rcv.release) }) }
	defer release() // runs before srv.Close so a failing test never hangs
	o := newTestOutput(t, srv.URL, 1, 0, clockwork.NewFakeClock())

	writeAsync(t, o, gaugeAt(1, nil))
	require.Eventually(t, func() bool { return rcv.count() == 1 }, waitFor, tick)
	writeAsync(t, o, gaugeAt(2, nil))

	require.Never(t, func() bool { return rcv.count() > 1 }, 200*time.Millisecond, tick,
		"second batch must wait for the first POST to finish")
	release()
	require.Eventually(t, func() bool { return rcv.count() == 2 }, waitFor, tick)
	require.Equal(t, int32(1), rcv.peak.Load())
}

// A batch that fills while a signal is already pending does not block, and the
// sender's next pass sends every full batch.
func TestSender_FullSignalAlreadyPending(t *testing.T) {
	rcv := &scriptedReceiver{release: make(chan struct{})}
	srv := httptest.NewServer(http.HandlerFunc(rcv.handler))
	defer srv.Close()
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { close(rcv.release) }) }
	defer release()
	o := newTestOutput(t, srv.URL, 1, 0, clockwork.NewFakeClock())

	writeAsync(t, o, gaugeAt(1, nil))
	require.Eventually(t, func() bool { return rcv.count() == 1 }, waitFor, tick)
	writeAsync(t, o, gaugeAt(2, nil)) // signals the busy sender
	writeAsync(t, o, gaugeAt(3, nil)) // finds that signal still pending
	release()
	require.Eventually(t, func() bool { return rcv.count() == 3 }, waitFor, tick)
}

// A partial batch goes out when the deadline elapses.
func TestSender_DeadlineFlush(t *testing.T) {
	rcv := &scriptedReceiver{}
	srv := httptest.NewServer(http.HandlerFunc(rcv.handler))
	defer srv.Close()
	clk := clockwork.NewFakeClock()
	o := newTestOutput(t, srv.URL, 1000, 5*time.Second, clk)

	require.NoError(t, o.WriteMetric(context.Background(), gaugeAt(1, nil)))
	blockUntil(t, clk, 1)
	require.Never(t, func() bool { return rcv.count() > 0 }, 100*time.Millisecond, tick)
	clk.Advance(5 * time.Second)
	require.Eventually(t, func() bool { return rcv.count() == 1 }, waitFor, tick)
}

// A full batch goes out without waiting for the deadline.
func TestSender_SizeFlush(t *testing.T) {
	rcv := &scriptedReceiver{}
	srv := httptest.NewServer(http.HandlerFunc(rcv.handler))
	defer srv.Close()
	o := newTestOutput(t, srv.URL, 2, time.Hour, clockwork.NewFakeClock())

	require.NoError(t, o.WriteMetric(context.Background(), gaugeAt(1, nil)))
	require.NoError(t, o.WriteMetric(context.Background(), gaugeAt(2, nil)))
	require.Eventually(t, func() bool { return rcv.count() == 1 }, waitFor, tick)
	samples := 0
	for _, ts := range rcv.series(t, 0) {
		samples += len(ts.Samples)
	}
	require.Equal(t, 2, samples, "both samples go out in one POST")
}

// 5xx is retried with backoff until it succeeds.
func TestSender_RetriesOn5xx(t *testing.T) {
	rcv := &scriptedReceiver{script: []int{503, 503, 204}}
	srv := httptest.NewServer(http.HandlerFunc(rcv.handler))
	defer srv.Close()
	clk := clockwork.NewFakeClock()
	o := newTestOutput(t, srv.URL, 1, 0, clk)

	require.NoError(t, o.WriteMetric(context.Background(), gaugeAt(1, nil)))
	for want := 1; want <= 2; want++ {
		require.Eventually(t, func() bool { return rcv.count() == want }, waitFor, tick)
		blockUntil(t, clk, 1)
		clk.Advance(maxBackoff)
	}
	require.Eventually(t, func() bool { return rcv.count() == 3 }, waitFor, tick)
	require.Never(t, func() bool { return rcv.count() > 3 }, 100*time.Millisecond, tick)
}

// 429 waits the server's Retry-After rather than the default backoff.
func TestSender_HonorsRetryAfter(t *testing.T) {
	rcv := &scriptedReceiver{script: []int{429, 204}, retryAfter: "2"}
	srv := httptest.NewServer(http.HandlerFunc(rcv.handler))
	defer srv.Close()
	clk := clockwork.NewFakeClock()
	o := newTestOutput(t, srv.URL, 1, 0, clk)

	require.NoError(t, o.WriteMetric(context.Background(), gaugeAt(1, nil)))
	require.Eventually(t, func() bool { return rcv.count() == 1 }, waitFor, tick)
	blockUntil(t, clk, 1)
	clk.Advance(time.Second)
	require.Never(t, func() bool { return rcv.count() > 1 }, 100*time.Millisecond, tick)
	clk.Advance(time.Second)
	require.Eventually(t, func() bool { return rcv.count() == 2 }, waitFor, tick)
}

// Other 4xx responses are not retried.
func TestSender_NoRetryOn4xx(t *testing.T) {
	rcv := &scriptedReceiver{script: []int{400}}
	srv := httptest.NewServer(http.HandlerFunc(rcv.handler))
	defer srv.Close()
	o := newTestOutput(t, srv.URL, 1, 0, clockwork.NewFakeClock())

	require.NoError(t, o.WriteMetric(context.Background(), gaugeAt(1, nil)))
	require.Eventually(t, func() bool { return rcv.count() == 1 }, waitFor, tick)
	require.Never(t, func() bool { return rcv.count() > 1 }, 200*time.Millisecond, tick)
}

// Recoverable failures stop after maxAttempts so a dead endpoint can't wedge
// the sender.
func TestSender_RetryCap(t *testing.T) {
	rcv := &scriptedReceiver{script: []int{500}}
	srv := httptest.NewServer(http.HandlerFunc(rcv.handler))
	defer srv.Close()
	clk := clockwork.NewFakeClock()
	o := newTestOutput(t, srv.URL, 1, 0, clk)

	require.NoError(t, o.WriteMetric(context.Background(), gaugeAt(1, nil)))
	for want := 1; want < maxAttempts; want++ {
		require.Eventually(t, func() bool { return rcv.count() == want }, waitFor, tick)
		blockUntil(t, clk, 1)
		clk.Advance(maxBackoff)
	}
	require.Eventually(t, func() bool { return rcv.count() == maxAttempts }, waitFor, tick)
	require.Never(t, func() bool { return rcv.count() > maxAttempts }, 200*time.Millisecond, tick)
}

// target_info is sent once per distinct target per batch; repeating it would
// be a duplicate sample the receiver rejects.
func TestSender_TargetInfoOncePerBatch(t *testing.T) {
	rcv := &scriptedReceiver{}
	srv := httptest.NewServer(http.HandlerFunc(rcv.handler))
	defer srv.Close()
	o := newTestOutput(t, srv.URL, 3, time.Hour, clockwork.NewFakeClock())

	res := map[string]any{"host.name": "athena", "telemetry.source": "hostmetrics"}
	// Out of order, so the newest is neither the first nor the last written.
	for _, i := range []int64{2, 3, 1} {
		require.NoError(t, o.WriteMetric(context.Background(), gaugeAt(i, res)))
	}
	require.Eventually(t, func() bool { return rcv.count() == 1 }, waitFor, tick)

	var targets []prompb.TimeSeries
	for _, ts := range rcv.series(t, 0) {
		for _, l := range ts.Labels {
			if l.Name == "__name__" && l.Value == "target_info" {
				targets = append(targets, ts)
			}
		}
	}
	require.Len(t, targets, 1)
	require.Equal(t, int64(1700000003000), targets[0].Samples[0].Timestamp, "the newest point's target_info is kept")
}

// take hands the sender batchN series at most (a whole family may carry it
// past), oldest first, and leaves the rest buffered.
func TestTakeStopsAtBatchSize(t *testing.T) {
	one := func(v float64) entry {
		return entry{fam: prommap.MetricFamily{Samples: []prommap.Sample{{Name: "m", Value: v}}}}
	}
	o := &Output{batchN: 2, buf: []entry{one(1), one(2), one(3), one(4), one(5)}, series: 5}
	batch, n := o.take(false)
	require.Equal(t, int64(2), n)
	require.Len(t, batch, 2)
	require.Equal(t, 1.0, batch[0].fam.Samples[0].Value)
	require.Equal(t, 3, o.series)
	require.Len(t, o.buf, 3)
}

// A full buffer drops its oldest series, as Prometheus loses the oldest
// unsent samples when its WAL is truncated: WriteMetric never blocks or
// fails, the newest series are kept, and each drop is logged and counted.
func TestSender_FullBufferDropsOldest(t *testing.T) {
	core, logs := observer.New(zap.WarnLevel)
	reader := sdkmetric.NewManualReader()
	tel := embed.TelemetrySettings{MeterProvider: sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))}
	// No deadline and a batch larger than the cap: nothing is ever sent, so
	// only the cap bounds the buffer.
	o, err := newWithClock("http://127.0.0.1:1/api/v1/write", "1.0", 100, 3, 0, time.Second, nil, tel, zap.New(core), clockwork.NewFakeClock())
	require.NoError(t, err)
	t.Cleanup(func() { stopNow(o) })

	for sec := int64(1); sec <= 5; sec++ {
		writeAsync(t, o, gaugeAt(sec, nil))
	}

	o.mu.Lock()
	var kept []int64
	for _, e := range o.buf {
		kept = append(kept, e.fam.Samples[0].TimestampMS/1000-1700000000)
	}
	series := o.series
	o.mu.Unlock()
	require.Equal(t, []int64{3, 4, 5}, kept, "the newest series are kept")
	require.Equal(t, 3, series)
	require.NotZero(t, logs.FilterMessageSnippet("dropped").Len(), "drops are logged")

	var rm metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(context.Background(), &rm))
	var dropped int64
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name == "blitz.output.prometheus_remote_write.series_dropped" {
				for _, dp := range m.Data.(metricdata.Sum[int64]).DataPoints {
					dropped += dp.Value
				}
			}
		}
	}
	require.Equal(t, int64(2), dropped)
}

// A failed final drain is logged like any other send failure, not returned
// from Stop: shutdown succeeded even when the endpoint did not take the data.
func TestSender_StopLogsDrainFailure(t *testing.T) {
	rcv := &scriptedReceiver{script: []int{400}}
	srv := httptest.NewServer(http.HandlerFunc(rcv.handler))
	defer srv.Close()
	core, logs := observer.New(zap.WarnLevel)
	o, err := newWithClock(srv.URL, "1.0", 100, 0, 0, 5*time.Second, nil, embed.TelemetrySettings{}, zap.New(core), clockwork.NewFakeClock())
	require.NoError(t, err)

	require.NoError(t, o.WriteMetric(context.Background(), gaugeAt(1, nil)))
	require.NoError(t, o.Stop(context.Background()))
	require.Equal(t, 1, rcv.count())
	require.Equal(t, 1, logs.FilterMessage("prometheus-remote-write send failed").Len())
}

// The drain at Stop retries a recoverable failure like any other send, so a
// transient 503 at shutdown does not lose the last batch.
func TestSender_StopDrainRetries(t *testing.T) {
	rcv := &scriptedReceiver{script: []int{503, 204}}
	srv := httptest.NewServer(http.HandlerFunc(rcv.handler))
	defer srv.Close()
	clk := clockwork.NewFakeClock()
	o := newTestOutput(t, srv.URL, 100, 0, clk)

	require.NoError(t, o.WriteMetric(context.Background(), gaugeAt(1, nil)))
	stopped := make(chan error, 1)
	go func() { stopped <- o.Stop(context.Background()) }()
	require.Eventually(t, func() bool { return rcv.count() == 1 }, waitFor, tick)
	blockUntil(t, clk, 1)
	clk.Advance(maxBackoff)
	require.NoError(t, <-stopped)
	require.Equal(t, 2, rcv.count())
}

// Stop's context bounds a drain's retries: when it ends, the backoff wait
// ends with it.
func TestSender_StopContextCutsDrainBackoff(t *testing.T) {
	rcv := &scriptedReceiver{script: []int{503}}
	srv := httptest.NewServer(http.HandlerFunc(rcv.handler))
	defer srv.Close()
	clk := clockwork.NewFakeClock()
	o := newTestOutput(t, srv.URL, 100, 0, clk)

	require.NoError(t, o.WriteMetric(context.Background(), gaugeAt(1, nil)))
	ctx, cancel := context.WithCancel(context.Background())
	stopped := make(chan error, 1)
	go func() { stopped <- o.Stop(ctx) }()
	require.Eventually(t, func() bool { return rcv.count() == 1 }, waitFor, tick)
	cancel()
	require.ErrorIs(t, <-stopped, context.Canceled)
	require.Eventually(t, func() bool {
		select {
		case <-o.done:
			return true
		default:
			return false
		}
	}, waitFor, tick, "sender still waiting out the backoff")
	require.Equal(t, 1, rcv.count())
}

// Stop's context bounds shutdown: when it ends, the in-flight POST is
// cancelled rather than left to run out the HTTP timeout.
func TestSender_StopCancelsInFlightPost(t *testing.T) {
	rcv := &scriptedReceiver{release: make(chan struct{})}
	srv := httptest.NewServer(http.HandlerFunc(rcv.handler))
	defer srv.Close()
	defer close(rcv.release)
	o := newTestOutput(t, srv.URL, 1, 0, clockwork.NewFakeClock())

	require.NoError(t, o.WriteMetric(context.Background(), gaugeAt(1, nil)))
	require.Eventually(t, func() bool { return rcv.count() == 1 }, waitFor, tick)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	require.ErrorIs(t, o.Stop(ctx), context.Canceled)
	require.Eventually(t, func() bool {
		select {
		case <-o.done:
			return true
		default:
			return false
		}
	}, waitFor, tick, "sender still blocked on the cancelled POST")
}

// A zero batch size takes the default, as the config's validation assumes,
// rather than sending one POST per series.
func TestNew_ZeroBatchSizeTakesTheDefault(t *testing.T) {
	o, err := newWithClock("http://127.0.0.1:1", "1.0", 0, 0, 0, time.Second, nil, embed.TelemetrySettings{}, zap.NewNop(), clockwork.NewFakeClock())
	require.NoError(t, err)
	t.Cleanup(func() { stopNow(o) })
	require.Equal(t, defaultBatchSize, o.batchN)
}

// A write after Stop is refused: the sender has drained and exited, so a
// buffered series would never be sent.
func TestWriteMetricAfterStopFails(t *testing.T) {
	rcv := &scriptedReceiver{}
	srv := httptest.NewServer(http.HandlerFunc(rcv.handler))
	defer srv.Close()
	o := newTestOutput(t, srv.URL, 100, 0, clockwork.NewFakeClock())

	require.NoError(t, o.Stop(context.Background()))
	require.Error(t, o.WriteMetric(context.Background(), gaugeAt(1, nil)))
}

// A batch that cannot be encoded is lost, so it is counted as a failed post,
// as a batch the endpoint rejects is, rather than vanishing from telemetry.
func TestSender_EncodeFailureCountsAsFailedPost(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	tel := embed.TelemetrySettings{MeterProvider: sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))}
	o, err := newWithClock("http://127.0.0.1:1/api/v1/write", "1.0", 100, 0, 0, time.Second, nil, tel, zap.NewNop(), clockwork.NewFakeClock())
	require.NoError(t, err)
	o.version = "bogus" // the only way to make encode fail

	require.NoError(t, o.WriteMetric(context.Background(), gaugeAt(1, nil)))
	require.NoError(t, o.Stop(context.Background()))

	var rm metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(context.Background(), &rm))
	var failed int64
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name == "blitz.output.prometheus_remote_write.posts_failed" {
				for _, dp := range m.Data.(metricdata.Sum[int64]).DataPoints {
					failed += dp.Value
				}
			}
		}
	}
	require.Equal(t, int64(1), failed)
}

// Post latency keeps sub-millisecond precision: a POST that takes 500µs
// records 0.5 ms, not a truncated 0.
func TestSender_PostLatencyKeepsFractionalMillis(t *testing.T) {
	clk := clockwork.NewFakeClock()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		clk.Advance(500 * time.Microsecond)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	reader := sdkmetric.NewManualReader()
	tel := embed.TelemetrySettings{MeterProvider: sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))}
	o, err := newWithClock(srv.URL, "1.0", 1, 0, 0, 5*time.Second, nil, tel, zap.NewNop(), clk)
	require.NoError(t, err)
	t.Cleanup(func() { stopNow(o) })

	require.NoError(t, o.WriteMetric(context.Background(), gaugeAt(1, nil)))
	var sum float64
	require.Eventually(t, func() bool {
		var rm metricdata.ResourceMetrics
		if err := reader.Collect(context.Background(), &rm); err != nil {
			return false
		}
		for _, sm := range rm.ScopeMetrics {
			for _, m := range sm.Metrics {
				if m.Name == "blitz.output.prometheus_remote_write.post_latency" {
					for _, dp := range m.Data.(metricdata.Histogram[float64]).DataPoints {
						if dp.Count == 1 {
							sum = dp.Sum
							return true
						}
					}
				}
			}
		}
		return false
	}, 5*time.Second, 10*time.Millisecond)
	require.InDelta(t, 0.5, sum, 1e-9)
}

// entries_received counts records, one per point, as every output does: a
// histogram point that maps to several series counts once, and a point that
// fails to map still counts, since the output received it.
func TestEntriesReceivedCountsPoints(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	tel := embed.TelemetrySettings{MeterProvider: sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))}
	o, err := newWithClock("http://127.0.0.1:1/api/v1/write", "1.0", 100, 0, 0, time.Second, nil, tel, zap.NewNop(), clockwork.NewFakeClock())
	require.NoError(t, err)
	t.Cleanup(func() { stopNow(o) })

	rec := output.MetricRecord{
		Name: "rpc.duration", Unit: "s", Type: output.MetricTypeHistogram,
		HistogramCount: 6, HistogramSum: 4.2,
		HistogramBucketBounds: []float64{0.1, 1}, HistogramBucketCounts: []uint64{1, 2, 3},
		Metadata: output.MetricPointMetadata{Timestamp: time.Unix(1700000000, 0)},
	}
	require.NoError(t, o.WriteMetric(context.Background(), rec))
	require.Error(t, o.WriteMetric(context.Background(), output.MetricRecord{Type: output.MetricTypeGauge}))

	var rm metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(context.Background(), &rm))
	var got int64
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name == "blitz.output.entries_received" {
				for _, dp := range m.Data.(metricdata.Sum[int64]).DataPoints {
					got += dp.Value
				}
			}
		}
	}
	require.Equal(t, int64(2), got)
}

// TestSender_DropClearsTheOldestSlot pins that dropping the oldest series
// clears its slot, so the dropped labels and samples are not kept reachable
// by the buffer's backing array.
func TestSender_DropClearsTheOldestSlot(t *testing.T) {
	o, err := newWithClock("http://127.0.0.1:1/api/v1/write", "1.0", 100, 3, 0, time.Second, nil, embed.TelemetrySettings{}, zap.NewNop(), clockwork.NewFakeClock())
	require.NoError(t, err)
	t.Cleanup(func() { stopNow(o) })
	for sec := int64(1); sec <= 3; sec++ {
		writeAsync(t, o, gaugeAt(sec, nil))
	}
	o.mu.Lock()
	o.buf = append(make([]entry, 0, 8), o.buf...) // room to grow in place
	held := o.buf[:2]
	o.mu.Unlock()

	writeAsync(t, o, gaugeAt(4, nil))

	o.mu.Lock()
	defer o.mu.Unlock()
	require.True(t, &held[1] == &o.buf[0], "the drop resliced in place")
	require.Zero(t, held[0], "the dropped slot is cleared")
}

// TestNewRejectsAnInvalidEndpoint pins that New, like config validation,
// refuses an endpoint that is not an http or https URL, instead of retrying
// every batch against a request that can never be built.
func TestNewRejectsAnInvalidEndpoint(t *testing.T) {
	for _, ep := range []string{"", "://bad", "ftp://host/write", "http://host:port/write", "http://127.0.0.1:9090/\x7f", "http:/prom.example.com:9090/api/v1/write"} {
		_, err := New(ep, "1.0", 0, 0, time.Hour, time.Second, nil, embed.TelemetrySettings{}, zap.NewNop())
		require.Error(t, err, ep)
	}
}

// Each POST attempt is its own sample: a 429 retried after Retry-After counts
// one failed POST, and post_latency records two requests without the wait.
func TestSender_MetricsCountEachAttempt(t *testing.T) {
	rcv := &scriptedReceiver{script: []int{429, 204}, retryAfter: "2"}
	srv := httptest.NewServer(http.HandlerFunc(rcv.handler))
	defer srv.Close()
	clk := clockwork.NewFakeClock()
	reader := sdkmetric.NewManualReader()
	tel := embed.TelemetrySettings{MeterProvider: sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))}
	o, err := newWithClock(srv.URL, "1.0", 1, 0, 0, 5*time.Second, nil, tel, zap.NewNop(), clk)
	require.NoError(t, err)
	t.Cleanup(func() { stopNow(o) })

	require.NoError(t, o.WriteMetric(context.Background(), gaugeAt(1, nil)))
	blockUntil(t, clk, 1)
	clk.Advance(2 * time.Second)

	var failed int64
	var latency metricdata.HistogramDataPoint[float64]
	require.Eventually(t, func() bool {
		var rm metricdata.ResourceMetrics
		if err := reader.Collect(context.Background(), &rm); err != nil {
			return false
		}
		failed, latency = 0, metricdata.HistogramDataPoint[float64]{}
		for _, sm := range rm.ScopeMetrics {
			for _, m := range sm.Metrics {
				switch m.Name {
				case "blitz.output.prometheus_remote_write.posts_failed":
					for _, dp := range m.Data.(metricdata.Sum[int64]).DataPoints {
						failed += dp.Value
					}
				case "blitz.output.prometheus_remote_write.post_latency":
					latency = m.Data.(metricdata.Histogram[float64]).DataPoints[0]
				}
			}
		}
		return rcv.count() == 2 && latency.Count == 2
	}, waitFor, tick)
	require.Equal(t, int64(1), failed)
	require.Less(t, latency.Sum, 2000.0, "the Retry-After wait is not request latency")
}

// A batch the endpoint rejects for good is lost, so its series count as
// failed: a receiver rejecting every batch must not leave the series
// counters reading as if nothing were lost.
func TestSender_RejectedBatchCountsFailedSeries(t *testing.T) {
	rcv := &scriptedReceiver{script: []int{400}}
	srv := httptest.NewServer(http.HandlerFunc(rcv.handler))
	defer srv.Close()
	reader := sdkmetric.NewManualReader()
	tel := embed.TelemetrySettings{MeterProvider: sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))}
	o, err := newWithClock(srv.URL, "1.0", 2, 0, 0, 5*time.Second, nil, tel, zap.NewNop(), clockwork.NewFakeClock())
	require.NoError(t, err)
	t.Cleanup(func() { stopNow(o) })

	require.NoError(t, o.WriteMetric(context.Background(), gaugeAt(1, nil)))
	require.NoError(t, o.WriteMetric(context.Background(), gaugeAt(2, map[string]any{"host.name": "h2"})))
	var failed int64
	require.Eventually(t, func() bool {
		var rm metricdata.ResourceMetrics
		if err := reader.Collect(context.Background(), &rm); err != nil {
			return false
		}
		failed = 0
		for _, sm := range rm.ScopeMetrics {
			for _, m := range sm.Metrics {
				if m.Name == "blitz.output.prometheus_remote_write.series_failed" {
					for _, dp := range m.Data.(metricdata.Sum[int64]).DataPoints {
						failed += dp.Value
					}
				}
			}
		}
		return failed > 0
	}, waitFor, tick)
	require.Equal(t, int64(2), failed)
}
