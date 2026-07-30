package kubernetes

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/observiq/blitz/embed"
	"github.com/observiq/blitz/generator/count"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// Compile-time assertion: the migrated generator satisfies embed.ProducerModule.
var _ embed.ProducerModule = (*Generator)(nil)

type mockWriter struct {
	mu     sync.Mutex
	writes [][]byte
}

func newMockWriter() *mockWriter {
	return &mockWriter{writes: make([][]byte, 0)}
}

func (m *mockWriter) ConsumeLogs(_ context.Context, records []embed.LogRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range records {
		m.writes = append(m.writes, append([]byte(nil), records[i].Message...))
	}
	return nil
}

func (m *mockWriter) getWrites() [][]byte {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([][]byte(nil), m.writes...)
}

func TestGenerator_Name(t *testing.T) {
	logger := zaptest.NewLogger(t)
	gen, err := New(logger, 1, 50*time.Millisecond, "cri-o", newMockWriter())
	require.NoError(t, err)
	assert.Equal(t, componentName, gen.Name())
}

func TestGenerator_NilConsumer(t *testing.T) {
	logger := zaptest.NewLogger(t)
	gen, err := New(logger, 1, 50*time.Millisecond, "cri-o", nil)
	assert.Error(t, err)
	assert.Nil(t, gen)
	assert.Contains(t, err.Error(), "consumer cannot be nil")
}

func TestGenerator_SetCountTracker(t *testing.T) {
	logger := zaptest.NewLogger(t)
	gen, err := New(logger, 1, 50*time.Millisecond, "cri-o", newMockWriter())
	require.NoError(t, err)

	assert.Nil(t, gen.tracker, "tracker should be nil initially")

	tracker := count.NewTracker(10)
	gen.SetCountTracker(tracker)
	assert.Equal(t, tracker, gen.tracker)
}

func TestGenerator_CountLimited(t *testing.T) {
	logger := zaptest.NewLogger(t)
	writer := newMockWriter()

	gen, err := New(logger, 2, 10*time.Millisecond, "cri-o", writer)
	require.NoError(t, err)

	tracker := count.NewTracker(5)
	gen.SetCountTracker(tracker)

	err = gen.Start(context.Background())
	require.NoError(t, err)

	select {
	case <-tracker.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("tracker should have been exhausted")
	}

	// After tracker exhaustion, no additional records should be produced.
	// Assert the bound holds across a short window instead of sleeping.
	require.Never(t, func() bool {
		return len(writer.getWrites()) > 5
	}, 100*time.Millisecond, 10*time.Millisecond, "tracker should have halted further writes")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err = gen.Stop(ctx)
	require.NoError(t, err)

	writes := writer.getWrites()
	assert.Equal(t, 5, len(writes), "Expected exactly 5 logs with count tracker")
}
