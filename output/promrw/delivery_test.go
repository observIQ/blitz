package promrw

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/observiq/blitz/embed"
	"github.com/observiq/blitz/generator/count"
	"github.com/observiq/blitz/internal/counttest"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func writeHeldMetric(t *testing.T, o *Output) count.Permit {
	t.Helper()
	ctx, p := counttest.Held(t)
	require.NoError(t, o.WriteMetric(ctx, metricRecord()))
	p.Commit()
	return p
}

// TestOutput_PermitSettlesOnAcceptedPost pins that a buffered series counts
// once the endpoint accepts its batch.
func TestOutput_PermitSettlesOnAcceptedPost(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc((&capturingReceiver{}).handler))
	defer srv.Close()
	o, err := New(srv.URL, "1.0", 1, 0, time.Hour, 5*time.Second, nil, embed.TelemetrySettings{}, zap.NewNop())
	require.NoError(t, err)
	defer func() { _ = o.Stop(context.Background()) }()

	p := writeHeldMetric(t, o)
	require.True(t, counttest.Settled(p))
	require.True(t, p.Delivered())
}

// TestOutput_RejectedPostReleasesPermit pins that a batch the endpoint
// rejects is not counted.
func TestOutput_RejectedPostReleasesPermit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()
	o, err := New(srv.URL, "1.0", 1, 0, time.Hour, 5*time.Second, nil, embed.TelemetrySettings{}, zap.NewNop())
	require.NoError(t, err)
	defer func() { _ = o.Stop(context.Background()) }()

	p := writeHeldMetric(t, o)
	require.True(t, counttest.Settled(p))
	require.False(t, p.Delivered())
}

// TestOutput_DroppedOldestReleasesPermit pins that a series the full buffer
// drops is not counted.
func TestOutput_DroppedOldestReleasesPermit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc((&capturingReceiver{}).handler))
	defer srv.Close()
	o, err := New(srv.URL, "1.0", 2, 2, time.Hour, 5*time.Second, nil, embed.TelemetrySettings{}, zap.NewNop())
	require.NoError(t, err)
	o.mu.Lock() // maxBuffered is guarded by mu
	o.maxBuffered = 1
	o.mu.Unlock()
	defer func() { _ = o.Stop(context.Background()) }()

	oldest := writeHeldMetric(t, o)
	writeHeldMetric(t, o)
	require.True(t, counttest.Settled(oldest))
	require.False(t, oldest.Delivered())
}
