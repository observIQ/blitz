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
	"github.com/observiq/blitz/output"
	"github.com/prometheus/prometheus/prompb"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
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
	o, err := newWithClock(url, "1.0", batchN, batchTimeout, 5*time.Second, nil, embed.TelemetrySettings{}, zap.NewNop(), clk)
	require.NoError(t, err)
	t.Cleanup(func() { _ = o.Stop(context.Background()) })
	return o
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
	for i := int64(1); i <= 3; i++ {
		require.NoError(t, o.WriteMetric(context.Background(), gaugeAt(i, res)))
	}
	require.Eventually(t, func() bool { return rcv.count() == 1 }, waitFor, tick)

	targets := 0
	for _, ts := range rcv.series(t, 0) {
		for _, l := range ts.Labels {
			if l.Name == "__name__" && l.Value == "target_info" {
				targets++
			}
		}
	}
	require.Equal(t, 1, targets)
}
