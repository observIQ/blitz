package wel

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/observiq/blitz/embed"
	"github.com/observiq/blitz/generator/wel/catalog"
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
