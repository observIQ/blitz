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
		Output: Output{
			UDP: UDPOutputConfig{Workers: 1},
			TCP: TCPOutputConfig{Workers: 1},
		},
	}
	require.Equal(t, expectedCfg, cfg)
}

func TestOverrideFlags(t *testing.T) {
	flagSet := pflag.NewFlagSet("test", pflag.PanicOnError)
	args := []string{
		"--logging-type", "stdout",
		"--logging-level", "warn",
		"--output-type", "tcp",
		"--output-tcp-host", "127.0.0.1",
		"--output-tcp-port", "9090",
		"--output-tcp-workers", "3",
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
		Output: Output{
			Type: OutputTypeTCP,
			UDP:  UDPOutputConfig{Workers: 1},
			TCP: TCPOutputConfig{
				Host:    "127.0.0.1",
				Port:    9090,
				Workers: 3,
			},
		},
	}
	require.Equal(t, expectedCfg, cfg)
}

func TestOverrideEnvs(t *testing.T) {
	t.Setenv("BINDPLANE_LOGGING_TYPE", "stdout")
	t.Setenv("BINDPLANE_LOGGING_LEVEL", "error")
	t.Setenv("BINDPLANE_OUTPUT_TYPE", "udp")
	t.Setenv("BINDPLANE_OUTPUT_UDP_HOST", "example.com")
	t.Setenv("BINDPLANE_OUTPUT_UDP_PORT", "8080")
	t.Setenv("BINDPLANE_OUTPUT_UDP_WORKERS", "4")

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
		Output: Output{
			Type: OutputTypeUDP,
			UDP: UDPOutputConfig{
				Host:    "example.com",
				Port:    8080,
				Workers: 4,
			},
			TCP: TCPOutputConfig{Workers: 1},
		},
	}
	require.Equal(t, expectedCfg, cfg)
}
