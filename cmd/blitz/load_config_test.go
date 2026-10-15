package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func writeConfig(t *testing.T, yaml string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "blitz.yaml")
	require.NoError(t, os.WriteFile(path, []byte(yaml), 0o600))
	return path
}

// TestLoadConfigReadsSeedEnvVars pins that a seed_config env var applies
// when the YAML leaves the key out.
func TestLoadConfigReadsSeedEnvVars(t *testing.T) {
	t.Setenv("BLITZ_ENVIRONMENT_SEED_CONFIG_USERS", "42")
	cfg, err := loadConfig(viper.New(), writeConfig(t, "generator:\n  type: json\n"))
	require.NoError(t, err)
	require.NotNil(t, cfg.Environment.SeedConfig.Users)
	require.Equal(t, int64(42), *cfg.Environment.SeedConfig.Users)
}

// TestLoadConfigDefaultsListSeeds pins that a list entry without a seed gets
// the random seed default rather than a deterministic 0.
func TestLoadConfigDefaultsListSeeds(t *testing.T) {
	cfg, err := loadConfig(viper.New(), writeConfig(t, "generators:\n  - type: hostmetrics\n"))
	require.NoError(t, err)
	require.Equal(t, int64(-1), cfg.Generators[0].HostMetrics.Seed)
}

// TestLoadConfigResolvesExpandedSeeds pins that a random seed on an entry
// expanding across OSes is resolved once at load.
func TestLoadConfigResolvesExpandedSeeds(t *testing.T) {
	cfg, err := loadConfig(viper.New(), writeConfig(t, "generators:\n  - type: hostmetrics\n    hostmetrics:\n      os: linux,windows\n"))
	require.NoError(t, err)
	require.GreaterOrEqual(t, cfg.Generators[0].HostMetrics.Seed, int64(0))
}

func TestLoadConfigReportsAnUnreadableFile(t *testing.T) {
	_, err := loadConfig(viper.New(), filepath.Join(t.TempDir(), "missing.yaml"))
	require.ErrorContains(t, err, "failed to read config file")
}
