package kubernetes

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/observiq/blitz/generator/count"
	"github.com/observiq/blitz/output"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

type mockWriter struct {
	mu     sync.Mutex
	writes [][]byte
}

func newMockWriter() *mockWriter {
	return &mockWriter{writes: make([][]byte, 0)}
}

func (m *mockWriter) Write(_ context.Context, data output.LogRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.writes = append(m.writes, append([]byte(nil), data.Message...))
	return nil
}

func (m *mockWriter) getWrites() [][]byte {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([][]byte(nil), m.writes...)
}

func TestGenerator_SetCountTracker(t *testing.T) {
	logger := zaptest.NewLogger(t)
	gen, err := New(logger, 1, 50*time.Millisecond, "cri-o")
	require.NoError(t, err)

	assert.Nil(t, gen.tracker, "tracker should be nil initially")

	tracker := count.NewTracker(10)
	gen.SetCountTracker(tracker)
	assert.Equal(t, tracker, gen.tracker)
}

func TestGenerator_CountLimited(t *testing.T) {
	logger := zaptest.NewLogger(t)
	writer := newMockWriter()

	gen, err := New(logger, 2, 10*time.Millisecond, "cri-o")
	require.NoError(t, err)

	tracker := count.NewTracker(5)
	gen.SetCountTracker(tracker)

	err = gen.Start(writer)
	require.NoError(t, err)

	select {
	case <-tracker.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("tracker should have been exhausted")
	}

	time.Sleep(100 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err = gen.Stop(ctx)
	require.NoError(t, err)

	writes := writer.getWrites()
	assert.Equal(t, 5, len(writes), "Expected exactly 5 logs with count tracker")
}
