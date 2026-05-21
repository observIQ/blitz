package postgres

import (
	"context"
	"errors"
	"regexp"
	"strings"
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

// mockWriter implements embed.LogConsumer for testing.
type mockWriter struct {
	mu       sync.Mutex
	writes   [][]byte
	errors   []error
	writeErr error
	delay    time.Duration
}

func newMockWriter() *mockWriter {
	return &mockWriter{
		writes: make([][]byte, 0),
		errors: make([]error, 0),
	}
}

func (m *mockWriter) ConsumeLogs(ctx context.Context, records []embed.LogRecord) error {
	if m.delay > 0 {
		select {
		case <-time.After(m.delay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.writeErr != nil {
		err := m.writeErr
		m.errors = append(m.errors, err)
		return err
	}

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

func (m *mockWriter) getErrors() []error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]error(nil), m.errors...)
}

func (m *mockWriter) setWriteError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.writeErr = err
}

func (m *mockWriter) setDelay(delay time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.delay = delay
}

func TestNew(t *testing.T) {
	logger := zaptest.NewLogger(t)
	workers := 5
	rate := 100 * time.Millisecond

	generator, err := New(logger, workers, rate, newMockWriter())

	assert.NoError(t, err)
	assert.NotNil(t, generator)
	assert.Equal(t, logger, generator.logger)
	assert.Equal(t, workers, generator.workers)
	assert.Equal(t, rate, generator.rate)
	assert.NotNil(t, generator.stopCh)
}

func TestNew_NilLogger(t *testing.T) {
	generator, err := New(nil, 5, 100*time.Millisecond, newMockWriter())

	assert.Error(t, err)
	assert.Nil(t, generator)
	assert.Contains(t, err.Error(), "logger cannot be nil")
}

func TestNew_InvalidWorkers(t *testing.T) {
	logger := zaptest.NewLogger(t)

	generator, err := New(logger, 0, 100*time.Millisecond, newMockWriter())
	assert.Error(t, err)
	assert.Nil(t, generator)
	assert.Contains(t, err.Error(), "workers must be 1 or greater")

	generator, err = New(logger, -1, 100*time.Millisecond, newMockWriter())
	assert.Error(t, err)
	assert.Nil(t, generator)
	assert.Contains(t, err.Error(), "workers must be 1 or greater")
}

func TestPostgresGenerator_Start(t *testing.T) {
	logger := zaptest.NewLogger(t)
	writer := newMockWriter()
	generator, err := New(logger, 2, 50*time.Millisecond, writer)
	require.NoError(t, err)

	err = generator.Start(context.Background())
	assert.NoError(t, err)

	require.Eventually(t, func() bool {
		return len(writer.getWrites()) > 0
	}, 2*time.Second, 10*time.Millisecond, "Expected some logs to be written")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err = generator.Stop(ctx)
	assert.NoError(t, err)

	writes := writer.getWrites()

	// PostgreSQL log format: timestamp [process_id]: user=...,db=...,app=...,client=... <severity>: <message>
	postgresPattern := regexp.MustCompile(`^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}\.\d{3} [A-Z]+ \[\d+\]: user=[^,]+,db=[^,]+,app=[^,]+,client=[^,]+,session=[^,]+,vxid=[^,]+,txid=[^,]+,line=\d+ (LOG|ERROR|FATAL|PANIC|WARNING|NOTICE|DEBUG|INFO):  .+$`)
	for _, write := range writes {
		line := string(write)
		assert.True(t, postgresPattern.MatchString(line), "Log should match PostgreSQL log format: %s", line)
		assert.Contains(t, line, "user=", "Log should contain user field")
		assert.Contains(t, line, "db=", "Log should contain database field")
		assert.Contains(t, line, "client=", "Log should contain client field")
	}
}

func TestPostgresGenerator_Stop_GracefulShutdown(t *testing.T) {
	logger := zaptest.NewLogger(t)
	writer := newMockWriter()
	generator, err := New(logger, 3, 10*time.Millisecond, writer)
	require.NoError(t, err)

	err = generator.Start(context.Background())
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return len(writer.getWrites()) > 0
	}, 2*time.Second, 5*time.Millisecond, "Expected some logs to be written before stopping")

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	start := time.Now()
	err = generator.Stop(ctx)
	duration := time.Since(start)

	assert.NoError(t, err)
	assert.Less(t, duration, 500*time.Millisecond, "Stop should complete quickly")
}

