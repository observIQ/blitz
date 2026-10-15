package promrw

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/golang/snappy"
	"github.com/observiq/blitz/embed"
	"github.com/observiq/blitz/output"
	"github.com/prometheus/prometheus/prompb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func i64(v int64) *int64 { return &v }

// capturingReceiver records the bodies and headers a remote-write client sends.
type capturingReceiver struct {
	mu      sync.Mutex
	bodies  [][]byte
	headers []http.Header
}

func (c *capturingReceiver) handler(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	c.mu.Lock()
	c.bodies = append(c.bodies, body)
	c.headers = append(c.headers, r.Header.Clone())
	c.mu.Unlock()
	w.WriteHeader(http.StatusNoContent)
}

func metricRecord() output.MetricRecord {
	return output.MetricRecord{
		Name:     "http.requests",
		Type:     output.MetricTypeCounter,
		IntValue: i64(42),
		Metadata: output.MetricPointMetadata{
			Timestamp:  time.Unix(1700000000, 0),
			Attributes: map[string]string{"method": "get"},
		},
	}
}

func TestOutput_FlushPostsRemoteWriteV1(t *testing.T) {
	rcv := &capturingReceiver{}
	srv := httptest.NewServer(http.HandlerFunc(rcv.handler))
	defer srv.Close()

	o, err := New(srv.URL, "1.0", 1, 0, time.Hour, 5*time.Second, nil, embed.TelemetrySettings{}, zap.NewNop())
	require.NoError(t, err)

	require.NoError(t, o.WriteMetric(context.Background(), metricRecord()))
	require.NoError(t, o.Stop(context.Background()))

	rcv.mu.Lock()
	defer rcv.mu.Unlock()
	require.NotEmpty(t, rcv.bodies, "receiver got at least one POST")

	// Headers identify remote-write 1.0.
	h := rcv.headers[0]
	assert.Equal(t, "application/x-protobuf", h.Get("Content-Type"))
	assert.Equal(t, "snappy", h.Get("Content-Encoding"))
	assert.Equal(t, "0.1.0", h.Get("X-Prometheus-Remote-Write-Version"))

	raw, err := snappy.Decode(nil, rcv.bodies[0])
	require.NoError(t, err)
	var req prompb.WriteRequest
	require.NoError(t, req.Unmarshal(raw))
	require.Len(t, req.Timeseries, 1)

	lm := map[string]string{}
	for _, l := range req.Timeseries[0].Labels {
		lm[l.Name] = l.Value
	}
	assert.Equal(t, "http_requests_total", lm["__name__"])
	assert.Equal(t, "get", lm["method"])
	require.Len(t, req.Timeseries[0].Samples, 1)
	assert.Equal(t, float64(42), req.Timeseries[0].Samples[0].Value)
}

func TestOutput_LogsUnsupported(t *testing.T) {
	o, err := New("http://localhost:9090/api/v1/write", "1.0", 500, 0, time.Hour, time.Second, nil, embed.TelemetrySettings{}, zap.NewNop())
	require.NoError(t, err)
	defer o.Stop(context.Background())
	assert.ErrorIs(t, o.Write(context.Background(), output.LogRecord{}), output.ErrUnsupportedTelemetryType)
}

func TestOutput_SupportedTelemetryMetricsOnly(t *testing.T) {
	o := &Output{}
	assert.Equal(t, []string{"metrics"}, toStrings(o.SupportedTelemetry()))
}

func toStrings[T ~string](in []T) []string {
	out := make([]string, len(in))
	for i, v := range in {
		out[i] = string(v)
	}
	return out
}

// TestOutput_FlushPostsRemoteWriteV2Headers pins the remote-write 2.0
// request headers a v2 receiver requires.
func TestOutput_FlushPostsRemoteWriteV2Headers(t *testing.T) {
	rcv := &capturingReceiver{}
	srv := httptest.NewServer(http.HandlerFunc(rcv.handler))
	defer srv.Close()

	o, err := New(srv.URL, "2.0", 1, 0, time.Hour, 5*time.Second, nil, embed.TelemetrySettings{}, zap.NewNop())
	require.NoError(t, err)
	require.NoError(t, o.WriteMetric(context.Background(), metricRecord()))
	require.NoError(t, o.Stop(context.Background()))

	rcv.mu.Lock()
	defer rcv.mu.Unlock()
	require.NotEmpty(t, rcv.headers)
	h := rcv.headers[0]
	assert.Equal(t, "application/x-protobuf;proto=io.prometheus.write.v2.Request", h.Get("Content-Type"))
	assert.Equal(t, "snappy", h.Get("Content-Encoding"))
	assert.Equal(t, "2.0.0", h.Get("X-Prometheus-Remote-Write-Version"))
}

// TestNewAcceptsBareMajorVersions pins that "1" and "2" (an unquoted YAML 1.0
// or 2.0 after viper) select the same protocols as "1.0" and "2.0".
func TestNewAcceptsBareMajorVersions(t *testing.T) {
	for ver, want := range map[string]version{"1": versionV1, "2": versionV2} {
		o, err := New("http://127.0.0.1:9090/api/v1/write", ver, 1, 0, time.Hour, 5*time.Second, nil, embed.TelemetrySettings{}, zap.NewNop())
		require.NoError(t, err, ver)
		require.Equal(t, want, o.version, ver)
		require.NoError(t, o.Stop(context.Background()))
	}
}

// TestNewRejectsBatchLargerThanBuffer pins that New, like config validation,
// refuses a buffer smaller than a batch (defaults applied first): such a
// batch never fills, so series would drop and wait for Stop.
func TestNewRejectsBatchLargerThanBuffer(t *testing.T) {
	for _, c := range []struct{ batch, buffered int }{{500, 100}, {0, 100}, {20000, 0}} {
		_, err := New("http://127.0.0.1:9090/api/v1/write", "1.0", c.batch, c.buffered, time.Hour, 5*time.Second, nil, embed.TelemetrySettings{}, zap.NewNop())
		require.ErrorContains(t, err, "at least the batch size", "%+v", c)
	}
}

// TestNewCopiesHeaders pins that New keeps its own copy of headers: the
// sender reads them on every POST, so a caller changing its map afterwards
// must neither race with it nor change what is sent.
func TestNewCopiesHeaders(t *testing.T) {
	rcv := &capturingReceiver{}
	srv := httptest.NewServer(http.HandlerFunc(rcv.handler))
	defer srv.Close()

	headers := map[string]string{"X-Scope-OrgID": "tenant-a"}
	o, err := New(srv.URL, "1.0", 1, 0, time.Hour, 5*time.Second, headers, embed.TelemetrySettings{}, zap.NewNop())
	require.NoError(t, err)
	headers["X-Scope-OrgID"] = "tenant-b"
	require.NoError(t, o.WriteMetric(context.Background(), metricRecord()))
	require.NoError(t, o.Stop(context.Background()))

	rcv.mu.Lock()
	defer rcv.mu.Unlock()
	require.NotEmpty(t, rcv.headers)
	assert.Equal(t, "tenant-a", rcv.headers[0].Get("X-Scope-OrgID"))
}
