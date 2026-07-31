package apache

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

// mockConsumer implements embed.LogConsumer for testing. It captures
// every record pushed through ConsumeLogs and can inject errors for
// failure-path tests.
type mockConsumer struct {
	mu         sync.Mutex
	records    []embed.LogRecord
	errors     []error
	consumeErr error
	delay      time.Duration
}

func newMockConsumer() *mockConsumer {
	return &mockConsumer{
		records: make([]embed.LogRecord, 0),
		errors:  make([]error, 0),
	}
}

func (m *mockConsumer) ConsumeLogs(ctx context.Context, records []embed.LogRecord) error {
	if m.delay > 0 {
		select {
		case <-time.After(m.delay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.consumeErr != nil {
		err := m.consumeErr
		m.errors = append(m.errors, err)
		return err
	}

	for i := range records {
		m.records = append(m.records, records[i])
	}
	return nil
}

func (m *mockConsumer) writes() [][]byte {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([][]byte, len(m.records))
	for i, rec := range m.records {
		out[i] = []byte(rec.Message)
	}
	return out
}

func (m *mockConsumer) getErrors() []error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]error(nil), m.errors...)
}

func (m *mockConsumer) setConsumeError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.consumeErr = err
}

func TestNew(t *testing.T) {
	logger := zaptest.NewLogger(t)
	workers := 5
	rate := 100 * time.Millisecond

	generator, err := New(logger, workers, rate, newMockConsumer())

	assert.NoError(t, err)
	assert.NotNil(t, generator)
	assert.Equal(t, logger, generator.logger)
	assert.Equal(t, workers, generator.workers)
	assert.Equal(t, rate, generator.rate)
	assert.NotNil(t, generator.stopCh)
	assert.NotNil(t, generator.consumer)
}

func TestNew_NilLogger(t *testing.T) {
	generator, err := New(nil, 5, 100*time.Millisecond, newMockConsumer())

	assert.Error(t, err)
	assert.Nil(t, generator)
	assert.Contains(t, err.Error(), "logger cannot be nil")
}

func TestNew_NilConsumer(t *testing.T) {
	logger := zaptest.NewLogger(t)
	generator, err := New(logger, 5, 100*time.Millisecond, nil)

	assert.Error(t, err)
	assert.Nil(t, generator)
	assert.Contains(t, err.Error(), "consumer cannot be nil")
}

func TestNew_InvalidWorkers(t *testing.T) {
	logger := zaptest.NewLogger(t)

	// Test zero workers
	generator, err := New(logger, 0, 100*time.Millisecond, newMockConsumer())
	assert.Error(t, err)
	assert.Nil(t, generator)
	assert.Contains(t, err.Error(), "workers must be 1 or greater")

	// Test negative workers
	generator, err = New(logger, -1, 100*time.Millisecond, newMockConsumer())
	assert.Error(t, err)
	assert.Nil(t, generator)
	assert.Contains(t, err.Error(), "workers must be 1 or greater")
}

func TestApacheGenerator_Name(t *testing.T) {
	logger := zaptest.NewLogger(t)
	generator, err := New(logger, 1, 100*time.Millisecond, newMockConsumer())
	require.NoError(t, err)
	assert.Equal(t, componentName, generator.Name())
}

// Compile-time assertion: the migrated generator satisfies
// embed.ProducerModule. PR #1 contributed the marker; PR #2 wires up
// Name/Start/Stop with the correct signatures.
var _ embed.ProducerModule = (*ApacheLogGenerator)(nil)

func TestApacheGenerator_Start(t *testing.T) {
	logger := zaptest.NewLogger(t)
	consumer := newMockConsumer()
	generator, err := New(logger, 2, 50*time.Millisecond, consumer)
	require.NoError(t, err)

	err = generator.Start(context.Background())
	assert.NoError(t, err)

	// Poll until logs have been generated, then stop.
	require.Eventually(t, func() bool {
		return len(consumer.writes()) > 0
	}, 5*time.Second, 10*time.Millisecond)

	// Stop the generator
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err = generator.Stop(ctx)
	assert.NoError(t, err)

	// Verify logs were written
	writes := consumer.writes()
	assert.Greater(t, len(writes), 0, "Expected some logs to be written")

	// Verify CLF format
	clfPattern := regexp.MustCompile(`^\d+\.\d+\.\d+\.\d+ - - \[.*\] ".*" \d+ \d+$`)
	for _, write := range writes {
		line := string(write)
		assert.True(t, clfPattern.MatchString(line), "Log should match CLF format: %s", line)
	}
}

func TestApacheGenerator_Stop_GracefulShutdown(t *testing.T) {
	logger := zaptest.NewLogger(t)
	consumer := newMockConsumer()
	generator, err := New(logger, 3, 10*time.Millisecond, consumer)
	require.NoError(t, err)

	err = generator.Start(context.Background())
	require.NoError(t, err)

	// Poll until logs have been generated, then stop.
	require.Eventually(t, func() bool {
		return len(consumer.writes()) > 0
	}, 5*time.Second, 10*time.Millisecond)

	// Stop with context
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	start := time.Now()
	err = generator.Stop(ctx)
	duration := time.Since(start)

	assert.NoError(t, err)
	assert.Less(t, duration, 500*time.Millisecond, "Stop should complete quickly")

	// Verify some logs were written before stopping
	writes := consumer.writes()
	assert.Greater(t, len(writes), 0, "Expected some logs to be written before stopping")
}

