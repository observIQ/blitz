package config_test

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/observiq/blitz/config"
	"github.com/observiq/blitz/embed"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

// seedBatches is how many batches a run fingerprints; several so a chance
// match between two random runs cannot pass the test.
const seedBatches = 5

// firstBatches records a content fingerprint of the first seedBatches batches.
type firstBatches struct {
	mu  sync.Mutex
	got []string
}

func (c *firstBatches) set(fp string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.got) < seedBatches {
		c.got = append(c.got, fp)
	}
}

func (c *firstBatches) done() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.got) == seedBatches
}

func (c *firstBatches) get() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string(nil), c.got...)
}

func (c *firstBatches) ConsumeMetrics(_ context.Context, points []embed.MetricPoint) error {
	var b strings.Builder
	for _, p := range points {
		fmt.Fprint(&b, p.Name, "=")
		if p.IntValue != nil {
			fmt.Fprint(&b, *p.IntValue)
		}
		if p.DoubleValue != nil {
			fmt.Fprint(&b, *p.DoubleValue)
		}
		b.WriteString(";")
	}
	c.set(b.String())
	return nil
}

func (c *firstBatches) ConsumeTraces(_ context.Context, spans []embed.Span) error {
	var b strings.Builder
	// TraceID/SpanID come from crypto/rand by design; name and kind are seeded.
	for _, s := range spans {
		fmt.Fprint(&b, s.Name, "/", s.Kind, ";")
	}
	c.set(b.String())
	return nil
}

// seedYAMLTail is the non-generator config every embedded load must carry. The
// environment seed is pinned so host identity is fixed and only the
// generator's own seed decides record content.
const seedYAMLTail = "environment:\n  seed_config: {shared: 7}\noutput:\n  type: nop\nlogging:\n  type: stdout\nmetrics:\n  port: 19000\n"

// firstContent builds the YAML's single generator, runs it until seedBatches
// batches arrive, and returns their fingerprints.
func firstContent(t *testing.T, yaml []byte) []string {
	t.Helper()
	c := &firstBatches{}
	mods, err := config.LoadModules(append(yaml, seedYAMLTail...), config.EmbedOpts{Logger: zap.NewNop(), MetricConsumer: c, TraceConsumer: c})
	require.NoError(t, err)
	require.Len(t, mods, 1)
	require.NoError(t, mods[0].Start(context.Background()))
	require.Eventually(t, c.done, 2*time.Second, 10*time.Millisecond)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	require.NoError(t, mods[0].Stop(ctx))
	return c.get()
}

// TestSeedZeroIsDeterministic pins the datagen seed contract: an explicit
// `seed: 0` is a real deterministic seed, so two runs emit the same first batches.
func TestSeedZeroIsDeterministic(t *testing.T) {
	yaml := []byte("generator:\n  type: hostmetrics\n  hostmetrics: {workers: 1, rate: 10ms, os: linux, seed: 0}\n")
	require.Equal(t, firstContent(t, yaml), firstContent(t, yaml))
}

// TestSeedZeroReachesTheTracesGenerator pins the same contract for traces,
// which emits each span at its wall-clock end time, so two runs' arrival order
// can differ: the generator must start on seed 0, not a random one.
func TestSeedZeroReachesTheTracesGenerator(t *testing.T) {
	core, logs := observer.New(zap.InfoLevel)
	mods, err := config.LoadModules(append([]byte("generator:\n  type: traces\n  traces: {workers: 1, rate: 1s, seed: 0}\n"), seedYAMLTail...),
		config.EmbedOpts{Logger: zap.New(core), TraceConsumer: &firstBatches{}})
	require.NoError(t, err)
	require.Len(t, mods, 1)
	require.NoError(t, mods[0].Start(context.Background()))
	require.NoError(t, mods[0].Stop(context.Background()))
	started := logs.FilterMessage("Starting traces generator").All()
	require.Len(t, started, 1)
	require.Equal(t, int64(0), started[0].ContextMap()["seed"])
}

// TestSeedOmittedResolvesRandom pins the other half of the contract for the
// embedded loader: an omitted seed resolves to -1 (randomize), not 0.
func TestSeedOmittedResolvesRandom(t *testing.T) {
	cfg, err := config.Load([]byte("generator:\n  type: hostmetrics\n  hostmetrics: {workers: 1, rate: 1s, os: linux}\n"+seedYAMLTail), config.LoadOpts{})
	require.NoError(t, err)
	require.Equal(t, int64(-1), cfg.Generator.HostMetrics.Seed, "hostmetrics")

	cfg, err = config.Load([]byte("generator:\n  type: traces\n  traces: {workers: 1, rate: 1s}\n"+seedYAMLTail), config.LoadOpts{})
	require.NoError(t, err)
	require.Equal(t, int64(-1), cfg.Generator.Traces.Seed, "traces")
}

// TestSeedEnvOverrideWins: a host-supplied EnvOverrides seed beats both the
// -1 default and a YAML seed, and an explicit 0 survives as 0.
func TestSeedEnvOverrideWins(t *testing.T) {
	for name, tc := range map[string]struct {
		yaml, env string
		want      int64
	}{
		"zero over default":     {yaml: "{workers: 1, rate: 1s, os: linux}", env: "0", want: 0},
		"negative over default": {yaml: "{workers: 1, rate: 1s, os: linux}", env: "-5", want: -5},
		"positive over yaml":    {yaml: "{workers: 1, rate: 1s, os: linux, seed: 7}", env: "42", want: 42},
	} {
		t.Run(name, func(t *testing.T) {
			cfg, err := config.Load([]byte("generator:\n  type: hostmetrics\n  hostmetrics: "+tc.yaml+"\n"+seedYAMLTail), config.LoadOpts{
				EnvOverrides: map[string]string{"generator.hostmetrics.seed": tc.env},
			})
			require.NoError(t, err)
			require.Equal(t, tc.want, cfg.Generator.HostMetrics.Seed)
		})
	}
}

// TestEnvironmentSeedViaEnvOverrides backs the docs/datagen.md claim that
// embedded hosts can set environment.seed_config keys through EnvOverrides.
func TestEnvironmentSeedViaEnvOverrides(t *testing.T) {
	cfg, err := config.Load([]byte("generator:\n  type: hostmetrics\n  hostmetrics: {workers: 1, rate: 1s, os: linux}\n"+seedYAMLTail), config.LoadOpts{
		EnvOverrides: map[string]string{"environment.seed_config.users": "0"},
	})
	require.NoError(t, err)
	require.NotNil(t, cfg.Environment.SeedConfig.Users)
	require.Equal(t, int64(0), *cfg.Environment.SeedConfig.Users)
}
