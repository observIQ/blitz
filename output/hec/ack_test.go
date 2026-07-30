package hec

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/goccy/go-json"
	"github.com/observiq/blitz/output"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestACKTracker_TrackAndConfirm(t *testing.T) {
	tracker := newACKTracker()

	tracker.track(1, []byte("batch1"))
	tracker.track(2, []byte("batch2"))
	tracker.track(3, []byte("batch3"))

	assert.Equal(t, 3, tracker.size())

	ids := tracker.pendingIDs()
	assert.Len(t, ids, 3)

	count := tracker.confirm([]int64{1, 3})
	assert.Equal(t, 2, count)
	assert.Equal(t, 1, tracker.size())

	// Confirming already-removed IDs returns 0
	count = tracker.confirm([]int64{1, 3})
	assert.Equal(t, 0, count)
}

func TestACKTracker_Expired(t *testing.T) {
	tracker := newACKTracker()

	tracker.track(1, []byte("batch1"))
	tracker.track(2, []byte("batch2"))

	// Manually set sendTime to the past for ackId 1
	tracker.mu.Lock()
	tracker.pending[1].sendTime = time.Now().Add(-10 * time.Minute)
	tracker.mu.Unlock()

	expired := tracker.expired(5 * time.Minute)
	assert.Len(t, expired, 1)
	assert.Equal(t, []byte("batch1"), expired[0].payload)

	// ackId 1 should be removed, ackId 2 should remain
	assert.Equal(t, 1, tracker.size())
}

func TestACKTracker_ConcurrentAccess(t *testing.T) {
	tracker := newACKTracker()
	var wg sync.WaitGroup

	// Concurrent writes
	for i := range 100 {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			tracker.track(int64(id), []byte(fmt.Sprintf("batch%d", id)))
		}(i)
	}
	wg.Wait()

	assert.Equal(t, 100, tracker.size())

	// Concurrent confirms
	for i := range 50 {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			tracker.confirm([]int64{int64(id)})
		}(i)
	}
	wg.Wait()

	assert.Equal(t, 50, tracker.size())
}

// mockACKServer creates a server that handles both event and ack endpoints.
func mockACKServer(t *testing.T) (*httptest.Server, *mockACKState) {
	t.Helper()

	state := &mockACKState{
		ackStatuses: make(map[int64]bool),
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case eventEndpoint:
			io.ReadAll(r.Body)
			ackID := state.nextAckID.Add(1) - 1
			resp := hecResponse{
				Text:  "Success",
				Code:  0,
				AckID: &ackID,
			}
			w.WriteHeader(200)
			json.NewEncoder(w).Encode(resp)

		case ackEndpoint:
			var req ackRequest
			json.NewDecoder(r.Body).Decode(&req)

			state.mu.Lock()
			acks := make(map[string]bool)
			for _, id := range req.Acks {
				key := fmt.Sprintf("%d", id)
				if status, ok := state.ackStatuses[id]; ok {
					acks[key] = status
					if status {
						// Splunk deletes status after returning true
						delete(state.ackStatuses, id)
					}
				} else {
					acks[key] = false
				}
			}
			state.mu.Unlock()

			resp := ackResponse{Acks: acks}
			w.WriteHeader(200)
			json.NewEncoder(w).Encode(resp)

		default:
			w.WriteHeader(404)
		}
	}))

	return server, state
}

type mockACKState struct {
	nextAckID   atomic.Int64
	mu          sync.Mutex
	ackStatuses map[int64]bool
}

// setACKStatus sets the ACK status for a given ackId
func (s *mockACKState) setACKStatus(id int64, status bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ackStatuses[id] = status
}

func TestHEC_ACKConfirmFlow(t *testing.T) {
	server, state := mockACKServer(t)
	defer server.Close()

	host, port := parseServerURL(t, server.URL)
	logger := zaptest.NewLogger(t)

	h, err := New(logger,
		WithHost(host),
		WithPort(port),
		WithToken("tok"),
		WithWorkers(1),
		WithBatchSize(1),
		WithEnableTLS(false),
		WithEnableACK(true),
		WithACKPollInterval(200*time.Millisecond),
		WithACKTimeout(30*time.Second),
		WithMaxRetries(3),
	)
	require.NoError(t, err)

	ctx := t.Context()

	// Send an event — it will get ackId 0
	err = h.Write(ctx, output.LogRecord{
		Message:  "ack test",
		Metadata: output.LogRecordMetadata{Timestamp: time.Now()},
	})
	require.NoError(t, err)

	// Wait for the event to be sent and tracked
	require.Eventually(t, func() bool {
		return h.workers[0].tracker.size() == 1
	}, 5*time.Second, 10*time.Millisecond)
	assert.Equal(t, 1, h.workers[0].tracker.size(), "expected 1 pending ackId")

	// Confirm the ackId in the mock server
	state.setACKStatus(0, true)

	// Wait for the poller to pick it up and remove the confirmed ackId
	require.Eventually(t, func() bool {
		return h.workers[0].tracker.size() == 0
	}, 5*time.Second, 10*time.Millisecond)
	assert.Equal(t, 0, h.workers[0].tracker.size(), "expected 0 pending ackIds after confirmation")

	require.NoError(t, h.Stop(ctx))
}