func TestPostgresGenerator_WriteErrors_Backoff(t *testing.T) {
	logger := zaptest.NewLogger(t)
	writer := newMockWriter()
	writer.setWriteError(errors.New("write failed"))
	generator, err := New(logger, 1, 10*time.Millisecond, writer)
	require.NoError(t, err)

	err = generator.Start(context.Background())
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return len(writer.getErrors()) > 0
	}, 2*time.Second, 10*time.Millisecond, "Expected some write errors")

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	err = generator.Stop(ctx)
	assert.NoError(t, err)
}

func TestPostgresGenerator_ConcurrentWorkers(t *testing.T) {
	logger := zaptest.NewLogger(t)
	writer := newMockWriter()
	generator, err := New(logger, 5, 20*time.Millisecond, writer)
	require.NoError(t, err)

	err = generator.Start(context.Background())
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return len(writer.getWrites()) > 10
	}, 2*time.Second, 10*time.Millisecond, "Expected many logs from multiple workers")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	err = generator.Stop(ctx)
	assert.NoError(t, err)
}

func TestFormatAsPostgres_Structure(t *testing.T) {
	logger := zaptest.NewLogger(t)
	writer := newMockWriter()
	generator, err := New(logger, 1, 10*time.Millisecond, writer)
	require.NoError(t, err)

	err = generator.Start(context.Background())
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return len(writer.getWrites()) > 0
	}, 2*time.Second, 5*time.Millisecond, "Expected at least one log to inspect format")

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	err = generator.Stop(ctx)
	assert.NoError(t, err)

	writes := writer.getWrites()

	line := string(writes[0])
	// Should contain timestamp, process ID, user, database, app, client, severity, and message
	assert.Contains(t, line, "[", "Should contain process ID in brackets")
	assert.Contains(t, line, "]:", "Should contain separator after process ID")
	assert.Contains(t, line, "user=", "Should contain user field")
	assert.Contains(t, line, "db=", "Should contain database field")
	assert.Contains(t, line, "client=", "Should contain client field")
}

func TestFormatAsPostgres_ParseFunc(t *testing.T) {
	logger := zaptest.NewLogger(t)
	writer := newMockWriter()
	generator, err := New(logger, 1, 10*time.Millisecond, writer)
	require.NoError(t, err)

	err = generator.Start(context.Background())
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return len(writer.getWrites()) > 0
	}, 2*time.Second, 5*time.Millisecond, "Expected at least one log to parse")

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	err = generator.Stop(ctx)
	assert.NoError(t, err)

	writes := writer.getWrites()

	line := string(writes[0])
	// Test that the parse function can extract fields
	parts := strings.Split(line, "]: ")
	assert.GreaterOrEqual(t, len(parts), 2, "PostgreSQL log format should have enough fields to parse")
}

// discardWriter implements output.Writer for benchmarking - discards all data
func TestGenerator_SetCountTracker(t *testing.T) {
	logger := zaptest.NewLogger(t)
	gen, err := New(logger, 1, 50*time.Millisecond, newMockWriter())
	require.NoError(t, err)

	assert.Nil(t, gen.tracker, "tracker should be nil initially")

	tracker := count.NewTracker(10)
	gen.SetCountTracker(tracker)
	assert.Equal(t, tracker, gen.tracker)
}

func TestGenerator_CountLimited(t *testing.T) {
	logger := zaptest.NewLogger(t)
	writer := newMockWriter()

	gen, err := New(logger, 2, 10*time.Millisecond, writer)
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

type discardWriter struct{}

func (d *discardWriter) ConsumeLogs(_ context.Context, _ []embed.LogRecord) error {
	return nil
}

func BenchmarkPostgresGenerator(b *testing.B) {
	logger := zaptest.NewLogger(b)
	writer := &discardWriter{}
	generator, err := New(logger, 1, 1*time.Millisecond, writer)
	require.NoError(b, err)

	err = generator.Start(context.Background())
	require.NoError(b, err)

	b.ResetTimer()
	time.Sleep(time.Duration(b.N) * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = generator.Stop(ctx)
}
