package winevt

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/observiq/blitz/internal/winevt/templates"
	"github.com/observiq/blitz/output"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// mockWriter implements output.Writer for testing
type mockWriter struct {
	mu     sync.Mutex
	writes [][]byte
}

func newMockWriter() *mockWriter {
	return &mockWriter{
		writes: make([][]byte, 0),
	}
}

func (m *mockWriter) Write(ctx context.Context, data output.LogRecord) error {
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

func TestNew(t *testing.T) {
	logger := zaptest.NewLogger(t)
	g, err := New(logger, 2, 50*time.Millisecond)
	assert.NoError(t, err)
	assert.NotNil(t, g)
}

func TestWinevtGenerator_GeneratesAndWrites(t *testing.T) {
	logger := zaptest.NewLogger(t)
	writer := newMockWriter()
	g, err := New(logger, 2, 20*time.Millisecond)
	require.NoError(t, err)

	err = g.Start(writer)
	require.NoError(t, err)

	time.Sleep(120 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	err = g.Stop(ctx)
	require.NoError(t, err)

	writes := writer.getWrites()
	assert.Greater(t, len(writes), 0)

	// Verify the rendered XML includes an IP from our list in both places
	foundBoth := false
	for _, b := range writes {
		out := string(b)
		containsA := false
		containsB := false
		for _, ip := range templates.DefaultIPs {
			if strings.Contains(out, "Source Network Address:\t"+ip) {
				containsA = true
			}
			if strings.Contains(out, "<Data Name='IpAddress'>"+ip+"</Data>") {
				containsB = true
			}
		}
		if containsA && containsB {
			foundBoth = true
			break
		}
	}
	assert.True(t, foundBoth, "expected to find IP address in both message and EventData")
}
