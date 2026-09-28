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

	o, err := New(srv.URL, "1.0", 1, time.Hour, 5*time.Second, nil, embed.TelemetrySettings{}, zap.NewNop())
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
	o, err := New("http://localhost:9090/api/v1/write", "1.0", 500, time.Hour, time.Second, nil, embed.TelemetrySettings{}, zap.NewNop())
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
