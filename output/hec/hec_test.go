package hec

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/goccy/go-json"
	"github.com/observiq/blitz/output"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

// mockHECServer creates a test HTTP server that simulates a Splunk HEC endpoint.
// It returns the server, a channel that receives batches of raw request bodies,
// and a function to set the response.
func mockHECServer(t *testing.T) (*httptest.Server, chan []byte, func(int, hecResponse)) {
	t.Helper()

	bodies := make(chan []byte, 100)
	var respCode atomic.Int32
	var respBody atomic.Value

	respCode.Store(200)
	respBody.Store(hecResponse{Text: "Success", Code: 0})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("failed to read request body: %v", err)
			return
		}
		bodies <- body

		w.WriteHeader(int(respCode.Load()))
		resp := respBody.Load().(hecResponse)
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Errorf("failed to encode response: %v", err)
		}
	}))

	setResponse := func(code int, resp hecResponse) {
		respCode.Store(int32(code))
		respBody.Store(resp)
	}

	return server, bodies, setResponse
}

func parseServerURL(t *testing.T, url string) (string, string) {
	t.Helper()
	// url is like http://127.0.0.1:PORT
	// We need host and port separately
	host := url[len("http://"):]
	// Split host:port
	for i := len(host) - 1; i >= 0; i-- {
		if host[i] == ':' {
			return host[:i], host[i+1:]
		}
	}
	t.Fatalf("could not parse URL: %s", url)
	return "", ""
}

func TestHEC_NewValidation(t *testing.T) {
	logger := zaptest.NewLogger(t)

	t.Run("nil logger", func(t *testing.T) {
		_, err := New(nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "logger cannot be nil")
	})

	t.Run("missing host", func(t *testing.T) {
		_, err := New(logger, WithPort("8088"), WithToken("tok"))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "host cannot be empty")
	})

	t.Run("missing port", func(t *testing.T) {
		_, err := New(logger, WithHost("localhost"), WithToken("tok"))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "port cannot be empty")
	})

	t.Run("missing token", func(t *testing.T) {
		_, err := New(logger, WithHost("localhost"), WithPort("8088"))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "token cannot be empty")
	})
}

func TestHEC_WriteAndBatchBySize(t *testing.T) {
	server, bodies, _ := mockHECServer(t)
	defer server.Close()

	host, port := parseServerURL(t, server.URL)
	logger := zaptest.NewLogger(t)

	h, err := New(logger,
		WithHost(host),
		WithPort(port),
		WithToken("test-token"),
		WithWorkers(1),
		WithBatchSize(3),
		WithBatchTimeout(10*time.Second), // Long timeout — batch should trigger by size
		WithEnableTLS(false),
		WithEnableACK(false),
	)
	require.NoError(t, err)

	ctx := context.Background()

	// Send exactly batchSize records
	for i := range 3 {
		err := h.Write(ctx, output.LogRecord{
			Message: "test message " + string(rune('0'+i)),
			Metadata: output.LogRecordMetadata{
				Timestamp: time.Now(),
			},
		})
		require.NoError(t, err)
	}

	// Wait for the batch to be sent
	select {
	case body := <-bodies:
		assert.NotEmpty(t, body)
		// The body should contain 3 JSON objects (one per line from the encoder)
		lines := 0
		for _, b := range body {
			if b == '\n' {
				lines++
			}
		}
		assert.Equal(t, 3, lines, "expected 3 events in batch")
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for batch")
	}

	require.NoError(t, h.Stop(ctx))
}

func TestHEC_WriteAndBatchByTimeout(t *testing.T) {
	server, bodies, _ := mockHECServer(t)
	defer server.Close()

	host, port := parseServerURL(t, server.URL)
	logger := zaptest.NewLogger(t)

	h, err := New(logger,
		WithHost(host),
		WithPort(port),
		WithToken("test-token"),
		WithWorkers(1),
		WithBatchSize(100),                     // Large batch size — won't trigger by size
		WithBatchTimeout(200*time.Millisecond), // Short timeout
		WithEnableTLS(false),
		WithEnableACK(false),
	)
	require.NoError(t, err)

	ctx := context.Background()

	// Send fewer events than batch size
	err = h.Write(ctx, output.LogRecord{
		Message:  "timeout test",
		Metadata: output.LogRecordMetadata{Timestamp: time.Now()},
	})
	require.NoError(t, err)

	// Wait for the timeout-triggered flush
	select {
	case body := <-bodies:
		assert.NotEmpty(t, body)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for timeout-triggered batch")
	}

	require.NoError(t, h.Stop(ctx))
}

