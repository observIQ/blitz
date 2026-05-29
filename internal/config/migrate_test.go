package config_test

import (
	"bytes"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/observiq/blitz/internal/config"
)

func TestMigrateDeprecatedKeys_PaloAltoLegacyKeyMoves(t *testing.T) {
	v := viper.New()
	v.SetConfigType("yaml")
	yaml := []byte(`
generator:
  type: palo-alto
  paloAlto:
    workers: 7
    rate: 250ms
`)
	require.NoError(t, v.ReadConfig(bytes.NewReader(yaml)))

	config.MigrateDeprecatedKeys(v)

	cfg := config.NewConfig()
	require.NoError(t, v.Unmarshal(cfg))
	assert.Equal(t, 7, cfg.Generator.PaloAlto.Workers)
}

func TestMigrateDeprecatedKeys_OTLPGrpcLegacyKeyMoves(t *testing.T) {
	v := viper.New()
	v.SetConfigType("yaml")
	yaml := []byte(`
output:
  type: otlp-grpc
  otlpGrpc:
    host: legacy.example.com
    port: 4318
`)
	require.NoError(t, v.ReadConfig(bytes.NewReader(yaml)))

	config.MigrateDeprecatedKeys(v)

	cfg := config.NewConfig()
	require.NoError(t, v.Unmarshal(cfg))
	assert.Equal(t, "legacy.example.com", cfg.Output.OTLPGrpc.Host)
	assert.Equal(t, 4318, cfg.Output.OTLPGrpc.Port)
}

func TestMigrateDeprecatedKeys_CanonicalKeyWorks(t *testing.T) {
	v := viper.New()
	v.SetConfigType("yaml")
	yaml := []byte(`
generator:
  type: palo-alto
  palo-alto:
    workers: 4
    rate: 100ms
output:
  type: otlp-grpc
  otlp-grpc:
    host: new.example.com
    port: 4317
`)
	require.NoError(t, v.ReadConfig(bytes.NewReader(yaml)))

	config.MigrateDeprecatedKeys(v)

	cfg := config.NewConfig()
	require.NoError(t, v.Unmarshal(cfg))
	assert.Equal(t, 4, cfg.Generator.PaloAlto.Workers)
	assert.Equal(t, "new.example.com", cfg.Output.OTLPGrpc.Host)
}

func TestMigrateDeprecatedKeys_CanonicalWinsWhenBothSet(t *testing.T) {
	v := viper.New()
	v.SetConfigType("yaml")
	yaml := []byte(`
generator:
  type: palo-alto
  paloAlto:
    workers: 99
  palo-alto:
    workers: 4
`)
	require.NoError(t, v.ReadConfig(bytes.NewReader(yaml)))

	config.MigrateDeprecatedKeys(v)

	cfg := config.NewConfig()
	require.NoError(t, v.Unmarshal(cfg))
	assert.Equal(t, 4, cfg.Generator.PaloAlto.Workers)
}

// TestMigrateDeprecatedKeys_LegacyKeyMovesEvenWhenCanonicalHasDefaults
// reproduces the CLI path's regression eKuG flagged on PR #222:
// `Override.Bind` calls `v.SetDefault("output.otlp-grpc.host", "")`
// (and similar for every override) BEFORE the YAML is read. With an
// `IsSet`-based guard, `v.IsSet("output.otlp-grpc")` returns true
// because the defaults populate the sub-tree, and the migration
// short-circuits — silently dropping the user's legacy `otlpGrpc:`
// values. The fix is to gate on `v.InConfig`, which only sees keys
// from the parsed config file/bytes, not defaults / env / flags.
func TestMigrateDeprecatedKeys_LegacyKeyMovesEvenWhenCanonicalHasDefaults(t *testing.T) {
	v := viper.New()
	v.SetConfigType("yaml")

	// Simulate the CLI path: defaults bound for the canonical sub-tree
	// at flag-init time, BEFORE the user's YAML is read.
	v.SetDefault("output.otlp-grpc.host", "")
	v.SetDefault("output.otlp-grpc.port", 4317)
	v.SetDefault("output.otlp-grpc.workers", 1)
	v.SetDefault("generator.palo-alto.workers", 1)
	v.SetDefault("generator.palo-alto.rate", "1s")

	yaml := []byte(`
generator:
  type: palo-alto
  paloAlto:
    workers: 11
    rate: 250ms
output:
  type: otlp-grpc
  otlpGrpc:
    host: legacy.example.com
    port: 4318
    workers: 5
`)
	require.NoError(t, v.ReadConfig(bytes.NewReader(yaml)))

	config.MigrateDeprecatedKeys(v)

	cfg := config.NewConfig()
	require.NoError(t, v.Unmarshal(cfg))
	assert.Equal(t, 11, cfg.Generator.PaloAlto.Workers,
		"legacy paloAlto.workers must override the bound default")
	assert.Equal(t, "legacy.example.com", cfg.Output.OTLPGrpc.Host,
		"legacy otlpGrpc.host must override the bound default")
	assert.Equal(t, 4318, cfg.Output.OTLPGrpc.Port,
		"legacy otlpGrpc.port must override the bound default")
	assert.Equal(t, 5, cfg.Output.OTLPGrpc.Workers,
		"legacy otlpGrpc.workers must override the bound default")
}
