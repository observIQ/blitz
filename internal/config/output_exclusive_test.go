package config

import (
	"strings"
	"testing"

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
	require.Error(t, ValidateExclusiveOutputConfig(v))
}

func TestValidateExclusiveOutputConfig_OnlyOutputsOK(t *testing.T) {
	v := viperFrom(t, "outputs:\n  - type: nop\n")
	require.NoError(t, ValidateExclusiveOutputConfig(v))
}

func TestValidateExclusiveOutputConfig_OnlyOutputOK(t *testing.T) {
	v := viperFrom(t, "output:\n  type: stdout\n")
	require.NoError(t, ValidateExclusiveOutputConfig(v))
}

func TestValidateExclusiveOutputConfig_NeitherOK(t *testing.T) {
	v := viperFrom(t, "logging:\n  type: stdout\n")
	require.NoError(t, ValidateExclusiveOutputConfig(v))
}
