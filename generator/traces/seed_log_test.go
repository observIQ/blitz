package traces

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/observiq/blitz/embed"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

// TestResolvedSeedIsLoggedAndReproduces pins the datagen seed promise: a
// randomized run logs the one seed it resolved, and rerunning with that seed
// reproduces the run (TraceID/SpanID excepted, which are crypto/rand).
func TestResolvedSeedIsLoggedAndReproduces(t *testing.T) {
	run := func(seed int64) (int64, []embed.Span) {
		core, logs := observer.New(zap.InfoLevel)
		cons := &mockTraceConsumer{}
		cfg := baseCfg(t, cons)
		cfg.Logger = zap.New(core)
		cfg.Rate = 20 * time.Millisecond
		cfg.Seed = seed
		g, err := New(cfg)
		require.NoError(t, err)
		require.NoError(t, g.Start(context.Background()))
		require.Eventually(t, func() bool { return cons.Count() >= 4 }, 3*time.Second, 10*time.Millisecond)
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		require.NoError(t, g.Stop(ctx))

		started := logs.FilterMessage("Starting traces generator").All()
		require.Len(t, started, 1)
		logged, ok := started[0].ContextMap()["seed"].(int64)
		require.True(t, ok, "start log has no int64 seed field: %v", started[0].ContextMap())
		return logged, cons.Snapshot()
	}

	resolved, a := run(-1)
	require.GreaterOrEqual(t, resolved, int64(0), "a randomized run must log the non-negative seed it resolved")
	again, b := run(resolved)
	require.Equal(t, resolved, again)

	require.Equal(t, a[0].Metadata.Resource["host.name"], b[0].Metadata.Resource["host.name"], "synthetic hostname must reproduce")
	require.Equal(t, firstTrace(a), firstTrace(b), "the first trace's spans must reproduce")
}

// firstTrace returns the sorted name/kind of every span in the earliest-started
// trace. Spans arrive on wall-clock timers, so delivery order is not
// seed-determined; Stop drains every scheduled span, so the trace is whole.
func firstTrace(spans []embed.Span) []string {
	first := spans[0]
	for _, sp := range spans {
		if sp.StartTime.Before(first.StartTime) {
			first = sp
		}
	}
	var out []string
	for _, sp := range spans {
		if sp.TraceID == first.TraceID {
			out = append(out, sp.Name+"/"+string(sp.Kind))
		}
	}
	slices.Sort(out)
	return out
}
