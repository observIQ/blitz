package traces

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/observiq/blitz/embed"
	"github.com/observiq/blitz/generator/count"
	"github.com/observiq/blitz/telemetry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

type mockTraceConsumer struct {
	mu       sync.Mutex
	spans    []embed.Span
	arrivals []time.Time // wall-clock time each span arrived
}

func (m *mockTraceConsumer) ConsumeTraces(_ context.Context, batch []embed.Span) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	arrival := time.Now()
	for _, sp := range batch {
		m.spans = append(m.spans, sp)
		m.arrivals = append(m.arrivals, arrival)
	}
	return nil
}

func (m *mockTraceConsumer) Arrivals() []time.Time {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]time.Time, len(m.arrivals))
	copy(out, m.arrivals)
	return out
}

func (m *mockTraceConsumer) Snapshot() []embed.Span {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]embed.Span, len(m.spans))
	copy(out, m.spans)
	return out
}

func (m *mockTraceConsumer) Count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.spans)
}

func baseCfg(t *testing.T, cons embed.TraceConsumer) Config {
	t.Helper()
	return Config{
		Logger:   zaptest.NewLogger(t),
		Workers:  1,
		Rate:     time.Second,
		Consumer: cons,
	}
}

func TestNew(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		g, err := New(baseCfg(t, &mockTraceConsumer{}))
		require.NoError(t, err)
		assert.NotNil(t, g)
	})

	t.Run("nil logger", func(t *testing.T) {
		cfg := baseCfg(t, &mockTraceConsumer{})
		cfg.Logger = nil
		_, err := New(cfg)
		require.Error(t, err)
	})

	t.Run("nil consumer", func(t *testing.T) {
		cfg := baseCfg(t, nil)
		_, err := New(cfg)
		require.Error(t, err)
	})

	t.Run("invalid workers", func(t *testing.T) {
		cfg := baseCfg(t, &mockTraceConsumer{})
		cfg.Workers = 0
		_, err := New(cfg)
		require.Error(t, err)
	})
}

func TestNameAndSupportedTelemetry(t *testing.T) {
	g, err := New(baseCfg(t, &mockTraceConsumer{}))
	require.NoError(t, err)

	assert.Equal(t, "traces", g.Name())
	assert.Equal(t, []telemetry.Type{telemetry.Traces}, g.SupportedTelemetry())
}

// TestStartStop confirms that running the generator briefly produces
// spans that flow through the consumer. Per-span emission via
// time.AfterFunc means it takes the longest span's duration of
// wall-clock time after a trace starts before all that trace's spans
// have arrived — the Eventually budget is sized accordingly.
func TestStartStop(t *testing.T) {
	cons := &mockTraceConsumer{}
	cfg := baseCfg(t, cons)
	cfg.Rate = 50 * time.Millisecond

	g, err := New(cfg)
	require.NoError(t, err)

	require.NoError(t, g.Start(context.Background()))

	require.Eventually(t, func() bool { return cons.Count() >= 2 }, 3*time.Second, 20*time.Millisecond,
		"should consume at least one trace (2 spans) within the EndTime budget")

	require.NoError(t, g.Stop(context.Background()))

	spans := cons.Snapshot()
	assert.GreaterOrEqual(t, len(spans), 2)
}

// TestSpansEmittedAtEndTime confirms that each span's arrival at the
// consumer happens at-or-after its own EndTime — i.e., the generator
// does NOT emit a trace's spans all at once at trace-start. This is
// the load-bearing realism guarantee for downstream distributed-blitz
// work. We pair each captured span with its recorded arrival time and
// assert arrival ≥ EndTime per span; this invariant holds whether Stop
// aborts or drains.
func TestSpansEmittedAtEndTime(t *testing.T) {
	cons := &mockTraceConsumer{}
	cfg := baseCfg(t, cons)
	cfg.Rate = 100 * time.Millisecond
	cfg.Seed = 0 // deterministic

	g, err := New(cfg)
	require.NoError(t, err)

	require.NoError(t, g.Start(context.Background()))
	require.Eventually(t, func() bool { return cons.Count() >= 4 }, 5*time.Second, 20*time.Millisecond)
	require.NoError(t, g.Stop(context.Background()))

	spans := cons.Snapshot()
	arrivals := cons.Arrivals()
	require.Equal(t, len(spans), len(arrivals), "consumer must record one arrival timestamp per span")

	for i, sp := range spans {
		assert.False(t, sp.EndTime.After(arrivals[i]),
			"span %d (%s) EndTime %v is after its arrival %v — emitted before EndTime",
			i, sp.Name, sp.EndTime, arrivals[i])
	}
}

