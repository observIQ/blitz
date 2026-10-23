package traces

import (
	"context"
	"errors"
	"maps"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/observiq/blitz/embed"
	"github.com/observiq/blitz/generator/count"
	"github.com/observiq/blitz/internal/datagen"
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

// TestNewProjectsIdentityResource confirms that when a resolved datagen
// identity is supplied, the generator's static resource carries the full
// host.* / os.* / deployment.* projection, so spans describe the simulated
// host rather than just its name.
func TestNewProjectsIdentityResource(t *testing.T) {
	cfg := baseCfg(t, &mockTraceConsumer{})
	cfg.Identity = &datagen.SystemIdentity{
		Hostname: "odin-api-01",
		HostID:   "def456",
		Arch:     datagen.ArchARM64,
		Tier:     datagen.TierStaging,
		OSInfo:   datagen.OSInfo{Type: datagen.OSLinux, Name: "Ubuntu", Version: "22.04.5"},
	}

	g, err := New(cfg)
	require.NoError(t, err)

	res := g.static.Record()
	assert.Equal(t, "odin-api-01", res["host.name"])
	assert.Equal(t, "def456", res["host.id"])
	assert.Equal(t, "arm64", res["host.arch"])
	assert.Equal(t, "linux", res["os.type"])
	assert.Equal(t, "staging", res["deployment.environment.name"])
	assert.Equal(t, "traces", res["telemetry.source"])
	assert.Equal(t, "odin-api-01", g.hostname)
}

// TestExplicitHostnameOverridesIdentityHostname confirms a set Hostname
// replaces only the resolved identity's hostname: the OS and other identity
// attributes stay, and the shared identity is not mutated.
func TestExplicitHostnameOverridesIdentityHostname(t *testing.T) {
	cfg := baseCfg(t, &mockTraceConsumer{})
	cfg.Hostname = "explicit-01"
	id := &datagen.SystemIdentity{
		Hostname: "identity-01",
		HostID:   "id-9",
		OSInfo:   datagen.OSInfo{Type: datagen.OSLinux},
	}
	cfg.Identity = id

	g, err := New(cfg)
	require.NoError(t, err)

	res := g.static.Record()
	assert.Equal(t, "explicit-01", res["host.name"])
	assert.Equal(t, "id-9", res["host.id"])
	assert.Equal(t, "linux", res["os.type"])
	assert.Equal(t, "explicit-01", g.hostname)
	assert.Equal(t, "identity-01", id.Hostname, "shared identity must not be mutated")
}

// TestSyntheticIdentityRandomSeed exercises the randomize branch of the
// synthetic-identity hostname generation (Seed < 0 → wall-clock seed).
func TestSyntheticIdentityRandomSeed(t *testing.T) {
	cfg := baseCfg(t, &mockTraceConsumer{})
	cfg.Hostname = ""
	cfg.Seed = -1

	g, err := New(cfg)
	require.NoError(t, err)
	assert.NotEmpty(t, g.hostname)
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

	a := spansByTrace(runOnce())
	b := spansByTrace(runOnce())

	n := min(len(a), len(b))
	require.Positive(t, n, "neither run produced a trace")

	// Compare deterministic fields. Span names ("GET /api/users",
	// "SELECT users", etc.) and Kind flow from the seeded RNG. TraceID /
	// SpanID use crypto/rand and are intentionally non-deterministic per
	// OTel spec.
	for i := range n {
		assert.Equal(t, a[i], b[i], "trace %d differs across seeded runs", i)
	}
}

// spansByTrace groups spans into traces in start order, listing each trace's
// span names and kinds sorted. A trace's spans are sent on their own timers,
// so they can arrive in either order.
func spansByTrace(spans []embed.Span) [][]string {
	start := map[string]time.Time{}
	shape := map[string][]string{}
	for _, sp := range spans {
		if s, ok := start[sp.TraceID]; !ok || sp.StartTime.Before(s) {
			start[sp.TraceID] = sp.StartTime
		}
		shape[sp.TraceID] = append(shape[sp.TraceID], sp.Name+"|"+string(sp.Kind))
	}
	ids := slices.Collect(maps.Keys(start))
	slices.SortFunc(ids, func(x, y string) int { return start[x].Compare(start[y]) })
	out := make([][]string, len(ids))
	for i, id := range ids {
		slices.Sort(shape[id])
		out[i] = shape[id]
	}
	return out
}

func TestGenerateTraceID(t *testing.T) {
	id := generateTraceID()
	assert.Len(t, id, 32) // 16 bytes = 32 hex chars
}

func TestGenerateSpanID(t *testing.T) {
	id := generateSpanID()
	assert.Len(t, id, 16) // 8 bytes = 16 hex chars
}

// TestScheduleEmission_PastEndSendsAtOnce pins that a span whose end time has
// already passed is sent at once and still settles its trace's permit.
func TestScheduleEmission_PastEndSendsAtOnce(t *testing.T) {
	consumer := &mockTraceConsumer{}
	g, err := New(baseCfg(t, consumer))
	require.NoError(t, err)
	tracker := count.NewTracker(1)
	permit, _, ok := tracker.Acquire()
	require.True(t, ok)
	ctx := count.WithPermit(context.Background(), permit)

	g.scheduleEmission(ctx, embed.Span{Name: "late", EndTime: time.Now().Add(-time.Second)})
	permit.Commit()
	g.wg.Wait()
	assert.Len(t, consumer.Arrivals(), 1)
	select {
	case <-tracker.Done():
	default:
		t.Fatal("the span's trace was not counted")
	}
}

// rejectingTraceConsumer rejects the span named reject and accepts the rest.
type rejectingTraceConsumer struct {
	mockTraceConsumer
	reject string
}

func (r *rejectingTraceConsumer) ConsumeTraces(ctx context.Context, batch []embed.Span) error {
	for _, sp := range batch {
		if sp.Name == r.reject {
			return errors.New("rejected")
		}
	}
	return r.mockTraceConsumer.ConsumeTraces(ctx, batch)
}

// TestScheduleEmission_FailedSpanReleasesTrace pins that one rejected span
// fails its whole trace's permit, so a trace with a lost span is not counted.
func TestScheduleEmission_FailedSpanReleasesTrace(t *testing.T) {
	consumer := &rejectingTraceConsumer{reject: "lost"}
	g, err := New(baseCfg(t, consumer))
	require.NoError(t, err)
	tracker := count.NewTracker(1)
	permit, _, ok := tracker.Acquire()
	require.True(t, ok)
	ctx := count.WithPermit(context.Background(), permit)

	past := time.Now().Add(-time.Second)
	g.scheduleEmission(ctx, embed.Span{Name: "kept", EndTime: past})
	g.scheduleEmission(ctx, embed.Span{Name: "lost", EndTime: past})
	permit.Commit()
	g.wg.Wait()
	assert.Len(t, consumer.Arrivals(), 1, "the other span still arrived")
	select {
	case <-permit.Settled():
	default:
		t.Fatal("the trace's permit never settled")
	}
	assert.False(t, permit.Delivered(), "a trace with a lost span is not counted")
}
