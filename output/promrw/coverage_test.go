package promrw

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/metric"
	metricnoop "go.opentelemetry.io/otel/metric/noop"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/observiq/blitz/embed"
	"github.com/observiq/blitz/output"
)

// Configured headers (an Authorization token, say) go on every request.
func TestHeadersAreSent(t *testing.T) {
	rcv := &capturingReceiver{}
	srv := httptest.NewServer(http.HandlerFunc(rcv.handler))
	defer srv.Close()
	o, err := New(srv.URL, "1.0", 1, 0, time.Hour, 5*time.Second, map[string]string{"Authorization": "Bearer t"}, embed.TelemetrySettings{}, zap.NewNop())
	require.NoError(t, err)
	require.NoError(t, o.WriteMetric(context.Background(), metricRecord()))
	require.NoError(t, o.Stop(context.Background()))
	rcv.mu.Lock()
	defer rcv.mu.Unlock()
	require.NotEmpty(t, rcv.headers)
	assert.Equal(t, "Bearer t", rcv.headers[0].Get("Authorization"))
}

// Retry-After is delay-seconds or an HTTP-date; anything else, or a time
// already past, means no hint.
func TestParseRetryAfter(t *testing.T) {
	now := time.Date(2026, 10, 12, 12, 0, 0, 0, time.UTC)
	for v, want := range map[string]time.Duration{
		"":                              0,
		"5":                             5 * time.Second,
		"0":                             0,
		"soon":                          0,
		"Mon, 12 Oct 2026 12:00:30 GMT": 30 * time.Second,
		"Mon, 12 Oct 2026 11:59:00 GMT": 0,
	} {
		assert.Equal(t, want, parseRetryAfter(v, now), v)
	}
}

// New rejects an unknown version and fills the defaults: a 30 s client
// timeout and a no-op logger.
func TestNewDefaultsAndVersion(t *testing.T) {
	_, err := New("http://127.0.0.1:9090/api/v1/write", "3.0", 0, 0, time.Hour, 0, nil, embed.TelemetrySettings{}, zap.NewNop())
	require.ErrorContains(t, err, "unsupported remote-write version")
	o, err := New("http://127.0.0.1:9090/api/v1/write", "", 0, 0, time.Hour, 0, nil, embed.TelemetrySettings{}, nil)
	require.NoError(t, err)
	assert.Equal(t, 30*time.Second, o.client.Timeout)
	assert.NotNil(t, o.logger)
	require.NoError(t, o.Stop(context.Background()))
}

// A metrics build failure disables self-telemetry with a Warn, whether the
// shared output metrics or the remote-write ones fail.
func TestMetricsFailureDisablesRecording(t *testing.T) {
	for _, mp := range []metric.MeterProvider{failingMeterProvider{gauge: true}, failingMeterProvider{histogram: true}} {
		core, logs := observer.New(zap.WarnLevel)
		o, err := New("http://127.0.0.1:9090/api/v1/write", "", 0, 0, time.Hour, 0, nil, embed.TelemetrySettings{MeterProvider: mp}, zap.New(core))
		require.NoError(t, err)
		assert.Nil(t, o.metrics)
		assert.Equal(t, 1, logs.FilterMessage("prometheus-remote-write metrics disabled").Len())
		require.NoError(t, o.Stop(context.Background()))
	}
}

// WriteMetric rejects a point prommap cannot map.
func TestWriteMetricRejectsUnmappable(t *testing.T) {
	o, err := New("http://127.0.0.1:9090/api/v1/write", "", 0, 0, time.Hour, 0, nil, embed.TelemetrySettings{}, zap.NewNop())
	require.NoError(t, err)
	t.Cleanup(func() { stopNow(o) })
	require.ErrorContains(t, o.WriteMetric(context.Background(), output.MetricRecord{Type: output.MetricTypeGauge}), "map metric")
}

// A status error names the code and the endpoint's reply; a request that
// cannot be built is an error too.
func TestStatusErrorAndBadRequest(t *testing.T) {
	assert.Equal(t, "promrw: endpoint returned 400: bad label", (&statusError{code: 400, body: "bad label"}).Error())
	// New rejects this endpoint; newWithClock reaches post's safety net.
	o, err := newWithClock("http://127.0.0.1:9090/\x7f", "", 0, 0, time.Hour, 0, nil, embed.TelemetrySettings{}, zap.NewNop(), clockwork.NewFakeClock())
	require.NoError(t, err)
	t.Cleanup(func() { stopNow(o) })
	require.ErrorContains(t, o.post(nil), "build request")
}

// failingMeterProvider fails the shared output metrics' first instrument
// (a gauge) or the remote-write metrics' first (its batch-size histogram).
type failingMeterProvider struct {
	metric.MeterProvider
	gauge, histogram bool
}

func (f failingMeterProvider) Meter(string, ...metric.MeterOption) metric.Meter {
	return failingMeter{Meter: metricnoop.NewMeterProvider().Meter("test"), f: f}
}

type failingMeter struct {
	metric.Meter
	f failingMeterProvider
}

func (m failingMeter) Int64Gauge(name string, opts ...metric.Int64GaugeOption) (metric.Int64Gauge, error) {
	if m.f.gauge {
		return nil, errors.New("instrument error")
	}
	return m.Meter.Int64Gauge(name, opts...)
}

func (m failingMeter) Int64Histogram(name string, opts ...metric.Int64HistogramOption) (metric.Int64Histogram, error) {
	if m.f.histogram && strings.Contains(name, "prometheus_remote_write") {
		return nil, errors.New("instrument error")
	}
	return m.Meter.Int64Histogram(name, opts...)
}

// Response bodies are drained before close, so POSTs reuse one connection
// whatever the receiver sends back, success or error.
func TestPostsReuseTheConnection(t *testing.T) {
	for name, tc := range map[string]struct {
		code int
		body string
	}{
		"2xx with a body":      {200, strings.Repeat("x", 100)},
		"error with long body": {500, strings.Repeat("x", 2048)},
	} {
		var conns atomic.Int32
		srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(tc.code)
			_, _ = w.Write([]byte(tc.body))
		}))
		srv.Config.ConnState = func(_ net.Conn, s http.ConnState) {
			if s == http.StateNew {
				conns.Add(1)
			}
		}
		srv.Start()
		o, err := New(srv.URL, "1.0", 1, 0, time.Hour, 5*time.Second, nil, embed.TelemetrySettings{}, zap.NewNop())
		require.NoError(t, err)
		for range 5 {
			_ = o.post([]byte("x"))
		}
		stopNow(o)
		srv.Close()
		assert.Equal(t, int32(1), conns.Load(), name)
	}
}
