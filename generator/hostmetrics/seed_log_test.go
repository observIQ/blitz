package hostmetrics

import (
	"context"
	"testing"
	"time"

	"github.com/observiq/blitz/embed"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

// TestResolvedSeedIsLoggedAndReproduces pins the datagen seed promise: a
// randomized run logs the one seed it resolved, and rerunning with that seed
// reproduces the run.
func TestResolvedSeedIsLoggedAndReproduces(t *testing.T) {
	run := func(seed int64) (int64, []embed.MetricPoint) {
		core, logs := observer.New(zap.InfoLevel)
		cons := &mockMetricConsumer{}
		cfg := baseCfg(t, cons)
		cfg.Logger = zap.New(core)
		cfg.Hostname = ""
		cfg.Rate = 20 * time.Millisecond
		cfg.Seed = seed
		cfg.ScraperNames = []string{"cpu", "memory"}
		g, err := New(cfg)
		require.NoError(t, err)
		require.NoError(t, g.Start(context.Background()))
		require.Eventually(t, func() bool { return cons.Count() >= 6 }, 2*time.Second, 10*time.Millisecond)
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		require.NoError(t, g.Stop(ctx))

		started := logs.FilterMessage("Starting host metrics generator").All()
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
	for i := 0; i < 6; i++ {
		require.Equal(t, a[i].Name, b[i].Name, "point %d", i)
		require.Equal(t, a[i].IntValue, b[i].IntValue, "point %d (%s)", i, a[i].Name)
		require.Equal(t, a[i].DoubleValue, b[i].DoubleValue, "point %d (%s)", i, a[i].Name)
	}
}