func TestHEC_AuthHeader(t *testing.T) {
	var gotAuth string
	var gotChannel string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotChannel = r.Header.Get("X-Splunk-Request-Channel")

		resp := hecResponse{Text: "Success", Code: 0}
		w.WriteHeader(200)
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	host, port := parseServerURL(t, server.URL)
	logger := zaptest.NewLogger(t)

	t.Run("with ACK enabled sends channel header", func(t *testing.T) {
		gotAuth = ""
		gotChannel = ""

		h, err := New(logger,
			WithHost(host),
			WithPort(port),
			WithToken("my-secret-token"),
			WithWorkers(1),
			WithBatchSize(1),
			WithEnableTLS(false),
			WithEnableACK(true),
		)
		require.NoError(t, err)

		ctx := context.Background()
		err = h.Write(ctx, output.LogRecord{
			Message:  "test",
			Metadata: output.LogRecordMetadata{Timestamp: time.Now()},
		})
		require.NoError(t, err)

		time.Sleep(500 * time.Millisecond)

		assert.Equal(t, "Splunk my-secret-token", gotAuth)
		assert.NotEmpty(t, gotChannel, "expected X-Splunk-Request-Channel header when ACK enabled")

		require.NoError(t, h.Stop(ctx))
	})

	t.Run("with ACK disabled omits channel header", func(t *testing.T) {
		gotAuth = ""
		gotChannel = ""

		h, err := New(logger,
			WithHost(host),
			WithPort(port),
			WithToken("my-secret-token"),
			WithWorkers(1),
			WithBatchSize(1),
			WithEnableTLS(false),
			WithEnableACK(false),
		)
		require.NoError(t, err)

		ctx := context.Background()
		err = h.Write(ctx, output.LogRecord{
			Message:  "test",
			Metadata: output.LogRecordMetadata{Timestamp: time.Now()},
		})
		require.NoError(t, err)

		time.Sleep(500 * time.Millisecond)

		assert.Equal(t, "Splunk my-secret-token", gotAuth)
		assert.Empty(t, gotChannel, "expected no X-Splunk-Request-Channel header when ACK disabled")

		require.NoError(t, h.Stop(ctx))
	})
}

func TestHEC_EventFormatRaw(t *testing.T) {
	var receivedBody []byte

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedBody, _ = io.ReadAll(r.Body)
		resp := hecResponse{Text: "Success", Code: 0}
		w.WriteHeader(200)
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	host, port := parseServerURL(t, server.URL)
	logger := zaptest.NewLogger(t)

	h, err := New(logger,
		WithHost(host),
		WithPort(port),
		WithToken("tok"),
		WithWorkers(1),
		WithBatchSize(1),
		WithEventFormat("raw"),
		WithEnableTLS(false),
		WithEnableACK(false),
	)
	require.NoError(t, err)

	ctx := context.Background()
	err = h.Write(ctx, output.LogRecord{
		Message:  `{"key":"value"}`,
		Metadata: output.LogRecordMetadata{Timestamp: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)},
	})
	require.NoError(t, err)

	time.Sleep(500 * time.Millisecond)

	var event hecEvent
	require.NoError(t, json.Unmarshal(receivedBody, &event))
	assert.Equal(t, `{"key":"value"}`, event.Event)

	require.NoError(t, h.Stop(ctx))
}

func TestHEC_EventFormatParsed(t *testing.T) {
	var receivedBody []byte

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedBody, _ = io.ReadAll(r.Body)
		resp := hecResponse{Text: "Success", Code: 0}
		w.WriteHeader(200)
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	host, port := parseServerURL(t, server.URL)
	logger := zaptest.NewLogger(t)

	h, err := New(logger,
		WithHost(host),
		WithPort(port),
		WithToken("tok"),
		WithWorkers(1),
		WithBatchSize(1),
		WithEventFormat("parsed"),
		WithEnableTLS(false),
		WithEnableACK(false),
	)
	require.NoError(t, err)

	ctx := context.Background()
	err = h.Write(ctx, output.LogRecord{
		Message: `original message`,
		ParseFunc: func(msg string) (map[string]any, error) {
			return map[string]any{"parsed": true, "msg": msg}, nil
		},
		Metadata: output.LogRecordMetadata{Timestamp: time.Now()},
	})
	require.NoError(t, err)

	time.Sleep(500 * time.Millisecond)

	var event hecEvent
	require.NoError(t, json.Unmarshal(receivedBody, &event))
	// In parsed mode, event should be a map
	eventMap, ok := event.Event.(map[string]any)
	require.True(t, ok, "expected event to be a map, got %T", event.Event)
	assert.Equal(t, true, eventMap["parsed"])
	assert.Equal(t, "original message", eventMap["msg"])

	require.NoError(t, h.Stop(ctx))
}

