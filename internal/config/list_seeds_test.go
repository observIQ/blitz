package config

import (
	"strings"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

// TestDefaultListSeeds pins the list seed default for every entry shape: a
// bare "- type: hostmetrics" and an entry with a body but no seed both get
// -1 (random), and an explicit seed, including 0, is kept.
func TestDefaultListSeeds(t *testing.T) {
	v := viper.New()
	v.SetConfigType("yaml")
	require.NoError(t, v.ReadConfig(strings.NewReader(`generators:
  - type: hostmetrics
  - type: hostmetrics
    hostmetrics:
      os: linux
  - type: hostmetrics
    hostmetrics:
      seed: 0
`)))
	DefaultListSeeds(v)
	var gens []Generator
	require.NoError(t, v.UnmarshalKey("generators", &gens))
	require.Len(t, gens, 3)
	require.Equal(t, int64(-1), gens[0].HostMetrics.Seed, "bare entry")
	require.Equal(t, int64(-1), gens[1].HostMetrics.Seed, "entry without a seed")
	require.Equal(t, int64(0), gens[2].HostMetrics.Seed, "explicit seed 0")
}

// TestDefaultListSeedsSkipsMalformedEntries pins that an entry that is not a
// map, or names no type with a seed, is left as written for validation to
// report, rather than given a seed, and that a config without a list is left
// alone.
func TestDefaultListSeedsSkipsMalformedEntries(t *testing.T) {
	v := viper.New()
	v.SetConfigType("yaml")
	require.NoError(t, v.ReadConfig(strings.NewReader(`generators:
  - not-a-map
  - type: no-such-type
`)))
	DefaultListSeeds(v)
	list := v.Get("generators").([]any)
	require.Equal(t, "not-a-map", list[0])
	require.Equal(t, map[string]any{"type": "no-such-type"}, list[1])

	empty := viper.New()
	DefaultListSeeds(empty)
	require.False(t, empty.IsSet("generators"), "no list, nothing to seed")
}

// TestDefaultListSeedsLeavesNoOverride pins that the seeded list lives only in
// the config just read: a later config without a list (as the next run in the
// same process reads) has none, rather than inheriting the previous one.
func TestDefaultListSeedsLeavesNoOverride(t *testing.T) {
	v := viper.New()
	v.SetConfigType("yaml")
	require.NoError(t, v.ReadConfig(strings.NewReader(`generators:
  - type: hostmetrics
`)))
	DefaultListSeeds(v)
	require.NoError(t, v.ReadConfig(strings.NewReader(`generator:
  type: hostmetrics
`)))
	require.False(t, v.IsSet("generators"))
}
