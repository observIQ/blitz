package paloalto

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/observiq/blitz/embed"
	"github.com/observiq/blitz/generator/count"
	"github.com/observiq/blitz/internal/datagen"
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

	generator, err := New(logger, workers, rate, newMockWriter(), embed.NopTelemetry())

	assert.NoError(t, err)
	assert.NotNil(t, generator)
	assert.Equal(t, logger, generator.logger)
	assert.Equal(t, workers, generator.workers)
	assert.Equal(t, rate, generator.rate)
	assert.NotNil(t, generator.stopCh)
}

func TestNew_NilLogger(t *testing.T) {
	generator, err := New(nil, 5, 100*time.Millisecond, newMockWriter(), embed.NopTelemetry())

	assert.Error(t, err)
	assert.Nil(t, generator)
	assert.Contains(t, err.Error(), "logger cannot be nil")
}

func TestNew_InvalidWorkers(t *testing.T) {
	logger := zaptest.NewLogger(t)

	// Test zero workers
	generator, err := New(logger, 0, 100*time.Millisecond, newMockWriter(), embed.NopTelemetry())
	assert.Error(t, err)
	assert.Nil(t, generator)
	assert.Contains(t, err.Error(), "workers must be 1 or greater")

	// Test negative workers
	generator, err = New(logger, -1, 100*time.Millisecond, newMockWriter(), embed.NopTelemetry())
	assert.Error(t, err)
	assert.Nil(t, generator)
	assert.Contains(t, err.Error(), "workers must be 1 or greater")
}

func TestGenerator_Start(t *testing.T) {
	logger := zaptest.NewLogger(t)
	writer := newMockWriter()
	generator, err := New(logger, 2, 50*time.Millisecond, writer, embed.NopTelemetry())
	require.NoError(t, err)

	err = generator.Start(context.Background())
	assert.NoError(t, err)

	// Wait for at least one record to land before stopping.
	require.Eventually(t, func() bool {
		return len(writer.getWrites()) > 0
	}, 2*time.Second, 10*time.Millisecond, "Expected some logs to be written")

	// Stop the generator
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err = generator.Stop(ctx)
	assert.NoError(t, err)
}

func TestGenerator_Stop_GracefulShutdown(t *testing.T) {
	logger := zaptest.NewLogger(t)
	writer := newMockWriter()
	generator, err := New(logger, 3, 10*time.Millisecond, writer, embed.NopTelemetry())
	require.NoError(t, err)

	err = generator.Start(context.Background())
	require.NoError(t, err)

	// Wait until at least one worker has emitted a record so the Stop
	// path is exercised against actively-running workers, not a
	// just-started generator that may not have produced anything yet.
	require.Eventually(t, func() bool {
		return len(writer.getWrites()) > 0
	}, 2*time.Second, 5*time.Millisecond, "Expected some logs to be written before stopping")

	// Stop with context — the duration assertion below is the real
	// timing check this test cares about.
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	start := time.Now()
	err = generator.Stop(ctx)
	duration := time.Since(start)

	assert.NoError(t, err)
	assert.Less(t, duration, 500*time.Millisecond, "Stop should complete quickly")
}

func TestGenerator_WriteErrors_Backoff(t *testing.T) {
	logger := zaptest.NewLogger(t)
	writer := newMockWriter()
	writer.setWriteError(errors.New("write failed"))
	generator, err := New(logger, 1, 10*time.Millisecond, writer, embed.NopTelemetry())
	require.NoError(t, err)

	err = generator.Start(context.Background())
	require.NoError(t, err)

	// Wait until at least one write error has been recorded so the
	// backoff path has clearly been exercised.
	require.Eventually(t, func() bool {
		return len(writer.getErrors()) > 0
	}, 2*time.Second, 10*time.Millisecond, "Expected some write errors")

	// Stop the generator
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	err = generator.Stop(ctx)
	assert.NoError(t, err)
}

