package hec

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/goccy/go-json"
	"github.com/observiq/blitz/embed"
	"github.com/observiq/blitz/generator/count"
	"github.com/observiq/blitz/internal/config"
	"github.com/observiq/blitz/internal/counttest"
	"github.com/observiq/blitz/output"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func newDeliveryHEC(t *testing.T, url string, ack bool, opts ...Option) *HEC {
	t.Helper()
	host, port := parseServerURL(t, url)
	base := []Option{WithHost(host), WithPort(port), WithToken("tok"), WithWorkers(1),
		WithBatchSize(1), WithEnableTLS(false), WithEnableACK(ack)}
	h, err := New(zap.NewNop(), append(base, opts...)...)
	require.NoError(t, err)
	return h
}

func writeHeld(t *testing.T, h *HEC) func() bool {
	t.Helper()
	ctx, p := counttest.Held(t)
	require.NoError(t, h.WriteLog(ctx, output.LogRecord{Message: "x", Metadata: output.LogRecordMetadata{Timestamp: time.Now()}}))
	p.Commit()
	return func() bool {
		require.True(t, counttest.Settled(p))
		return p.Delivered()
	}
}

// TestHEC_PermitSettlesOnAcceptedPost pins that without ACKs a record counts
// once HEC accepts its batch.
func TestHEC_PermitSettlesOnAcceptedPost(t *testing.T) {
	server, _, _ := mockHECServer(t)
	defer server.Close()
	h := newDeliveryHEC(t, server.URL, false)
	defer func() { _ = h.Stop(context.Background()) }()

	require.True(t, writeHeld(t, h)())
}

// TestHEC_RejectedPostReleasesPermit pins that a batch HEC rejects is not
// counted.
func TestHEC_RejectedPostReleasesPermit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(hecResponse{Text: "Invalid token", Code: 4})
	}))
	defer server.Close()
	h := newDeliveryHEC(t, server.URL, false)
	defer func() { _ = h.Stop(context.Background()) }()

	require.False(t, writeHeld(t, h)())
}

// TestHEC_Non2xxWithoutCodeReleasesPermit pins that an error status counts
// as a failure even when the body has no HEC error code, as from a proxy.
func TestHEC_Non2xxWithoutCodeReleasesPermit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`{"message":"bad gateway"}`))
	}))
	defer server.Close()
	h := newDeliveryHEC(t, server.URL, false)
	defer func() { _ = h.Stop(context.Background()) }()

	require.False(t, writeHeld(t, h)())
}

// TestHEC_UndecodableResponseReleasesPermit pins that a response body that is
// not HEC JSON, such as a proxy's HTML page, counts as a failure whatever its
// status.
func TestHEC_UndecodableResponseReleasesPermit(t *testing.T) {
	for _, status := range []int{http.StatusOK, http.StatusBadGateway} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = io.ReadAll(r.Body)
				w.WriteHeader(status)
				_, _ = w.Write([]byte("<html>proxy</html>"))
			}))
			defer server.Close()
			h := newDeliveryHEC(t, server.URL, false)
			defer func() { _ = h.Stop(context.Background()) }()

			require.False(t, writeHeld(t, h)())
		})
	}
}

// TestHEC_TransportFailureReleasesPermit pins that a batch that never
// reaches HEC is not counted.
func TestHEC_TransportFailureReleasesPermit(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	url := server.URL
	server.Close()
	h := newDeliveryHEC(t, url, false)
	defer func() { _ = h.Stop(context.Background()) }()

	require.False(t, writeHeld(t, h)())
}

// TestHEC_PermitWaitsForACK pins that with ACKs a record counts only once
// Splunk confirms its batch was indexed.
func TestHEC_PermitWaitsForACK(t *testing.T) {
	server, state := mockACKServer(t)
	defer server.Close()
	h := newDeliveryHEC(t, server.URL, true, WithACKPollInterval(20*time.Millisecond), WithACKTimeout(30*time.Second))
	defer func() { _ = h.Stop(context.Background()) }()

	ctx, p := counttest.Held(t)
	require.NoError(t, h.WriteLog(ctx, output.LogRecord{Message: "x", Metadata: output.LogRecordMetadata{Timestamp: time.Now()}}))
	p.Commit()
	require.Eventually(t, func() bool { return h.workers[0].tracker.size() == 1 }, 5*time.Second, 5*time.Millisecond)
	counttest.StaysPending(t, p, "posted but not yet indexed")

	state.setACKStatus(0, true)
	require.True(t, counttest.Settled(p))
	require.True(t, p.Delivered())
}

// TestHEC_ACKMaxRetriesReleasesPermit pins that a batch dropped after its
// retries is not counted.
func TestHEC_ACKMaxRetriesReleasesPermit(t *testing.T) {
	server, _ := mockACKServer(t)
	defer server.Close()
	h := newDeliveryHEC(t, server.URL, true, WithACKPollInterval(20*time.Millisecond),
		WithACKTimeout(50*time.Millisecond), WithMaxRetries(1))
	defer func() { _ = h.Stop(context.Background()) }()

	require.False(t, writeHeld(t, h)())
}

// TestHEC_UnconfirmedAtStopReleasesPermit pins that a batch still awaiting
// its ACK when the output stops is not counted.
func TestHEC_UnconfirmedAtStopReleasesPermit(t *testing.T) {
	server, _ := mockACKServer(t)
	defer server.Close()
	h := newDeliveryHEC(t, server.URL, true, WithACKPollInterval(20*time.Millisecond), WithACKTimeout(30*time.Second))

	ctx, p := counttest.Held(t)
	require.NoError(t, h.WriteLog(ctx, output.LogRecord{Message: "x", Metadata: output.LogRecordMetadata{Timestamp: time.Now()}}))
	p.Commit()
	require.Eventually(t, func() bool { return h.workers[0].tracker.size() == 1 }, 5*time.Second, 5*time.Millisecond)

	require.NoError(t, h.Stop(context.Background()))
	require.True(t, counttest.Settled(p))
	require.False(t, p.Delivered())
}