func TestApacheGenerator_WriteErrors_Backoff(t *testing.T) {
	logger := zaptest.NewLogger(t)
	consumer := newMockConsumer()
	consumer.setConsumeError(errors.New("write failed"))
	generator, err := New(logger, 1, 10*time.Millisecond, consumer)
	require.NoError(t, err)

	err = generator.Start(context.Background())
	require.NoError(t, err)

	// Poll until write errors have been recorded, then stop.
	require.Eventually(t, func() bool {
		return len(consumer.getErrors()) > 0
	}, 5*time.Second, 10*time.Millisecond)

	// Stop the generator
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	err = generator.Stop(ctx)
	assert.NoError(t, err)

	// Verify errors were logged
	errs := consumer.getErrors()
	assert.Greater(t, len(errs), 0, "Expected some write errors")
}

func TestApacheGenerator_ConcurrentWorkers(t *testing.T) {
	logger := zaptest.NewLogger(t)
	consumer := newMockConsumer()
	generator, err := New(logger, 5, 20*time.Millisecond, consumer)
	require.NoError(t, err)

	err = generator.Start(context.Background())
	require.NoError(t, err)

	// Poll until many logs have been written by the workers, then stop.
	require.Eventually(t, func() bool {
		return len(consumer.writes()) > 10
	}, 5*time.Second, 10*time.Millisecond)

	// Stop the generator
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	err = generator.Stop(ctx)
	assert.NoError(t, err)

	// Verify logs were written by multiple workers
	writes := consumer.writes()
	assert.Greater(t, len(writes), 10, "Expected many logs from multiple workers")
}

func TestFormatAsApacheCLF_DefaultLog(t *testing.T) {
	logger := zaptest.NewLogger(t)
	consumer := newMockConsumer()
	generator, err := New(logger, 1, 10*time.Millisecond, consumer)
	require.NoError(t, err)

	err = generator.Start(context.Background())
	require.NoError(t, err)

	// Poll until logs have been generated, then stop.
	require.Eventually(t, func() bool {
		return len(consumer.writes()) > 0
	}, 5*time.Second, 10*time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	err = generator.Stop(ctx)
	assert.NoError(t, err)

	writes := consumer.writes()
	require.Greater(t, len(writes), 0)

	// Verify CLF structure
	line := string(writes[0])
	parts := strings.Fields(line)
	assert.GreaterOrEqual(t, len(parts), 7, "CLF should have at least 7 fields")

	// Verify IP address format
	ipPattern := regexp.MustCompile(`^\d+\.\d+\.\d+\.\d+$`)
	assert.True(t, ipPattern.MatchString(parts[0]), "First field should be IP address")

	// Verify request is quoted
	assert.True(t, strings.Contains(line, `"`), "Request should be quoted")
}

func TestFormatAsApacheCLF_ParseFunc(t *testing.T) {
	logger := zaptest.NewLogger(t)
	consumer := newMockConsumer()
	generator, err := New(logger, 1, 10*time.Millisecond, consumer)
	require.NoError(t, err)

	err = generator.Start(context.Background())
	require.NoError(t, err)

	// Poll until logs have been generated, then stop.
	require.Eventually(t, func() bool {
		return len(consumer.writes()) > 0
	}, 5*time.Second, 10*time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	err = generator.Stop(ctx)
	assert.NoError(t, err)

	writes := consumer.writes()
	require.Greater(t, len(writes), 0)

	// Test that ParseFunc is wired on the captured record and that it
	// returns a usable field map for the matching message.
	consumer.mu.Lock()
	rec := consumer.records[0]
	consumer.mu.Unlock()
	require.NotNil(t, rec.ParseFunc)
	fields, err := rec.ParseFunc(rec.Message)
	require.NoError(t, err)
	assert.NotEmpty(t, fields["remote_host"])

	// Test that the format is also parseable by raw inspection.
	line := string(writes[0])
	parts := strings.Fields(line)
	assert.GreaterOrEqual(t, len(parts), 7, "CLF should have enough fields to parse")
}

// discardConsumer implements embed.LogConsumer for benchmarking — discards all data
type discardConsumer struct{}

func (d *discardConsumer) ConsumeLogs(_ context.Context, _ []embed.LogRecord) error {
	return nil
}

func TestApacheLogGenerator_SetCountTracker(t *testing.T) {
	logger := zaptest.NewLogger(t)
	gen, err := New(logger, 1, 50*time.Millisecond, newMockConsumer())
	require.NoError(t, err)

	assert.Nil(t, gen.tracker, "tracker should be nil initially")

	tracker := count.NewTracker(10)
	gen.SetCountTracker(tracker)
	assert.Equal(t, tracker, gen.tracker)
}

func TestApacheLogGenerator_CountLimited(t *testing.T) {
	logger := zaptest.NewLogger(t)
	consumer := newMockConsumer()

	gen, err := New(logger, 2, 10*time.Millisecond, consumer)
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

	// Poll until all 5 writes have landed (the tracker caps at 5, so it will not
	// exceed), then stop.
	require.Eventually(t, func() bool {
		return len(consumer.writes()) == 5
	}, 5*time.Second, 10*time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err = gen.Stop(ctx)
	require.NoError(t, err)

	writes := consumer.writes()
	assert.Equal(t, 5, len(writes), "Expected exactly 5 logs with count tracker")
}

func BenchmarkApacheGenerator(b *testing.B) {
	logger := zaptest.NewLogger(b)
	consumer := &discardConsumer{}
	generator, err := New(logger, 1, 1*time.Millisecond, consumer)
	require.NoError(b, err)

	err = generator.Start(context.Background())
	require.NoError(b, err)

	b.ResetTimer()
	time.Sleep(time.Duration(b.N) * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = generator.Stop(ctx)
}
