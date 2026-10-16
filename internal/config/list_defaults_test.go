package config

import (
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// loadLikeCLI mirrors the standalone load: override defaults bound onto the
// singular form, optional singular values (as env would set them), Unmarshal,
// then ApplyListDefaults.
func loadLikeCLI(t *testing.T, yaml string, singular map[string]any) *Config {
	t.Helper()
	v := viperFrom(t, yaml)
	SetDefaults(v)
	for k, val := range singular {
		v.Set(k, val)
	}
	cfg := NewConfig()
	require.NoError(t, v.Unmarshal(cfg))
	require.NoError(t, ApplyListDefaults(v, cfg))
	return cfg
}

func TestApplyListDefaults_ExplicitZeroValuesKept(t *testing.T) {
	cfg := loadLikeCLI(t, `
outputs:
  - type: syslog
    syslog:
      host: h
      port: 514
      facility: 0
  - type: file
    file:
      path: /tmp/x.log
      rotation:
        compress: false
`, nil)

	outs := cfg.EffectiveOutputs()
	require.Len(t, outs, 2)
	assert.Equal(t, 0, outs[0].Syslog.Facility, "explicit facility 0 (kern) must be kept")
	assert.False(t, outs[1].File.Rotation.Compress, "explicit compress false must be kept")
}

func TestApplyListDefaults_UnsetFieldsGetTypeDefaults(t *testing.T) {
	cfg := loadLikeCLI(t, `
outputs:
  - type: tcp
    tcp:
      host: h
      port: 5000
  - type: syslog
    syslog:
      host: h
      port: 514
`, nil)

	outs := cfg.EffectiveOutputs()
	require.Len(t, outs, 2)
	assert.Equal(t, 1, outs[0].TCP.Workers, "unset workers gets the type default")
	assert.Equal(t, "h", outs[0].TCP.Host)
	assert.Equal(t, 5000, outs[0].TCP.Port)
	assert.Equal(t, 1, outs[1].Syslog.Facility, "unset facility gets the type default")
}

func TestApplyListDefaults_SingularValuesNeverReachListEntries(t *testing.T) {
	cfg := loadLikeCLI(t, `
outputs:
  - type: tcp
    tcp:
      host: h
      port: 5000
`, map[string]any{"output.tcp.workers": 9})

	outs := cfg.EffectiveOutputs()
	require.Len(t, outs, 1)
	assert.Equal(t, 1, outs[0].TCP.Workers, "singular output values must not leak into list entries")
}

// TestApplyListDefaults_DecodeErrorNamesOutputs pins that an outputs entry
// that cannot decode fails with the list named, not a bare decode error.
func TestApplyListDefaults_DecodeErrorNamesOutputs(t *testing.T) {
	v := viperFrom(t, "outputs:\n  - type: tcp\n    tcp:\n      workers: many\n")
	require.ErrorContains(t, ApplyListDefaults(v, NewConfig()), "decode outputs")
}

func TestApplyListDefaults_DecodeErrorNamesGenerators(t *testing.T) {
	v := viperFrom(t, "generators:\n  - type: json\n    json:\n      workers: many\n")
	require.ErrorContains(t, ApplyListDefaults(v, NewConfig()), "decode generators")
}

func TestApplyListDefaults_GeneratorExplicitFalseKept(t *testing.T) {
	cfg := loadLikeCLI(t, `
generators:
  - type: filegen
    filegen:
      source: /tmp/x.log
      cache-enabled: false
`, nil)

	gens := cfg.EffectiveGenerators()
	require.Len(t, gens, 1)
	assert.False(t, gens[0].Filegen.CacheEnabled, "explicit cache-enabled false (default true) must be kept")
}

func TestApplyListDefaults_GeneratorUnsetFieldsGetTypeDefaults(t *testing.T) {
	cfg := loadLikeCLI(t, `
generators:
  - type: json
    json:
      rate: 2s
`, nil)

	gens := cfg.EffectiveGenerators()
	require.Len(t, gens, 1)
	assert.Equal(t, 1, gens[0].JSON.Workers, "unset workers gets the type default")
	assert.Equal(t, 2*time.Second, gens[0].JSON.Rate, "user-set rate is kept")
}

func TestApplyListDefaults_SingularGeneratorValuesNeverReachListEntries(t *testing.T) {
	cfg := loadLikeCLI(t, `
generators:
  - type: json
`, map[string]any{"generator.json.workers": 9})

	gens := cfg.EffectiveGenerators()
	require.Len(t, gens, 1)
	assert.Equal(t, 1, gens[0].JSON.Workers, "singular generator values must not leak into list entries")
}

func TestValidate_GeneratorErrorNamesYAMLPosition(t *testing.T) {
	// The hostmetrics entry expands into two generators (one per OS), which
	// must not shift the index reported for the entry after it.
	cfg := loadLikeCLI(t, `
generators:
  - type: hostmetrics
    hostmetrics:
      os: linux,windows
  - type: json
    json:
      workers: -1
`, nil)

	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "generators[1]", "the index is the entry's position in the YAML list")
}

// assertNoSharedStorage fails if any slice or map field of a and b (walked
// recursively) points at the same backing storage.
func assertNoSharedStorage(t *testing.T, a, b reflect.Value, path string) {
	t.Helper()
	switch a.Kind() {
	case reflect.Struct:
		for i := range a.NumField() {
			assertNoSharedStorage(t, a.Field(i), b.Field(i), path+"."+a.Type().Field(i).Name)
		}
	case reflect.Slice:
		if a.Cap() > 0 && b.Cap() > 0 {
			assert.NotEqual(t, a.Pointer(), b.Pointer(), "%s shares a backing array across entries", path)
		}
	case reflect.Map:
		if !a.IsNil() && !b.IsNil() {
			assert.NotEqual(t, a.Pointer(), b.Pointer(), "%s shares a map across entries", path)
		}
	}
}

func TestApplyListDefaults_EntriesNeverShareStorage(t *testing.T) {
	cfg := loadLikeCLI(t, `
generators:
  - type: wel
  - type: wel
    wel:
      channels: [Security, System]
`, nil)

	require.Len(t, cfg.Generators, 2)
	defs, err := typeDefaults()
	require.NoError(t, err)
	assert.Equal(t, defs.Generator.Wel.Channels, cfg.Generators[0].Wel.Channels,
		"an entry's unset slice keeps the default, untouched by a sibling's value")
	assertNoSharedStorage(t, reflect.ValueOf(cfg.Generators[0]), reflect.ValueOf(cfg.Generators[1]), "Generator")
}