func heldSends(t *testing.T) (count.Sends, count.Permit) {
	t.Helper()
	ctx, p := counttest.Held(t)
	s := count.Hold(ctx)
	p.Commit()
	return count.Sends{s}, p
}

// TestHEC_WriteLogUnqueuedReleasesPermit pins that a record that never
// reaches the queue is released.
func TestHEC_WriteLogUnqueuedReleasesPermit(t *testing.T) {
	counttest.ReleasesUnqueued(t, func(writeCtx, outputCtx context.Context) error {
		h := &HEC{dataChan: make(chan hecItem), ctx: outputCtx}
		return h.WriteLog(writeCtx, output.LogRecord{Message: "x"})
	})
}

// TestHEC_ResendSettlesOnResult pins how a resent batch settles: failures
// release it, an accepted batch without ACK counts, and one with a new ackId
// waits for that ACK.
func TestHEC_ResendSettlesOnResult(t *testing.T) {
	closed := httptest.NewServer(http.NotFoundHandler())
	closed.Close()
	for name, tc := range map[string]struct {
		url       string
		status    int
		resp      string
		delivered bool
		pending   bool
	}{
		"transport failure": {url: closed.URL},
		"HEC error code":    {status: http.StatusBadRequest, resp: `{"text":"Invalid data format","code":6}`},
		"accepted":          {status: http.StatusOK, resp: `{"text":"Success","code":0}`, delivered: true},
		"new ackId":         {status: http.StatusOK, resp: `{"text":"Success","code":0,"ackId":7}`, pending: true},
	} {
		t.Run(name, func(t *testing.T) {
			url := tc.url
			if url == "" {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					_, _ = io.ReadAll(r.Body)
					w.WriteHeader(tc.status)
					_, _ = w.Write([]byte(tc.resp))
				}))
				defer server.Close()
				url = server.URL
			}
			m, err := newHECMetrics(embed.TelemetrySettings{})
			require.NoError(t, err)
			w := &worker{logger: zap.NewNop(), cfg: Config{token: "tok", enableACK: true}, httpClient: http.DefaultClient,
				baseURL: url, channelID: "c", metrics: m, tracker: newACKTracker(),
				resendCh: make(chan resendItem, 1), resendsDone: make(chan struct{})}
			sends, p := heldSends(t)
			w.resendCh <- resendItem{payload: []byte(`{"event":"x"}`), retryCount: 1, sends: sends}
			close(w.resendCh)
			w.processResends()

			if tc.pending {
				counttest.StaysPending(t, p)
				require.Equal(t, 1, w.tracker.size())
				return
			}
			require.True(t, counttest.Settled(p))
			require.Equal(t, tc.delivered, p.Delivered())
		})
	}
}

// TestHEC_ResendQueueFullReleasesPermit pins that an expired batch with no
// room to resend is released.
func TestHEC_ResendQueueFullReleasesPermit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.ReadAll(r.Body)
		_, _ = w.Write([]byte(`{"acks":{}}`))
	}))
	defer server.Close()
	m, err := newHECMetrics(embed.TelemetrySettings{})
	require.NoError(t, err)
	tracker := newACKTracker()
	p := newACKPoller(zap.NewNop(), tracker, http.DefaultClient, server.URL, "tok", "c",
		time.Hour, time.Nanosecond, 3, make(chan resendItem), m, embed.TelemetrySettings{})
	sends, permit := heldSends(t)
	tracker.track(1, []byte(`{"event":"x"}`), sends)

	p.poll()
	require.True(t, counttest.Settled(permit))
	require.False(t, permit.Delivered())
}

// TestHEC_EncodeFailureReleasesPermit pins that a batch that cannot be
// encoded is released.
func TestHEC_EncodeFailureReleasesPermit(t *testing.T) {
	m, err := newHECMetrics(embed.TelemetrySettings{})
	require.NoError(t, err)
	w := &worker{logger: zap.NewNop(), cfg: Config{eventFormat: config.HECEventFormatParsed}, metrics: m}
	sends, p := heldSends(t)
	unencodable := func(string) (map[string]any, error) { return map[string]any{"c": make(chan int)}, nil }

	w.sendBatch([]output.LogRecord{{Message: "x", ParseFunc: unencodable}}, sends)
	require.True(t, counttest.Settled(p))
	require.False(t, p.Delivered())
}

// TestHEC_FailedACKQueryStillExpires pins that a batch whose ACK query keeps
// failing still times out, so it is resent or dropped instead of holding its
// permits forever.
func TestHEC_FailedACKQueryStillExpires(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()
	m, err := newHECMetrics(embed.TelemetrySettings{})
	require.NoError(t, err)
	tracker := newACKTracker()
	p := newACKPoller(zap.NewNop(), tracker, http.DefaultClient, server.URL, "tok", "c",
		time.Hour, time.Nanosecond, 0, make(chan resendItem, 1), m, embed.TelemetrySettings{})
	sends, permit := heldSends(t)
	tracker.track(1, []byte(`{"event":"x"}`), sends)

	p.poll()
	require.True(t, counttest.Settled(permit), "the expired batch was dropped after max retries")
	require.False(t, permit.Delivered())
}
