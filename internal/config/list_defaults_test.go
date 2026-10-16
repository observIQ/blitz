package config

import (
	"testing"

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
