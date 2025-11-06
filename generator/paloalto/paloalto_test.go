package paloalto

import (
	"context"
	"errors"
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
	assert.NotNil(t, generator.logsGenerated)
	assert.NotNil(t, generator.activeWorkers)
	assert.NotNil(t, generator.writeErrors)
}

func TestNew_NilLogger(t *testing.T) {
	generator, err := New(nil, 5, 100*time.Millisecond)

	assert.Error(t, err)
	assert.Nil(t, generator)
	assert.Contains(t, err.Error(), "logger cannot be nil")
}

func TestNew_InvalidWorkers(t *testing.T) {
	logger := zaptest.NewLogger(t)

	// Test zero workers
	generator, err := New(logger, 0, 100*time.Millisecond)
	assert.Error(t, err)
	assert.Nil(t, generator)
	assert.Contains(t, err.Error(), "workers must be 1 or greater")

	// Test negative workers
	generator, err = New(logger, -1, 100*time.Millisecond)
	assert.Error(t, err)
	assert.Nil(t, generator)
	assert.Contains(t, err.Error(), "workers must be 1 or greater")
}

func TestGenerator_Start(t *testing.T) {
	logger := zaptest.NewLogger(t)
	writer := newMockWriter()
	generator, err := New(logger, 2, 50*time.Millisecond)
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
}

func TestGenerator_Stop_GracefulShutdown(t *testing.T) {
	logger := zaptest.NewLogger(t)
	writer := newMockWriter()
	generator, err := New(logger, 3, 10*time.Millisecond)
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

func TestGenerator_WriteErrors_Backoff(t *testing.T) {
	logger := zaptest.NewLogger(t)
	writer := newMockWriter()
	writer.setWriteError(errors.New("write failed"))
	generator, err := New(logger, 1, 10*time.Millisecond)
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

func TestGenerator_ConcurrentWorkers(t *testing.T) {
	logger := zaptest.NewLogger(t)
	writer := newMockWriter()
	generator, err := New(logger, 5, 20*time.Millisecond)
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
}

func TestGeneratePaloAltoLog_Format(t *testing.T) {
	// Generate multiple logs to test variety
	logs := make([]string, 100)
	for i := 0; i < 100; i++ {
		logs[i] = generatePaloAltoLog()
	}

	// Verify all logs have expected format
	for _, log := range logs {
		// Should start with timestamp (no priority tag in new format)
		assert.True(t, len(log) > 0, "Log should not be empty")
		assert.Contains(t, log, "1,", "Log should contain log type indicator")
		// Should contain a timestamp in format like "Nov 06 15:39:29"
		assert.Regexp(t, `^[A-Z][a-z]{2} \d{1,2} \d{2}:\d{2}:\d{2}`, log, "Log should start with timestamp")
	}

	// Verify we get different log types
	logTypes := []string{"TRAFFIC", "THREAT", "SYSTEM", "CONFIG"}
	foundTypes := make(map[string]bool)
	for _, log := range logs {
		for _, logType := range logTypes {
			if strings.Contains(log, ","+logType+",") {
				foundTypes[logType] = true
				break
			}
		}
	}
	assert.Greater(t, len(foundTypes), 1, "Expected variety in log types")
}

func TestGenerateRandomIP(t *testing.T) {
	ips := make(map[string]bool)
	for i := 0; i < 100; i++ {
		ip := generateRandomIP()
		// Verify IP format
		parts := strings.Split(ip, ".")
		assert.Equal(t, 4, len(parts), "IP should have 4 octets")
		ips[ip] = true
	}
	// Should generate some variety
	assert.Greater(t, len(ips), 1, "Should generate different IPs")
}

func TestGenerateRandomPort(t *testing.T) {
	ports := make(map[string]bool)
	for i := 0; i < 100; i++ {
		port := generateRandomPort()
		// Port should be a valid string representation of a number
		assert.NotEmpty(t, port, "Port should not be empty")
		ports[port] = true
	}
	// Should generate some variety
	assert.Greater(t, len(ports), 1, "Should generate different ports")
}

func TestGenerateRandomSessionID(t *testing.T) {
	sessionIDs := make(map[string]bool)
	for i := 0; i < 100; i++ {
		sessionID := generateRandomSessionID()
		// Session ID should be 12 characters (6 bytes hex encoded)
		assert.Equal(t, 12, len(sessionID), "Session ID should be 12 characters")
		// Should be uppercase hex
		for _, char := range sessionID {
			assert.True(t, (char >= '0' && char <= '9') || (char >= 'A' && char <= 'F'),
				"Session ID should contain only hex characters")
		}
		sessionIDs[sessionID] = true
	}
	// Should generate unique session IDs
	assert.Greater(t, len(sessionIDs), 1, "Should generate different session IDs")
}

func TestGenerator_MultipleStartStop(t *testing.T) {
	logger := zaptest.NewLogger(t)
	writer := newMockWriter()

	// Start and stop multiple times with new generator instances
	for i := 0; i < 3; i++ {
		generator, err := New(logger, 2, 20*time.Millisecond)
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

func TestGenerator_VeryFastRate(t *testing.T) {
	logger := zaptest.NewLogger(t)
	writer := newMockWriter()
	generator, err := New(logger, 1, 1*time.Millisecond)
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
