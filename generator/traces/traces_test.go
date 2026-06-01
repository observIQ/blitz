package traces

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/observiq/blitz/generator/count"
	"github.com/observiq/blitz/output"
	"github.com/observiq/blitz/telemetry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

type mockTraceWriter struct {
	mu      sync.Mutex
	records []output.TraceRecord
}

func (m *mockTraceWriter) WriteTrace(_ context.Context, rec output.TraceRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.records = append(m.records, rec)
	return nil
}

func (m *mockTraceWriter) Records() []output.TraceRecord {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]output.TraceRecord, len(m.records))
	copy(result, m.records)
	return result
}

func TestNew(t *testing.T) {
	logger := zaptest.NewLogger(t)

	t.Run("valid", func(t *testing.T) {
		g, err := New(logger, 1, time.Second)
		require.NoError(t, err)
		assert.NotNil(t, g)
	})

	t.Run("nil logger", func(t *testing.T) {
		_, err := New(nil, 1, time.Second)
		require.Error(t, err)
	})

	t.Run("invalid workers", func(t *testing.T) {
		_, err := New(logger, 0, time.Second)
		require.Error(t, err)
	})
}

func TestSupportedTelemetry(t *testing.T) {
	logger := zaptest.NewLogger(t)
	g, err := New(logger, 1, time.Second)
	require.NoError(t, err)

	types := g.SupportedTelemetry()
	assert.Equal(t, []telemetry.Type{telemetry.Traces}, types)
}

func TestStartStop(t *testing.T) {
	logger := zaptest.NewLogger(t)
	writer := &mockTraceWriter{}

	g, err := New(logger, 1, 50*time.Millisecond)
	require.NoError(t, err)

	require.NoError(t, g.Start(writer))

	// Wait for at least one trace
	time.Sleep(150 * time.Millisecond)

	require.NoError(t, g.Stop(context.Background()))

	records := writer.Records()
	assert.NotEmpty(t, records, "should have generated traces")

	// Each trace has at least 2 spans (root + db child)
	assert.GreaterOrEqual(t, len(records), 2)
}

func TestTraceStructure(t *testing.T) {
	logger := zaptest.NewLogger(t)
	writer := &mockTraceWriter{}

	g, err := New(logger, 1, 50*time.Millisecond)
	require.NoError(t, err)

	require.NoError(t, g.Start(writer))
	time.Sleep(100 * time.Millisecond)
	require.NoError(t, g.Stop(context.Background()))

	records := writer.Records()
	require.NotEmpty(t, records)

	// Check first record (root span)
	root := records[0]
	assert.NotEmpty(t, root.TraceID)
	assert.NotEmpty(t, root.SpanID)
	assert.Empty(t, root.ParentSpanID, "root span should have no parent")
	assert.Equal(t, output.SpanKindServer, root.Kind)
	assert.False(t, root.StartTime.IsZero())
	assert.False(t, root.EndTime.IsZero())
	assert.True(t, root.EndTime.After(root.StartTime))
	assert.NotNil(t, root.Metadata.Attributes)

	// Check second record (DB child span)
	if len(records) >= 2 {
		child := records[1]
		assert.Equal(t, root.TraceID, child.TraceID, "child should share trace ID")
		assert.NotEmpty(t, child.ParentSpanID, "child should have parent")
		assert.Equal(t, root.SpanID, child.ParentSpanID)
		assert.Equal(t, output.SpanKindClient, child.Kind)
	}
}

func TestCountTracker(t *testing.T) {
	logger := zaptest.NewLogger(t)
	writer := &mockTraceWriter{}

	g, err := New(logger, 1, 50*time.Millisecond)
	require.NoError(t, err)

	tracker := count.NewTracker(2)
	g.SetCountTracker(tracker)

	require.NoError(t, g.Start(writer))

	select {
	case <-tracker.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("tracker should have completed")
	}

	require.NoError(t, g.Stop(context.Background()))
}

func TestGenerateTraceID(t *testing.T) {
	id := generateTraceID()
	assert.Len(t, id, 32) // 16 bytes = 32 hex chars
}

func TestGenerateSpanID(t *testing.T) {
	id := generateSpanID()
	assert.Len(t, id, 16) // 8 bytes = 16 hex chars
}
