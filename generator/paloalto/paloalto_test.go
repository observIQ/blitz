package paloalto

import (
	"context"
	"errors"
	mrand "math/rand"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/observiq/blitz/embed"
	"github.com/observiq/blitz/generator/count"
	"github.com/observiq/blitz/internal/counttest"
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
		logs[i], _ = generatePaloAltoLog(fallbackDevice())
	}

	// Verify all logs have expected format
	for _, log := range logs {
		// Should start with timestamp (no priority tag in new format)
		assert.True(t, len(log) > 0, "Log should not be empty")
		assert.Contains(t, log, "1,", "Log should contain log type indicator")
		// Should start with a BSD timestamp like "Nov  6 15:39:29" and the host
		assert.Regexp(t, `^[A-Z][a-z]{2} [ 1-3]\d \d{2}:\d{2}:\d{2} PA-VM 1,`, log, "Log should start with timestamp and hostname")
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

// zeroSource is a math/rand source that always yields 0, so every draw from it
// is the same no matter how many other goroutines draw too.
type zeroSource struct{}

func (zeroSource) Int63() int64 { return 0 }
func (zeroSource) Seed(int64)   {}

// TestAddressDrawsShareOneSource pins that every address draw comes from the
// shared addrRand source, rather than a fresh math/rand source per call.
func TestAddressDrawsShareOneSource(t *testing.T) {
	addrRand.mu.Lock()
	orig := addrRand.r
	addrRand.r = mrand.New(zeroSource{}) // #nosec G404 -- test source
	addrRand.mu.Unlock()
	t.Cleanup(func() {
		addrRand.mu.Lock()
		addrRand.r = orig
		addrRand.mu.Unlock()
	})

	for name, tc := range map[string]struct {
		draw func() string
		want string
	}{
		"public":  {generatePublicIP, "1.0.0.1"},
		"private": {generatePrivateIP, "10.0.0.1"},
		"random":  {generateRandomIP, "10.0.0.1"},
	} {
		for range 3 {
			assert.Equal(t, tc.want, tc.draw(), name)
		}
	}
}

// TestSourceUserFollowsTheHostMapping pins the capture's User-ID behavior: a
// source address's user comes from the device's mapping for that address, so
// every record from one host, THREAT included, names the same user, and a
// User-ID record for the host names it too.
func TestSourceUserFollowsTheHostMapping(t *testing.T) {
	addrRand.mu.Lock()
	orig := addrRand.r
	addrRand.r = mrand.New(zeroSource{}) // #nosec G404 -- test source: every private draw is 10.0.0.1
	addrRand.mu.Unlock()
	t.Cleanup(func() {
		addrRand.mu.Lock()
		addrRand.r = orig
		addrRand.mu.Unlock()
	})

	dev := newDevice("pa-fw-01", generateSerial())
	users := map[string]bool{}
	for _, lt := range []string{"TRAFFIC", "THREAT", "DECRYPTION", "USERID"} {
		field := "Source User"
		if lt == "USERID" {
			field = "User"
		}
		for range 30 {
			rec := buildLogFields(lt, dev)
			require.Equal(t, "10.0.0.1", rec[indexOf(logFieldNames[lt], map[bool]string{true: "Source IP", false: "Source Address"}[lt == "USERID"])], lt)
			users[rec[indexOf(logFieldNames[lt], field)]] = true
		}
	}
	require.Len(t, users, 1, "one host maps to one user: %v", users)
	require.False(t, users[""], "the host is mapped")
}

// TestUserForIsStablePerHostAndVariesAcrossHosts pins userFor's mapping: one
// address always maps to one user, and addresses across a subnet map to every
// kind of user (two domain users and the bare GlobalProtect-style one).
func TestUserForIsStablePerHostAndVariesAcrossHosts(t *testing.T) {
	dev := newDevice("pa-fw-01", generateSerial())
	seen := map[string]bool{}
	for i := 1; i <= 254; i++ {
		addr := "10.0.0." + strconv.Itoa(i)
		u := dev.userFor(addr)
		require.Equal(t, u, dev.userFor(addr), addr)
		seen[u] = true
	}
	require.Equal(t, map[string]bool{`corp\jdoe`: true, `corp\asmith`: true, "jdoe": true}, seen)
}

// TestGenerator_RefusedWorkerWakesOnReset pins that a worker refused a permit
// waits for the tracker, then generates again once Reset is called.
func TestGenerator_RefusedWorkerWakesOnReset(t *testing.T) {
	writer := newMockWriter()
	gen, err := New(zaptest.NewLogger(t), 1, time.Millisecond, writer, embed.NopTelemetry())
	require.NoError(t, err)
	tracker := count.NewTracker(1)
	gen.SetCountTracker(tracker)
	require.NoError(t, gen.Start(context.Background()))
	t.Cleanup(func() { _ = gen.Stop(context.Background()) })
	counttest.WakesOnReset(t, tracker, func() int { return len(writer.getWrites()) })
}

// TestGenerator_RefusedWorkerStops pins that a worker refused a permit stops
// when the generator stops.
func TestGenerator_RefusedWorkerStops(t *testing.T) {
	writer := newMockWriter()
	gen, err := New(zaptest.NewLogger(t), 1, time.Millisecond, writer, embed.NopTelemetry())
	require.NoError(t, err)
	tracker := count.NewTracker(1)
	gen.SetCountTracker(tracker)
	require.NoError(t, gen.Start(context.Background()))
	counttest.StopsWhileRefused(t, tracker, func() int { return len(writer.getWrites()) }, gen.Stop)
}