// TestTraceStructure inspects coherent shape of one trace: a Server
// root span plus 1-4 child spans (any of Client/Internal kinds), with
// every child's ParentSpanID pointing at the root.
func TestTraceStructure(t *testing.T) {
	cons := &mockTraceConsumer{}
	cfg := baseCfg(t, cons)
	cfg.Rate = 100 * time.Millisecond

	g, err := New(cfg)
	require.NoError(t, err)

	require.NoError(t, g.Start(context.Background()))
	require.Eventually(t, func() bool { return cons.Count() >= 2 }, 5*time.Second, 20*time.Millisecond)
	require.NoError(t, g.Stop(context.Background()))

	spans := cons.Snapshot()
	require.NotEmpty(t, spans)

	byTrace := make(map[string][]embed.Span)
	for _, sp := range spans {
		byTrace[sp.TraceID] = append(byTrace[sp.TraceID], sp)
	}

	for traceID, group := range byTrace {
		var root *embed.Span
		var children []embed.Span
		for i := range group {
			sp := group[i]
			if sp.Kind == embed.SpanKindServer && sp.ParentSpanID == "" {
				root = &sp
				continue
			}
			children = append(children, sp)
		}
		if root == nil || len(children) == 0 {
			continue
		}
		assert.NotEmpty(t, root.SpanID, "trace %s: root SpanID should be set", traceID)
		assert.True(t, root.EndTime.After(root.StartTime), "trace %s: root EndTime > StartTime", traceID)
		assert.NotEmpty(t, root.Metadata.Resource["host.name"], "trace %s: root should carry host.name resource", traceID)
		assert.Equal(t, "traces", root.Metadata.Resource["telemetry.source"], "trace %s: root should carry telemetry.source", traceID)
		assert.LessOrEqual(t, len(children), 4, "trace %s: child count should not exceed 4", traceID)
		for _, child := range children {
			assert.Equal(t, root.SpanID, child.ParentSpanID, "trace %s: child %q should parent to root", traceID, child.Name)
			assert.NotEmpty(t, child.Metadata.Resource["host.name"], "trace %s: child %q should carry host.name resource", traceID, child.Name)
		}
		return // one complete trace verified is enough
	}
	t.Fatalf("no complete trace (root + child) found in %d spans across %d traces", len(spans), len(byTrace))
}

func TestCountTracker(t *testing.T) {
	cons := &mockTraceConsumer{}
	cfg := baseCfg(t, cons)
	cfg.Rate = 50 * time.Millisecond

	g, err := New(cfg)
	require.NoError(t, err)

	tracker := count.NewTracker(2)
	g.SetCountTracker(tracker)

	require.NoError(t, g.Start(context.Background()))

	select {
	case <-tracker.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("tracker should have completed")
	}

	require.NoError(t, g.Stop(context.Background()))
}

// TestSeedDeterminism confirms two generators with the same Seed produce
// identical TraceIDs (well, they don't — TraceIDs come from crypto/rand
// which is independent — but they DO produce identical method / path /
// statusCode / duration choices because those flow from the seeded
// mathrand.Rand). We assert the deterministic fields match across runs.
func TestSeedDeterminism(t *testing.T) {
	runOnce := func() []embed.Span {
		cons := &mockTraceConsumer{}
		cfg := baseCfg(t, cons)
		cfg.Rate = 20 * time.Millisecond
		cfg.Seed = 7777

		g, err := New(cfg)
		require.NoError(t, err)
		require.NoError(t, g.Start(context.Background()))
		require.Eventually(t, func() bool { return cons.Count() >= 4 }, 3*time.Second, 10*time.Millisecond)
		stopCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		require.NoError(t, g.Stop(stopCtx))
		return cons.Snapshot()
	}

	a := runOnce()
	b := runOnce()

	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	require.Positive(t, n, "neither run produced spans")

	// Compare deterministic fields. Span names ("GET /api/users",
	// "SELECT users", etc.), Kind, and HTTP status_code attribute all
	// flow from the seeded RNG. TraceID / SpanID use crypto/rand and
	// are intentionally non-deterministic per OTel spec.
	for i := 0; i < n; i++ {
		assert.Equal(t, a[i].Name, b[i].Name, "span %d: Name mismatch across seeded runs", i)
		assert.Equal(t, a[i].Kind, b[i].Kind, "span %d: Kind mismatch across seeded runs", i)
	}
}

func TestGenerateTraceID(t *testing.T) {
	id := generateTraceID()
	assert.Len(t, id, 32) // 16 bytes = 32 hex chars
}

func TestGenerateSpanID(t *testing.T) {
	id := generateSpanID()
	assert.Len(t, id, 16) // 8 bytes = 16 hex chars
}
