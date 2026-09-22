package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEffectiveGenerators_ListEntriesInheritDefaults(t *testing.T) {
	// c.Generator is the viper-defaulted template (workers filled). A list
	// entry that omits workers must inherit that default; user-set fields win.
	cfg := &Config{
		Generator: Generator{JSON: JSONGeneratorConfig{Workers: 1}},
		Generators: []Generator{
			{Type: GeneratorTypeJSON, JSON: JSONGeneratorConfig{Rate: time.Second}},
		},
	}

	gens := cfg.EffectiveGenerators()
	require.Len(t, gens, 1)
	assert.Equal(t, 1, gens[0].JSON.Workers, "unset workers should inherit the template default")
	assert.Equal(t, time.Second, gens[0].JSON.Rate, "user-set rate must be preserved")
}

func TestEffectiveGenerators_ListEntryUserValueOverridesDefault(t *testing.T) {
	cfg := &Config{
		Generator: Generator{JSON: JSONGeneratorConfig{Workers: 1}},
		Generators: []Generator{
			{Type: GeneratorTypeJSON, JSON: JSONGeneratorConfig{Workers: 8, Rate: time.Second}},
		},
	}

	gens := cfg.EffectiveGenerators()
	assert.Equal(t, 8, gens[0].JSON.Workers, "explicit workers must win over the template default")
}

func TestValidateExclusiveGeneratorConfig_BothSetFails(t *testing.T) {
	v := viperFrom(t, "generator:\n  type: json\ngenerators:\n  - type: json\n")
	require.Error(t, ValidateExclusiveGeneratorConfig(v))
}

func TestValidateExclusiveGeneratorConfig_OnlyGeneratorsOK(t *testing.T) {
	v := viperFrom(t, "generators:\n  - type: json\n")
	require.NoError(t, ValidateExclusiveGeneratorConfig(v))
}

func TestValidateExclusiveGeneratorConfig_OnlyGeneratorOK(t *testing.T) {
	v := viperFrom(t, "generator:\n  type: json\n")
	require.NoError(t, ValidateExclusiveGeneratorConfig(v))
}
