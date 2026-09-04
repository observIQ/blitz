package wel

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/observiq/blitz/embed"
	"github.com/observiq/blitz/generator/count"
	"github.com/observiq/blitz/generator/wel/catalog"
	"github.com/observiq/blitz/internal/datagen"
	"github.com/observiq/blitz/telemetry"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// mockConsumer implements embed.LogConsumer for tests.
type mockConsumer struct {
	mu       sync.Mutex
	messages []string
}

func (m *mockConsumer) ConsumeLogs(_ context.Context, records []embed.LogRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, r := range records {
		m.messages = append(m.messages, r.Message)
	}
	return nil
}

func (m *mockConsumer) Count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.messages)
}

func (m *mockConsumer) Messages() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := make([]string, len(m.messages))
	copy(cp, m.messages)
	return cp
}

func TestNewGenerator(t *testing.T) {
	logger := zap.NewNop()

	t.Run("valid config", func(t *testing.T) {
		gen, err := New(Config{
			Logger:    logger,
			Workers:   1,
			Rate:      100 * time.Millisecond,
			Computer:  "TESTPC",
			Domain:    "TESTDOMAIN",
			Role:      catalog.RoleMember,
			Usernames: []string{"jsmith"},
			IPs:       []string{"10.0.0.1"},
			Hostnames: []string{"WS01"},
			Consumer:  &mockConsumer{},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if gen == nil {
			t.Fatal("generator should not be nil")
		}
	})

	t.Run("nil logger", func(t *testing.T) {
		_, err := New(Config{Workers: 1, Consumer: &mockConsumer{}})
		if err == nil {
			t.Error("expected error for nil logger")
		}
	})

	t.Run("nil consumer", func(t *testing.T) {
		_, err := New(Config{Logger: logger, Workers: 1})
		if err == nil {
			t.Error("expected error for nil consumer")
		}
	})

	t.Run("zero workers", func(t *testing.T) {
		_, err := New(Config{Logger: logger, Workers: 0, Consumer: &mockConsumer{}})
		if err == nil {
			t.Error("expected error for zero workers")
		}
	})

	t.Run("default role", func(t *testing.T) {
		gen, err := New(Config{
			Logger:    logger,
			Workers:   1,
			Rate:      time.Second,
			Usernames: []string{"test"},
			Consumer:  &mockConsumer{},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if gen.role != catalog.RoleMember {
			t.Errorf("expected default role member, got %s", gen.role)
		}
	})

	t.Run("invalid channel filter", func(t *testing.T) {
		_, err := New(Config{
			Logger:    logger,
			Workers:   1,
			Rate:      time.Second,
			Channels:  []string{"NonExistentChannel"},
			Usernames: []string{"test"},
			Consumer:  &mockConsumer{},
		})
		if err == nil {
			t.Error("expected error for invalid channel filter")
		}
	})
}

func TestGeneratorStartStop(t *testing.T) {
	logger := zap.NewNop()
	consumer := &mockConsumer{}
	gen, err := New(Config{
		Logger:    logger,
		Workers:   2,
		Rate:      10 * time.Millisecond,
		Computer:  "TESTPC",
		Domain:    "CONTOSO",
		Role:      catalog.RoleWorkstation,
		Usernames: []string{"jsmith", "admin"},
		IPs:       []string{"10.0.0.1"},
		Hostnames: []string{"WS01"},
		Consumer:  consumer,
	})
	require.NoError(t, err)

	require.NoError(t, gen.Start(context.Background()))

	// Wait for the workers to land at least one record. A fixed sleep
	// would have flake risk under a slow scheduler.
	require.Eventually(t, func() bool {
		return consumer.Count() > 0
	}, 2*time.Second, 5*time.Millisecond, "expected at least one event before Stop")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, gen.Stop(ctx))

	// Verify output is valid XML-like
	for _, msg := range consumer.Messages() {
		if !strings.Contains(msg, "<Event xmlns=") {
			t.Errorf("expected XML event output, got: %s", msg[:min(len(msg), 100)])
		}
	}
}

// TestGeneratorHonorsFiniteCount confirms the WEL generator stops at the
// configured record budget: with a count tracker set, generation completes the
// tracker (Done fires) rather than running unbounded (PIPE-1111).
func TestGeneratorHonorsFiniteCount(t *testing.T) {
	logger := zap.NewNop()
	consumer := &mockConsumer{}
	gen, err := New(Config{
		Logger:   logger,
		Workers:  2,
		Rate:     5 * time.Millisecond,
		Computer: "TESTPC",
		Domain:   "CONTOSO",
		Role:     catalog.RoleWorkstation,
		Consumer: consumer,
	})
	require.NoError(t, err)

	tracker := count.NewTracker(5)
	gen.SetCountTracker(tracker)

	require.NoError(t, gen.Start(context.Background()))

	select {
	case <-tracker.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("WEL generator should honor the finite count and complete the tracker")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, gen.Stop(ctx))

	require.GreaterOrEqual(t, tracker.Emitted(), int64(5), "tracker should record at least the requested count")
	require.Positive(t, consumer.Count(), "consumer should have received records")
}

// TestGeneratorResumesAfterCountReset covers the idle-and-resume path: once the
// budget is exhausted the worker idles (emitting nothing), and a tracker Reset
// unblocks it via ResumeC so generation continues (PIPE-1111).
func TestGeneratorResumesAfterCountReset(t *testing.T) {
	logger := zap.NewNop()
	consumer := &mockConsumer{}
	gen, err := New(Config{
		Logger:   logger,
		Workers:  1,
		Rate:     5 * time.Millisecond,
		Computer: "TESTPC",
		Domain:   "CONTOSO",
		Role:     catalog.RoleWorkstation,
		Consumer: consumer,
	})
	require.NoError(t, err)

	tracker := count.NewTracker(3)
	gen.SetCountTracker(tracker)
	require.NoError(t, gen.Start(context.Background()))

	// The budget completes, then generation holds at the budget: the worker is
	// idle in the ResumeC select for the whole window.
	require.Eventually(t, func() bool { return consumer.Count() >= 3 }, 5*time.Second, 5*time.Millisecond)
	require.Never(t, func() bool { return consumer.Count() > 3 }, 150*time.Millisecond, 10*time.Millisecond,
		"generation must not exceed the budget while idle")

	// Reset re-opens the budget and unblocks the idle worker via ResumeC.
	tracker.Reset()
	require.Eventually(t, func() bool { return consumer.Count() > 3 }, 5*time.Second, 10*time.Millisecond,
		"generation should resume after the tracker is reset")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, gen.Stop(ctx))
}

func TestGeneratorSupportedTelemetry(t *testing.T) {
	logger := zap.NewNop()
	gen, err := New(Config{
		Logger:    logger,
		Workers:   1,
		Rate:      time.Second,
		Usernames: []string{"test"},
		Consumer:  &mockConsumer{},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	types := gen.SupportedTelemetry()
	if len(types) != 1 {
		t.Fatalf("expected 1 telemetry type, got %d", len(types))
	}
	if types[0] != telemetry.Logs {
		t.Errorf("expected Logs telemetry type, got %v", types[0])
	}
}

func TestSetHostIdentity(t *testing.T) {
	gen, err := New(Config{
		Logger:    zap.NewNop(),
		Workers:   1,
		Rate:      time.Second,
		Usernames: []string{"test"},
		Consumer:  &mockConsumer{},
	})
	require.NoError(t, err)

	gen.SetHostIdentity(&datagen.SystemIdentity{
		Hostname: "IDENTITY-HOST",
		OSInfo:   datagen.OSInfo{Type: datagen.OSLinux},
	})
	require.Equal(t, "IDENTITY-HOST", gen.static.Record()["host.name"])

	gen.SetHostIdentity(nil)
	require.NotEmpty(t, gen.static.Record()["host.name"])
}
