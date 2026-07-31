package okta

import (
	"context"
	"encoding/json"
	"errors"
	"math/rand"
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
}

func newMockWriter() *mockWriter {
	return &mockWriter{
		writes: make([][]byte, 0),
		errors: make([]error, 0),
	}
}

func (m *mockWriter) ConsumeLogs(_ context.Context, records []embed.LogRecord) error {
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

func TestOktaGenerator_Start(t *testing.T) {
	logger := zaptest.NewLogger(t)
	writer := newMockWriter()
	generator, err := New(logger, 2, 50*time.Millisecond, writer)
	require.NoError(t, err)

	err = generator.Start(context.Background())
	assert.NoError(t, err)

	// Poll until logs have been generated, then stop.
	require.Eventually(t, func() bool {
		return len(writer.getWrites()) > 0
	}, 5*time.Second, 10*time.Millisecond)

	// Stop the generator
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err = generator.Stop(ctx)
	assert.NoError(t, err)

	// Verify logs were written
	writes := writer.getWrites()
	assert.Greater(t, len(writes), 0, "Expected some logs to be written")

	// Verify log structure - each log should be valid JSON with Okta fields
	for _, write := range writes {
		var log map[string]any
		err := json.Unmarshal(write, &log)
		assert.NoError(t, err, "Log should be valid JSON")

		// Verify required Okta System Log fields
		assert.Contains(t, log, "uuid", "Should have uuid field")
		assert.Contains(t, log, "published", "Should have published field")
		assert.Contains(t, log, "eventType", "Should have eventType field")
		assert.Contains(t, log, "severity", "Should have severity field")
		assert.Contains(t, log, "displayMessage", "Should have displayMessage field")
		assert.Contains(t, log, "actor", "Should have actor field")
		assert.Contains(t, log, "client", "Should have client field")
		assert.Contains(t, log, "outcome", "Should have outcome field")
		assert.Contains(t, log, "target", "Should have target field")

		// Verify displayMessage is a string (not a nested object)
		_, ok := log["displayMessage"].(string)
		assert.True(t, ok, "displayMessage should be a string")
	}
}

func TestOktaGenerator_Stop_GracefulShutdown(t *testing.T) {
	logger := zaptest.NewLogger(t)
	writer := newMockWriter()
	generator, err := New(logger, 3, 10*time.Millisecond, writer)
	require.NoError(t, err)

	err = generator.Start(context.Background())
	require.NoError(t, err)

	// Poll until at least one record has landed so Stop is exercised against
	// actively-running workers, then stop.
	require.Eventually(t, func() bool {
		return len(writer.getWrites()) > 0
	}, 5*time.Second, 10*time.Millisecond)

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

func TestOktaGenerator_WriteErrors_Backoff(t *testing.T) {
	logger := zaptest.NewLogger(t)
	writer := newMockWriter()
	writer.setWriteError(errors.New("write failed"))
	generator, err := New(logger, 1, 10*time.Millisecond, writer)
	require.NoError(t, err)

	err = generator.Start(context.Background())
	require.NoError(t, err)

	// Poll until write errors have been recorded, then stop.
	require.Eventually(t, func() bool {
		return len(writer.getErrors()) > 0
	}, 5*time.Second, 10*time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	err = generator.Stop(ctx)
	assert.NoError(t, err)

	errs := writer.getErrors()
	assert.Greater(t, len(errs), 0, "Expected some write errors")
}

func TestOktaGenerator_ConcurrentWorkers(t *testing.T) {
	logger := zaptest.NewLogger(t)
	writer := newMockWriter()
	generator, err := New(logger, 5, 20*time.Millisecond, writer)
	require.NoError(t, err)

	err = generator.Start(context.Background())
	require.NoError(t, err)

	// Poll until the combined worker output exceeds 10 records, then stop.
	require.Eventually(t, func() bool {
		return len(writer.getWrites()) > 10
	}, 5*time.Second, 10*time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	err = generator.Stop(ctx)
	assert.NoError(t, err)

	writes := writer.getWrites()
	assert.Greater(t, len(writes), 10, "Expected many logs from multiple workers")
}

func TestOktaGenerator_EventTypeVariety(t *testing.T) {
	logger := zaptest.NewLogger(t)
	writer := newMockWriter()
	generator, err := New(logger, 1, 5*time.Millisecond, writer)
	require.NoError(t, err)

	err = generator.Start(context.Background())
	require.NoError(t, err)

	// Poll until many logs have been generated, then stop.
	require.Eventually(t, func() bool {
		return len(writer.getWrites()) > 10
	}, 5*time.Second, 10*time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	err = generator.Stop(ctx)
	assert.NoError(t, err)

	writes := writer.getWrites()
	assert.Greater(t, len(writes), 10, "Expected many logs")

	eventTypeSet := make(map[string]int)
	severitySet := make(map[string]int)

	for _, write := range writes {
		var log map[string]any
		err := json.Unmarshal(write, &log)
		require.NoError(t, err)

		if et, ok := log["eventType"].(string); ok {
			eventTypeSet[et]++
		}
		if sev, ok := log["severity"].(string); ok {
			severitySet[sev]++
		}
	}

	assert.Greater(t, len(eventTypeSet), 1, "Expected variety in event types")
	assert.Greater(t, len(severitySet), 1, "Expected variety in severity levels")
}

func TestOktaGenerator_FailureOutcomeHasReason(t *testing.T) {
	logger := zaptest.NewLogger(t)
	r := rand.New(rand.NewSource(42)) // #nosec G404

	generator, err := New(logger, 1, time.Second, newMockWriter())
	require.NoError(t, err)

	// Generate enough logs to get some failures
	for range 200 {
		logRecord, err := generator.generateOktaLog(r)
		require.NoError(t, err)

		var log map[string]any
		err = json.Unmarshal([]byte(logRecord.Message), &log)
		require.NoError(t, err)

		outcome, ok := log["outcome"].(map[string]any)
		require.True(t, ok)

		result, ok := outcome["result"].(string)
		require.True(t, ok)

		if result == "FAILURE" {
			_, hasReason := outcome["reason"]
			assert.True(t, hasReason, "FAILURE outcomes should have a reason field")
		}
	}
}

func TestOktaGenerator_MultipleStartStop(t *testing.T) {
	logger := zaptest.NewLogger(t)
	writer := newMockWriter()

	for range 3 {
		generator, err := New(logger, 2, 20*time.Millisecond, writer)
		require.NoError(t, err)

		err = generator.Start(context.Background())
		assert.NoError(t, err)

		// Poll until this cycle has written at least one more log, then stop.
		before := len(writer.getWrites())
		require.Eventually(t, func() bool {
			return len(writer.getWrites()) > before
		}, 5*time.Second, 10*time.Millisecond)

		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		err = generator.Stop(ctx)
		cancel()
		assert.NoError(t, err)
	}

	writes := writer.getWrites()
	assert.Greater(t, len(writes), 0, "Expected logs from multiple start/stop cycles")
}

func TestGenerateUUID(t *testing.T) {
	r := rand.New(rand.NewSource(42)) // #nosec G404

	uuid := generateUUID(r)
	assert.NotEmpty(t, uuid)

	// UUID format: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
	parts := make([]string, 0)
	for _, p := range splitUUID(uuid) {
		parts = append(parts, p)
	}
	assert.Len(t, parts, 5, "UUID should have 5 dash-separated parts")
}

func splitUUID(uuid string) []string {
	var parts []string
	current := ""
	for _, c := range uuid {
		if c == '-' {
			parts = append(parts, current)
			current = ""
		} else {
			current += string(c)
		}
	}
	if current != "" {
		parts = append(parts, current)
	}
	return parts
}

func TestGenerateRandomIP(t *testing.T) {
	r := rand.New(rand.NewSource(42)) // #nosec G404

	ip := generateRandomIP(r)
	assert.NotEmpty(t, ip)
	assert.Regexp(t, `^\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}$`, ip)
}

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

func TestGenerateRequestID(t *testing.T) {
	r := rand.New(rand.NewSource(42)) // #nosec G404

	id := generateRequestID(r)
	assert.Len(t, id, 20)
	assert.Regexp(t, `^[A-Za-z0-9]+$`, id)
}
