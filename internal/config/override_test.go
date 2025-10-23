package config

import (
	"strings"
	"testing"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func TestOverrideDefaults(t *testing.T) {
	flagSet := pflag.NewFlagSet("test", pflag.PanicOnError)
	overrides := DefaultOverrides()
	for _, override := range overrides {
		require.NoError(t, override.Bind(flagSet))
	}

	viper.SetConfigType("yaml")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	cfg := NewConfig()
	err := viper.Unmarshal(cfg)
	require.NoError(t, err)

	// build expected config and compare full struct
	expectedCfg := &Config{
		Logging: Logging{Type: LoggingTypeStdout, Level: LogLevelInfo},
	}
	require.Equal(t, expectedCfg, cfg)
}

func TestOverrideFlags(t *testing.T) {
	flagSet := pflag.NewFlagSet("test", pflag.PanicOnError)
	args := []string{
		"--logging-type", "stdout",
		"--logging-level", "warn",
	}

	overrides := DefaultOverrides()
	for _, override := range overrides {
		require.NoError(t, override.Bind(flagSet))
	}

	require.NoError(t, flagSet.Parse(args))

	viper.SetConfigType("yaml")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	cfg := NewConfig()
	err := viper.Unmarshal(cfg)
	require.NoError(t, err)

	// build expected config and compare full struct
	expectedCfg := &Config{
		Logging: Logging{Type: LoggingTypeStdout, Level: LogLevelWarn},
	}
	require.Equal(t, expectedCfg, cfg)
}

func TestOverrideEnvs(t *testing.T) {
	t.Setenv("BINDPLANE_LOGGING_TYPE", "stdout")
	t.Setenv("BINDPLANE_LOGGING_LEVEL", "error")

	flagSet := pflag.NewFlagSet("test", pflag.PanicOnError)
	overrides := DefaultOverrides()
	for _, override := range overrides {
		require.NoError(t, override.Bind(flagSet))
	}

	viper.SetConfigType("yaml")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	cfg := NewConfig()
	err := viper.Unmarshal(cfg)
	require.NoError(t, err)

	// build expected config and compare full struct
	expectedCfg := &Config{
		Logging: Logging{Type: LoggingTypeStdout, Level: LogLevelError},
	}
	require.Equal(t, expectedCfg, cfg)
}
