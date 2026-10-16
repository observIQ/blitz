package config

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEffectiveGenerators_SingleGenerator(t *testing.T) {
	cfg := &Config{
		Generator: Generator{
			Type: GeneratorTypeJSON,
			JSON: JSONGeneratorConfig{
				Workers: 1,
				Rate:    time.Second,
			},
		},
	}

	gens := cfg.EffectiveGenerators()
	require.Len(t, gens, 1)
	assert.Equal(t, GeneratorTypeJSON, gens[0].Type)
}

func TestEffectiveGenerators_MultiGenerator(t *testing.T) {
	cfg := &Config{
		Generators: []Generator{
			{
				Type: GeneratorTypeHostMetrics,
				HostMetrics: HostMetricsGeneratorConfig{
					Workers: 1,
					Rate:    time.Second,
					OS:      "linux",
				},
			},
			{
				Type: GeneratorTypeTraces,
				Traces: TracesGeneratorConfig{
					Workers: 1,
					Rate:    time.Second,
				},
			},
		},
	}

	gens := cfg.EffectiveGenerators()
	require.Len(t, gens, 2)
	assert.Equal(t, GeneratorTypeHostMetrics, gens[0].Type)
	assert.Equal(t, GeneratorTypeTraces, gens[1].Type)
}

func TestEffectiveGenerators_MultiOverridesSingle(t *testing.T) {
	cfg := &Config{
		Generator: Generator{
			Type: GeneratorTypeJSON,
		},
		Generators: []Generator{
			{Type: GeneratorTypeHostMetrics},
			{Type: GeneratorTypeTraces},
		},
	}

	gens := cfg.EffectiveGenerators()
	require.Len(t, gens, 2, "Generators field should take precedence")
}

func TestEffectiveGenerators_ExpandsHostmetricsOS(t *testing.T) {
	cfg := &Config{
		Generators: []Generator{
			{
				Type: GeneratorTypeHostMetrics,
				HostMetrics: HostMetricsGeneratorConfig{
					Workers: 1,
					Rate:    time.Second,
					OS:      "linux,windows",
				},
			},
		},
	}

	gens := cfg.EffectiveGenerators()
	require.Len(t, gens, 2)
	assert.Equal(t, "linux", gens[0].HostMetrics.OS)
	assert.Equal(t, "windows", gens[1].HostMetrics.OS)
}

func TestEffectiveGenerators_Empty(t *testing.T) {
	cfg := &Config{}

	gens := cfg.EffectiveGenerators()
	require.Len(t, gens, 1, "should wrap empty Generator in slice")
}

// TestExpandedHostMetricsGetDistinctSeeds pins that one hostmetrics entry
// expanded across several OSes yields distinct deterministic seeds, so the
// hosts do not emit identical values; a random seed stays random per clone.
func TestExpandedHostMetricsGetDistinctSeeds(t *testing.T) {
	gen := func(seed int64) Generator {
		return Generator{Type: GeneratorTypeHostMetrics, HostMetrics: HostMetricsGeneratorConfig{OS: "linux,windows,macos", Seed: seed}}
	}
	got := expandGenerators([]Generator{gen(0)})
	require.Len(t, got, 3)
	seen := map[int64]bool{}
	for _, g := range got {
		require.GreaterOrEqual(t, g.HostMetrics.Seed, int64(0), "a deterministic seed stays deterministic")
		seen[g.HostMetrics.Seed] = true
	}
	require.Len(t, seen, 3, "each expanded host needs its own seed")
	require.Equal(t, got, expandGenerators([]Generator{gen(0)}), "expansion is deterministic")

	near := expandGenerators([]Generator{gen(math.MaxInt64 - 5)})
	for _, g := range near {
		require.GreaterOrEqual(t, g.HostMetrics.Seed, int64(0), "a seed near MaxInt64 must not overflow into random")
	}
}

// TestExpandedRandomSeedReplays pins that a random seed on an expanded
// hostmetrics entry resolves once for the entry, so the seed the first host
// logs replays every host: rerunning with it reproduces all their seeds.
func TestExpandedRandomSeedReplays(t *testing.T) {
	cfg := &Config{Generators: []Generator{{Type: GeneratorTypeHostMetrics, HostMetrics: HostMetricsGeneratorConfig{OS: "linux,windows", Seed: -1}}}}
	cfg.ResolveExpandedSeeds()
	got := cfg.EffectiveGenerators()
	require.Len(t, got, 2)
	logged := got[0].HostMetrics.Seed
	require.GreaterOrEqual(t, logged, int64(0))
	require.Equal(t, got, cfg.EffectiveGenerators(), "every call sees the same seeds")

	replay := &Config{Generators: []Generator{{Type: GeneratorTypeHostMetrics, HostMetrics: HostMetricsGeneratorConfig{OS: "linux,windows", Seed: logged}}}}
	replay.ResolveExpandedSeeds()
	require.Equal(t, got, replay.EffectiveGenerators())
}

// TestRunLevelCountIsValidatedNextToAList pins that a run-level
// generator.count next to a generators: list is validated like the singular
// form's: a negative count is rejected, not silently unlimited.
func TestRunLevelCountIsValidatedNextToAList(t *testing.T) {
	list := "generators:\n  - type: nop\n"
	cfg := loadLikeCLI(t, list, map[string]any{"generator.count": -3})
	require.ErrorContains(t, cfg.Validate(), "generator count must be 0 or greater, got -3")

	cfg = loadLikeCLI(t, list, map[string]any{"generator.count": 3})
	require.NoError(t, cfg.Validate())
}