func TestHEC_EventFormatParsedFallback(t *testing.T) {
	var receivedBody []byte

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedBody, _ = io.ReadAll(r.Body)
		resp := hecResponse{Text: "Success", Code: 0}
		w.WriteHeader(200)
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	host, port := parseServerURL(t, server.URL)
	logger := zaptest.NewLogger(t)

	h, err := New(logger,
		WithHost(host),
		WithPort(port),
		WithToken("tok"),
		WithWorkers(1),
		WithBatchSize(1),
		WithEventFormat("parsed"),
		WithEnableTLS(false),
		WithEnableACK(false),
	)
	require.NoError(t, err)

	ctx := context.Background()
	// No ParseFunc — should fall back to raw
	err = h.Write(ctx, output.LogRecord{
		Message:  "raw fallback",
		Metadata: output.LogRecordMetadata{Timestamp: time.Now()},
	})
	require.NoError(t, err)

	time.Sleep(500 * time.Millisecond)

	var event hecEvent
	require.NoError(t, json.Unmarshal(receivedBody, &event))
	assert.Equal(t, "raw fallback", event.Event)

	require.NoError(t, h.Stop(ctx))
}

func TestHEC_HECErrorResponse(t *testing.T) {
	requestCount := atomic.Int32{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.ReadAll(r.Body)
		requestCount.Add(1)
		resp := hecResponse{Text: "Invalid token", Code: 4}
		w.WriteHeader(403)
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	host, port := parseServerURL(t, server.URL)
	logger := zaptest.NewLogger(t, zaptest.Level(zap.ErrorLevel))

	h, err := New(logger,
		WithHost(host),
		WithPort(port),
		WithToken("bad-token"),
		WithWorkers(1),
		WithBatchSize(1),
		WithEnableTLS(false),
		WithEnableACK(false),
	)
	require.NoError(t, err)

	ctx := context.Background()
	err = h.Write(ctx, output.LogRecord{
		Message:  "test",
		Metadata: output.LogRecordMetadata{Timestamp: time.Now()},
	})
	require.NoError(t, err)

	time.Sleep(500 * time.Millisecond)

	// Should have attempted the request (error is logged, not returned)
	assert.GreaterOrEqual(t, requestCount.Load(), int32(1))

	require.NoError(t, h.Stop(ctx))
}

func TestHEC_GracefulShutdown(t *testing.T) {
	batchCount := atomic.Int32{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.ReadAll(r.Body)
		batchCount.Add(1)
		resp := hecResponse{Text: "Success", Code: 0}
		w.WriteHeader(200)
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	host, port := parseServerURL(t, server.URL)
	logger := zaptest.NewLogger(t)

	h, err := New(logger,
		WithHost(host),
		WithPort(port),
		WithToken("tok"),
		WithWorkers(1),
		WithBatchSize(100), // Large batch — events will be flushed on shutdown
		WithBatchTimeout(10*time.Second),
		WithEnableTLS(false),
		WithEnableACK(false),
	)
	require.NoError(t, err)

	ctx := context.Background()

	// Write some events that won't fill a batch
	for range 5 {
		err := h.Write(ctx, output.LogRecord{
			Message:  "shutdown test",
			Metadata: output.LogRecordMetadata{Timestamp: time.Now()},
		})
		require.NoError(t, err)
	}

	// Stop should flush remaining events
	require.NoError(t, h.Stop(ctx))

	// Give a moment for the final flush
	time.Sleep(200 * time.Millisecond)

	assert.GreaterOrEqual(t, batchCount.Load(), int32(1), "expected at least one batch to be sent during shutdown")
}

func TestHEC_MultipleWorkers(t *testing.T) {
	requestCount := atomic.Int32{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.ReadAll(r.Body)
		requestCount.Add(1)
		resp := hecResponse{Text: "Success", Code: 0}
		w.WriteHeader(200)
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	host, port := parseServerURL(t, server.URL)
	logger := zaptest.NewLogger(t)

	h, err := New(logger,
		WithHost(host),
		WithPort(port),
		WithToken("tok"),
		WithWorkers(3),
		WithBatchSize(1),
		WithEnableTLS(false),
		WithEnableACK(false),
	)
	require.NoError(t, err)

	ctx := context.Background()

	// Send enough events for multiple workers to process
	for range 10 {
		err := h.Write(ctx, output.LogRecord{
			Message:  "multi worker test",
			Metadata: output.LogRecordMetadata{Timestamp: time.Now()},
		})
		require.NoError(t, err)
	}

	time.Sleep(1 * time.Second)

	assert.GreaterOrEqual(t, requestCount.Load(), int32(10), "expected at least 10 requests with batch size 1")

	require.NoError(t, h.Stop(ctx))
}

func TestGenerateChannelID(t *testing.T) {
	id1 := generateChannelID()
	id2 := generateChannelID()

	assert.NotEqual(t, id1, id2, "channel IDs should be unique")
	assert.Len(t, id1, 36, "channel ID should be a UUID string (36 chars)")
	// Basic UUID format check: 8-4-4-4-12
	assert.Equal(t, '-', rune(id1[8]))
	assert.Equal(t, '-', rune(id1[13]))
	assert.Equal(t, '-', rune(id1[18]))
	assert.Equal(t, '-', rune(id1[23]))
}
