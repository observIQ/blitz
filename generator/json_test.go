package generator

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// mockWriter implements generatorWriter for testing
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

func (m *mockWriter) Write(ctx context.Context, data []byte) error {
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

	m.writes = append(m.writes, data)
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

func TestNewJSONGenerator(t *testing.T) {
	logger := zaptest.NewLogger(t)
	workers := 5
	rate := 100 * time.Millisecond

	generator, err := NewJSONGenerator(logger, workers, rate)

	assert.NoError(t, err)
	assert.NotNil(t, generator)
	assert.Equal(t, logger, generator.logger)
	assert.Equal(t, workers, generator.workers)
	assert.Equal(t, rate, generator.rate)
	assert.NotNil(t, generator.stopCh)
}

func TestNewJSONGenerator_NilLogger(t *testing.T) {
	generator, err := NewJSONGenerator(nil, 5, 100*time.Millisecond)

	assert.Error(t, err)
	assert.Nil(t, generator)
	assert.Contains(t, err.Error(), "logger cannot be nil")
}

func TestNewJSONGenerator_InvalidWorkers(t *testing.T) {
	logger := zaptest.NewLogger(t)

	// Test zero workers
	generator, err := NewJSONGenerator(logger, 0, 100*time.Millisecond)
	assert.Error(t, err)
	assert.Nil(t, generator)
	assert.Contains(t, err.Error(), "workers must be 1 or greater")

	// Test negative workers
	generator, err = NewJSONGenerator(logger, -1, 100*time.Millisecond)
	assert.Error(t, err)
	assert.Nil(t, generator)
	assert.Contains(t, err.Error(), "workers must be 1 or greater")
}

func TestJSONGenerator_Start(t *testing.T) {
	logger := zaptest.NewLogger(t)
	writer := newMockWriter()
	generator, err := NewJSONGenerator(logger, 2, 50*time.Millisecond)
	require.NoError(t, err)

	err = generator.Start(writer)
	assert.NoError(t, err)

	// Wait for some logs to be generated
	time.Sleep(200 * time.Millisecond)

	// Stop the generator
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err = generator.Stop(ctx)
	assert.NoError(t, err)

	// Verify logs were written
	writes := writer.getWrites()
	assert.Greater(t, len(writes), 0, "Expected some logs to be written")

	// Verify log structure
	for _, write := range writes {
		var log jsonLog
		err := json.Unmarshal(write, &log)
		assert.NoError(t, err, "Log should be valid JSON")

		// Verify required fields
		assert.NotZero(t, log.Timestamp, "Timestamp should be set")
		assert.NotEmpty(t, log.Level, "Level should not be empty")
		assert.NotEmpty(t, log.Environment, "Environment should not be empty")
		assert.NotEmpty(t, log.Location, "Location should not be empty")
		assert.NotEmpty(t, log.Message, "Message should not be empty")

		// Verify field values are from expected sets
		assert.Contains(t, severityLevels, log.Level, "Level should be from expected set")
		assert.Contains(t, environments, log.Environment, "Environment should be from expected set")
		assert.Contains(t, locations, log.Location, "Location should be from expected set")
		assert.Contains(t, logMessages, log.Message, "Message should be from expected set")
	}
}

func TestJSONGenerator_Stop_GracefulShutdown(t *testing.T) {
	logger := zaptest.NewLogger(t)
	writer := newMockWriter()
	generator, err := NewJSONGenerator(logger, 3, 10*time.Millisecond)
	require.NoError(t, err)

	err = generator.Start(writer)
	require.NoError(t, err)

	// Let it run briefly
	time.Sleep(50 * time.Millisecond)

	// Stop with context
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	start := time.Now()
	err = generator.Stop(ctx)
	duration := time.Since(start)

	assert.NoError(t, err)
	assert.Less(t, duration, 500*time.Millisecond, "Stop should complete quickly")

	// Verify some logs were written before stopping
	writes := writer.getWrites()
	assert.Greater(t, len(writes), 0, "Expected some logs to be written before stopping")
}
func TestJSONGenerator_WriteErrors_Backoff(t *testing.T) {
	logger := zaptest.NewLogger(t)
	writer := newMockWriter()
	writer.setWriteError(errors.New("write failed"))
	generator, err := NewJSONGenerator(logger, 1, 10*time.Millisecond)
	require.NoError(t, err)

	err = generator.Start(writer)
	require.NoError(t, err)

	// Let it run briefly to trigger write errors and backoff
	time.Sleep(200 * time.Millisecond)

	// Stop the generator
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	err = generator.Stop(ctx)
	assert.NoError(t, err)

	// Verify errors were logged
	errors := writer.getErrors()
	assert.Greater(t, len(errors), 0, "Expected some write errors")
}

func TestJSONGenerator_ConcurrentWorkers(t *testing.T) {
	logger := zaptest.NewLogger(t)
	writer := newMockWriter()
	generator, err := NewJSONGenerator(logger, 5, 20*time.Millisecond)
	require.NoError(t, err)

	err = generator.Start(writer)
	require.NoError(t, err)

	// Let multiple workers run
	time.Sleep(200 * time.Millisecond)

	// Stop the generator
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	err = generator.Stop(ctx)
	assert.NoError(t, err)

	// Verify logs were written by multiple workers
	writes := writer.getWrites()
	assert.Greater(t, len(writes), 10, "Expected many logs from multiple workers")

	// Verify logs have different timestamps (indicating concurrent generation)
	timestamps := make(map[string]int)
	for _, write := range writes {
		var log jsonLog
		err := json.Unmarshal(write, &log)
		require.NoError(t, err)
		timestamps[log.Timestamp.Format(time.RFC3339Nano)]++
	}
	assert.Greater(t, len(timestamps), 1, "Expected logs with different timestamps")
}

func TestJSONGenerator_LogMessageVariety(t *testing.T) {
	logger := zaptest.NewLogger(t)
	writer := newMockWriter()
	generator, err := NewJSONGenerator(logger, 1, 5*time.Millisecond)
	require.NoError(t, err)

	err = generator.Start(writer)
	require.NoError(t, err)

	// Let it run to generate many logs
	time.Sleep(100 * time.Millisecond)

	// Stop the generator
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	err = generator.Stop(ctx)
	assert.NoError(t, err)

	// Verify variety in generated logs
	writes := writer.getWrites()
	assert.Greater(t, len(writes), 10, "Expected many logs")

	levels := make(map[string]int)
	environments := make(map[string]int)
	locations := make(map[string]int)
	messages := make(map[string]int)

	for _, write := range writes {
		var log jsonLog
		err := json.Unmarshal(write, &log)
		require.NoError(t, err)

		levels[log.Level]++
		environments[log.Environment]++
		locations[log.Location]++
		messages[log.Message]++
	}

	// Verify we get variety in the random fields
	assert.Greater(t, len(levels), 1, "Expected variety in log levels")
	assert.Greater(t, len(environments), 1, "Expected variety in environments")
	assert.Greater(t, len(locations), 1, "Expected variety in locations")
	assert.Greater(t, len(messages), 1, "Expected variety in messages")
}

func TestJSONGenerator_LogMessageSize(t *testing.T) {
	logger := zaptest.NewLogger(t)
	writer := newMockWriter()
	generator, err := NewJSONGenerator(logger, 1, 10*time.Millisecond)
	require.NoError(t, err)

	err = generator.Start(writer)
	require.NoError(t, err)

	// Let it run briefly
	time.Sleep(50 * time.Millisecond)

	// Stop the generator
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	err = generator.Stop(ctx)
	assert.NoError(t, err)

	// Verify log message sizes are substantial
	writes := writer.getWrites()
	assert.Greater(t, len(writes), 0, "Expected some logs")

	for _, write := range writes {
		var log jsonLog
		err := json.Unmarshal(write, &log)
		require.NoError(t, err)

		// The total JSON should be substantial due to other fields
		assert.Greater(t, len(write), 200, "Total JSON should be substantial")
		assert.Greater(t, len(log.Message), 100, "Message should be substantial")
		assert.Less(t, len(log.Message), 1000, "Message should not be too large")
	}
}

func TestJSONGenerator_MultipleStartStop(t *testing.T) {
	logger := zaptest.NewLogger(t)
	writer := newMockWriter()

	// Start and stop multiple times with new generator instances
	for i := 0; i < 3; i++ {
		generator, err := NewJSONGenerator(logger, 2, 20*time.Millisecond)
		require.NoError(t, err)

		err = generator.Start(writer)
		assert.NoError(t, err)

		time.Sleep(50 * time.Millisecond)

		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		err = generator.Stop(ctx)
		cancel()
		assert.NoError(t, err)
	}

	// Verify logs were written in each cycle
	writes := writer.getWrites()
	assert.Greater(t, len(writes), 0, "Expected logs from multiple start/stop cycles")
}

func TestJSONGenerator_ZeroWorkers(t *testing.T) {
	logger := zaptest.NewLogger(t)
	generator, err := NewJSONGenerator(logger, 0, 10*time.Millisecond)

	assert.Error(t, err)
	assert.Nil(t, generator)
	assert.Contains(t, err.Error(), "workers must be 1 or greater")
}

func TestJSONGenerator_VeryFastRate(t *testing.T) {
	logger := zaptest.NewLogger(t)
	writer := newMockWriter()
	generator, err := NewJSONGenerator(logger, 1, 1*time.Millisecond)
	require.NoError(t, err)

	err = generator.Start(writer)
	require.NoError(t, err)

	// Run for a short time with very fast rate
	time.Sleep(10 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	err = generator.Stop(ctx)
	assert.NoError(t, err)

	// Should have generated many logs
	writes := writer.getWrites()
	assert.Greater(t, len(writes), 5, "Expected many logs with fast rate")
}

// discardWriter implements generatorWriter for benchmarking - discards all data
type discardWriter struct{}

func (d *discardWriter) Write(ctx context.Context, data []byte) error {
	// Discard the data - do nothing
	return nil
}

func BenchmarkGenerateRandomLog(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := generateRandomLog()
		// Prevent compiler optimization
		_ = err
	}
}