func TestHEC_ACKExpireAndResend(t *testing.T) {
	server, state := mockACKServer(t)
	defer server.Close()

	host, port := parseServerURL(t, server.URL)
	logger := zaptest.NewLogger(t)

	h, err := New(logger,
		WithHost(host),
		WithPort(port),
		WithToken("tok"),
		WithWorkers(1),
		WithBatchSize(1),
		WithEnableTLS(false),
		WithEnableACK(true),
		WithACKPollInterval(100*time.Millisecond),
		WithACKTimeout(300*time.Millisecond), // Short timeout for test
		WithMaxRetries(5),
	)
	require.NoError(t, err)

	ctx := t.Context()

	// Send an event — gets ackId 0
	err = h.Write(ctx, output.LogRecord{
		Message:  "resend test",
		Metadata: output.LogRecordMetadata{Timestamp: time.Now()},
	})
	require.NoError(t, err)

	// Wait for event to be sent, ackId 0 to expire, and a resend to produce a
	// new ackId. The mock server auto-increments ackIds: the initial send is
	// ackId 0 (nextAckID -> 1); once a resend happens nextAckID reaches 2.
	require.Eventually(t, func() bool {
		return state.nextAckID.Load() >= 2
	}, 5*time.Second, 10*time.Millisecond)

	// Confirm ALL ackIds the server has seen so far (one of them will be the current pending)
	// The server auto-increments ackIds, so let's confirm a range
	for i := range int(state.nextAckID.Load()) + 1 {
		state.setACKStatus(int64(i), true)
	}

	// Wait for the poller to pick up the confirmation
	require.Eventually(t, func() bool {
		return h.workers[0].tracker.size() == 0
	}, 5*time.Second, 10*time.Millisecond)
	assert.Equal(t, 0, h.workers[0].tracker.size(), "expected 0 pending after resend+confirm")

	require.NoError(t, h.Stop(ctx))
}

func TestHEC_ACKMaxRetriesDrop(t *testing.T) {
	server, _ := mockACKServer(t)
	defer server.Close()

	host, port := parseServerURL(t, server.URL)
	logger := zaptest.NewLogger(t)

	h, err := New(logger,
		WithHost(host),
		WithPort(port),
		WithToken("tok"),
		WithWorkers(1),
		WithBatchSize(1),
		WithEnableTLS(false),
		WithEnableACK(true),
		WithACKPollInterval(50*time.Millisecond),
		WithACKTimeout(100*time.Millisecond), // Very short
		WithMaxRetries(1),                    // Only 1 retry
	)
	require.NoError(t, err)

	ctx := t.Context()

	// Send an event
	err = h.Write(ctx, output.LogRecord{
		Message:  "drop test",
		Metadata: output.LogRecordMetadata{Timestamp: time.Now()},
	})
	require.NoError(t, err)

	// First wait for the event to be sent and tracked, so the subsequent
	// wait-for-drop cannot pass vacuously against an empty tracker.
	require.Eventually(t, func() bool {
		return h.workers[0].tracker.size() == 1
	}, 5*time.Second, 10*time.Millisecond)

	// After initial send, expiry, resend, second expiry, and drop
	// (50ms poll + 100ms timeout), maxRetries is exhausted and the batch
	// is dropped, leaving the tracker empty.
	require.Eventually(t, func() bool {
		return h.workers[0].tracker.size() == 0
	}, 5*time.Second, 10*time.Millisecond)
	assert.Equal(t, 0, h.workers[0].tracker.size(), "expected 0 pending after max retries drop")

	require.NoError(t, h.Stop(ctx))
}

func TestHEC_ACKDisabledNoTracker(t *testing.T) {
	server, _ := mockACKServer(t)
	defer server.Close()

	host, port := parseServerURL(t, server.URL)
	logger := zaptest.NewLogger(t)

	h, err := New(logger,
		WithHost(host),
		WithPort(port),
		WithToken("tok"),
		WithWorkers(1),
		WithBatchSize(1),
		WithEnableTLS(false),
		WithEnableACK(false),
	)
	require.NoError(t, err)

	// When ACK is disabled, tracker should be nil
	assert.Nil(t, h.workers[0].tracker)
	assert.Nil(t, h.workers[0].poller)

	ctx := t.Context()
	err = h.Write(ctx, output.LogRecord{
		Message:  "no ack",
		Metadata: output.LogRecordMetadata{Timestamp: time.Now()},
	})
	require.NoError(t, err)

	// Stop flushes any pending event; no state to wait on beforehand.
	require.NoError(t, h.Stop(ctx))
}

func TestHEC_ACKPerWorkerChannels(t *testing.T) {
	server, _ := mockACKServer(t)
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
		WithEnableACK(true),
		WithACKPollInterval(1*time.Second),
		WithACKTimeout(30*time.Second),
	)
	require.NoError(t, err)

	// Each worker should have its own channel UUID
	channels := make(map[string]bool)
	for _, w := range h.workers {
		assert.NotEmpty(t, w.channelID)
		channels[w.channelID] = true
		// Each worker should have its own tracker
		assert.NotNil(t, w.tracker)
	}
	assert.Len(t, channels, 3, "expected 3 unique channel UUIDs")

	require.NoError(t, h.Stop(t.Context()))
}