func TestGenerator_ConcurrentWorkers(t *testing.T) {
	logger := zaptest.NewLogger(t)
	writer := newMockWriter()
	generator, err := New(logger, 5, 20*time.Millisecond, writer, embed.NopTelemetry())
	require.NoError(t, err)

	err = generator.Start(context.Background())
	require.NoError(t, err)

	// Wait for the combined worker output to exceed 10 records — this
	// demonstrates concurrent workers are actually producing.
	require.Eventually(t, func() bool {
		return len(writer.getWrites()) > 10
	}, 2*time.Second, 10*time.Millisecond, "Expected many logs from multiple workers")

	// Stop the generator
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	err = generator.Stop(ctx)
	assert.NoError(t, err)
}

func TestGeneratePaloAltoLog_Format(t *testing.T) {
	// Generate multiple logs to test variety
	logs := make([]string, 100)
	for i := range 100 {
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
	for range 100 {
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
	for range 100 {
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
	for range 100 {
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

	// Start and stop multiple times with new generator instances. Each
	// cycle must produce at least one additional record before the
	// generator is stopped — otherwise the cycle proved nothing.
	for range 3 {
		generator, err := New(logger, 2, 20*time.Millisecond, writer, embed.NopTelemetry())
		require.NoError(t, err)

		err = generator.Start(context.Background())
		assert.NoError(t, err)

		baseline := len(writer.getWrites())
		require.Eventually(t, func() bool {
			return len(writer.getWrites()) > baseline
		}, 2*time.Second, 5*time.Millisecond, "Expected this cycle to add at least one record")

		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		err = generator.Stop(ctx)
		cancel()
		assert.NoError(t, err)
	}
}

func TestGenerator_VeryFastRate(t *testing.T) {
	logger := zaptest.NewLogger(t)
	writer := newMockWriter()
	generator, err := New(logger, 1, 1*time.Millisecond, writer, embed.NopTelemetry())
	require.NoError(t, err)

	err = generator.Start(context.Background())
	require.NoError(t, err)

	// Poll until the worker produces more than 5 records. A fixed
	// time.Sleep + assert.Greater was flake-prone on CI: a slow scheduler
	// could see exactly 5 writes inside the sleep budget.
	require.Eventually(t, func() bool {
		return len(writer.getWrites()) > 5
	}, 2*time.Second, 5*time.Millisecond, "Expected many logs with fast rate")

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	err = generator.Stop(ctx)
	assert.NoError(t, err)
}

func TestGenerator_SetCountTracker(t *testing.T) {
	logger := zaptest.NewLogger(t)
	gen, err := New(logger, 1, 50*time.Millisecond, newMockWriter(), embed.NopTelemetry())
	require.NoError(t, err)

	assert.Nil(t, gen.tracker, "tracker should be nil initially")

	tracker := count.NewTracker(10)
	gen.SetCountTracker(tracker)
	assert.Equal(t, tracker, gen.tracker)
}

func TestGenerator_CountLimited(t *testing.T) {
	logger := zaptest.NewLogger(t)
	writer := newMockWriter()

	gen, err := New(logger, 2, 10*time.Millisecond, writer, embed.NopTelemetry())
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
	// Use Never to assert the bound holds across a short window; a fixed
	// time.Sleep here only made it slightly more likely that a stray
	// post-Done record would have landed before the assertion ran.
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

func TestSetHostIdentity(t *testing.T) {
	logger := zaptest.NewLogger(t)
	gen, err := New(logger, 1, 100*time.Millisecond, newMockWriter(), embed.NopTelemetry())
	require.NoError(t, err)

	gen.SetHostIdentity(&datagen.SystemIdentity{
		Hostname: "IDENTITY-HOST",
		OSInfo:   datagen.OSInfo{Type: datagen.OSLinux},
	})
	assert.Equal(t, "IDENTITY-HOST", gen.static.Record()["host.name"])

	gen.SetHostIdentity(nil)
	assert.NotEmpty(t, gen.static.Record()["host.name"])
}
