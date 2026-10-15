package config

import (
	"strings"
	"testing"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func viperFrom(t *testing.T, yaml string) *viper.Viper {
	t.Helper()
	v := viper.New()
	v.SetConfigType("yaml")
	require.NoError(t, v.ReadConfig(strings.NewReader(yaml)))
	return v
}

func TestValidateExclusiveOutputConfig_BothSetFails(t *testing.T) {
	v := viperFrom(t, "output:\n  type: stdout\noutputs:\n  - type: nop\n")
	require.Error(t, ValidateExclusiveOutputConfig(v, nil))
}

func TestValidateExclusiveOutputConfig_OnlyOutputsOK(t *testing.T) {
	v := viperFrom(t, "outputs:\n  - type: nop\n")
	require.NoError(t, ValidateExclusiveOutputConfig(v, nil))
}

func TestValidateExclusiveOutputConfig_OnlyOutputOK(t *testing.T) {
	v := viperFrom(t, "output:\n  type: stdout\n")
	require.NoError(t, ValidateExclusiveOutputConfig(v, nil))
}

func TestValidateExclusiveOutputConfig_NeitherOK(t *testing.T) {
	v := viperFrom(t, "logging:\n  type: stdout\n")
	require.NoError(t, ValidateExclusiveOutputConfig(v, nil))
}

func TestValidateExclusiveOutputConfig_RejectsBadListForms(t *testing.T) {
	for name, tc := range map[string]struct{ yaml, want string }{
		"not a list":    {"outputs: stdout\n", "outputs must be a list of output entries"},
		"map not list":  {"outputs:\n  type: stdout\n", "outputs must be a list of output entries"},
		"empty list":    {"outputs: []\n", "outputs must not be empty"},
		"entry no type": {"outputs:\n  - tcp:\n      host: h\n", "outputs[0]: type is required"},
		"scalar entry":  {"outputs:\n  - stdout\n", "outputs[0] must be a map"},
	} {
		t.Run(name, func(t *testing.T) {
			err := ValidateExclusiveOutputConfig(viperFrom(t, tc.yaml), nil)
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.want)
		})
	}
}

// TestValidateExclusiveOutputConfig_SingularFromEnvOrFlagFails pins that the
// one-or-the-other rule covers every source: a singular output value set by
// env var, flag, or embedded override next to a YAML outputs: list is
// rejected, not silently ignored. Declared defaults alone never trigger it.
func TestValidateExclusiveOutputConfig_SingularFromEnvOrFlagFails(t *testing.T) {
	yaml := "outputs:\n  - type: nop\n"

	defaults := viperFrom(t, yaml)
	defaults.SetDefault("output.type", "nop")
	defaults.SetDefault("output.tcp.workers", 1)
	require.NoError(t, ValidateExclusiveOutputConfig(defaults, nil), "defaults are not a singular config")

	override := viperFrom(t, yaml)
	override.SetDefault("output.type", "nop")
	override.Set("output.type", "tcp") // embedded EnvOverrides path
	require.ErrorContains(t, ValidateExclusiveOutputConfig(override, nil), "not both")

	t.Setenv("BLITZ_OUTPUT_TCP_HOST", "collector")
	env := viperFrom(t, yaml)
	env.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	env.SetDefault("output.tcp.host", "")
	require.NoError(t, env.BindEnv("output.tcp.host", "BLITZ_OUTPUT_TCP_HOST"))
	require.ErrorContains(t, ValidateExclusiveOutputConfig(env, nil), "not both")
}

// TestValidateExclusiveOutputConfig_CLIBoundDefaultsPass pins the CLI path:
// with every override bound to flags and env the way cmd/blitz binds them, an
// outputs: list with no singular output passes. Overrides that share one flag
// (the flattened OTLP TLS keys) must still register their declared defaults.
func TestValidateExclusiveOutputConfig_CLIBoundDefaultsPass(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	for _, o := range DefaultOverrides() {
		require.NoError(t, o.Bind(flags))
	}
	viper.SetConfigType("yaml")
	require.NoError(t, viper.ReadConfig(strings.NewReader("outputs:\n  - type: nop\n  - type: stdout\n")))
	require.NoError(t, ValidateExclusiveOutputConfig(viper.GetViper(), CLIExplicit(flags)))
}

// TestValidateListFormMatchesTypeCaseInsensitively pins that a list entry's
// type key is found whatever its case, as decoding finds it.
func TestValidateListFormMatchesTypeCaseInsensitively(t *testing.T) {
	require.NoError(t, ValidateExclusiveOutputConfig(viperFrom(t, "outputs:\n  - Type: nop\n"), nil))
}
