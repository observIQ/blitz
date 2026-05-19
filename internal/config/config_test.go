package config

import (
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
