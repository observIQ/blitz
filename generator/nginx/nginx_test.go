package nginx

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/observiq/blitz/output"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// mockWriter implements output.Writer for testing
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

func (m *mockWriter) Write(ctx context.Context, data output.LogRecord) error {
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

	m.writes = append(m.writes, append([]byte(nil), data.Message...))
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

	generator, err := New(logger, workers, rate)

	assert.NoError(t, err)
	assert.NotNil(t, generator)
	assert.Equal(t, logger, generator.logger)
	assert.Equal(t, workers, generator.workers)
	assert.Equal(t, rate, generator.rate)
	assert.NotNil(t, generator.stopCh)
}

func TestNew_NilLogger(t *testing.T) {
	generator, err := New(nil, 5, 100*time.Millisecond)

	assert.Error(t, err)
	assert.Nil(t, generator)
	assert.Contains(t, err.Error(), "logger cannot be nil")
}

func TestNew_InvalidWorkers(t *testing.T) {
	logger := zaptest.NewLogger(t)

	generator, err := New(logger, 0, 100*time.Millisecond)
	assert.Error(t, err)
	assert.Nil(t, generator)
	assert.Contains(t, err.Error(), "workers must be 1 or greater")

	generator, err = New(logger, -1, 100*time.Millisecond)
	assert.Error(t, err)
	assert.Nil(t, generator)
	assert.Contains(t, err.Error(), "workers must be 1 or greater")
}

func TestNginxGenerator_Start(t *testing.T) {
	logger := zaptest.NewLogger(t)
	writer := newMockWriter()
	generator, err := New(logger, 2, 50*time.Millisecond)
	require.NoError(t, err)

	err = generator.Start(writer)
	assert.NoError(t, err)

	time.Sleep(200 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err = generator.Stop(ctx)
	assert.NoError(t, err)

	writes := writer.getWrites()
	assert.Greater(t, len(writes), 0, "Expected some logs to be written")

	nginxPattern := regexp.MustCompile(`^\d+\.\d+\.\d+\.\d+ - .* \[.*\] ".*" \d+ \d+ ".*" ".*"$`)
	for _, write := range writes {
		line := string(write)
		assert.True(t, nginxPattern.MatchString(line), "Log should match NGINX Combined Log Format: %s", line)
		assert.Contains(t, line, `"`, "Log should contain quoted fields")
	}
}

func TestNginxGenerator_Stop_GracefulShutdown(t *testing.T) {
	logger := zaptest.NewLogger(t)
	writer := newMockWriter()
	generator, err := New(logger, 3, 10*time.Millisecond)
	require.NoError(t, err)

	err = generator.Start(writer)
	require.NoError(t, err)

	time.Sleep(50 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	start := time.Now()
	err = generator.Stop(ctx)
	duration := time.Since(start)

	assert.NoError(t, err)
	assert.Less(t, duration, 500*time.Millisecond, "Stop should complete quickly")

	writes := writer.getWrites()
	assert.Greater(t, len(writes), 0, "Expected some logs to be written before stopping")
}

func TestNginxGenerator_WriteErrors_Backoff(t *testing.T) {
	logger := zaptest.NewLogger(t)
	writer := newMockWriter()
	writer.setWriteError(errors.New("write failed"))
	generator, err := New(logger, 1, 10*time.Millisecond)
	require.NoError(t, err)

	err = generator.Start(writer)
	require.NoError(t, err)

	time.Sleep(200 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	err = generator.Stop(ctx)
	assert.NoError(t, err)

	errors := writer.getErrors()
	assert.Greater(t, len(errors), 0, "Expected some write errors")
}

func TestNginxGenerator_ConcurrentWorkers(t *testing.T) {
	logger := zaptest.NewLogger(t)
	writer := newMockWriter()
	generator, err := New(logger, 5, 20*time.Millisecond)
	require.NoError(t, err)

	err = generator.Start(writer)
	require.NoError(t, err)

	time.Sleep(200 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	err = generator.Stop(ctx)
	assert.NoError(t, err)

	writes := writer.getWrites()
	assert.Greater(t, len(writes), 10, "Expected many logs from multiple workers")
}

func TestFormatAsNginxCombined_Structure(t *testing.T) {
	logger := zaptest.NewLogger(t)
	writer := newMockWriter()
	generator, err := New(logger, 1, 10*time.Millisecond)
	require.NoError(t, err)

	err = generator.Start(writer)
	require.NoError(t, err)

	time.Sleep(50 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	err = generator.Stop(ctx)
	assert.NoError(t, err)

	writes := writer.getWrites()
	require.Greater(t, len(writes), 0)

	line := string(writes[0])
	parts := strings.Fields(line)
	assert.GreaterOrEqual(t, len(parts), 9, "NGINX Combined Log Format should have at least 9 fields")

	ipPattern := regexp.MustCompile(`^\d+\.\d+\.\d+\.\d+$`)
	assert.True(t, ipPattern.MatchString(parts[0]), "First field should be IP address")

	assert.True(t, strings.Contains(line, `"`), "Request should be quoted")
	quotedFields := strings.Count(line, `"`)
	assert.GreaterOrEqual(t, quotedFields, 3, "Should have at least 3 quoted fields (request, referer, user-agent)")
}

func TestFormatAsNginxCombined_ParseFunc(t *testing.T) {
	logger := zaptest.NewLogger(t)
	writer := newMockWriter()
	generator, err := New(logger, 1, 10*time.Millisecond)
	require.NoError(t, err)

	err = generator.Start(writer)
	require.NoError(t, err)

	time.Sleep(50 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	err = generator.Stop(ctx)
	assert.NoError(t, err)

	writes := writer.getWrites()
	require.Greater(t, len(writes), 0)

	line := string(writes[0])
	parts := strings.Fields(line)
	assert.GreaterOrEqual(t, len(parts), 9, "NGINX Combined Log Format should have enough fields to parse")
}

// discardWriter implements output.Writer for benchmarking - discards all data
type discardWriter struct{}

func (d *discardWriter) Write(ctx context.Context, data output.LogRecord) error {
	return nil
}

func BenchmarkNginxGenerator(b *testing.B) {
	logger := zaptest.NewLogger(b)
	writer := &discardWriter{}
	generator, err := New(logger, 1, 1*time.Millisecond)
	require.NoError(b, err)

	err = generator.Start(writer)
	require.NoError(b, err)

	b.ResetTimer()
	time.Sleep(time.Duration(b.N) * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = generator.Stop(ctx)
}
