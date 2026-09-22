package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEffectiveOutputs_SingleOutput(t *testing.T) {
	cfg := &Config{Output: Output{Type: OutputTypeStdout}}

	outs := cfg.EffectiveOutputs()
	require.Len(t, outs, 1)
	assert.Equal(t, OutputTypeStdout, outs[0].Type)
}

func TestEffectiveOutputs_MultiOutput(t *testing.T) {
	cfg := &Config{
		Outputs: []Output{
			{Type: OutputTypeStdout},
			{Type: OutputTypeNop},
		},
	}

	outs := cfg.EffectiveOutputs()
	require.Len(t, outs, 2)
	assert.Equal(t, OutputTypeStdout, outs[0].Type)
	assert.Equal(t, OutputTypeNop, outs[1].Type)
}

func TestEffectiveOutputs_MultiOverridesSingle(t *testing.T) {
	cfg := &Config{
		Output:  Output{Type: OutputTypeStdout},
		Outputs: []Output{{Type: OutputTypeNop}, {Type: OutputTypeFile}},
	}

	outs := cfg.EffectiveOutputs()
	require.Len(t, outs, 2, "Outputs field should take precedence")
}

func TestEffectiveOutputs_ListEntriesInheritDefaults(t *testing.T) {
	// c.Output is the viper-defaulted template (workers filled). A list entry
	// that omits workers must inherit that default; user-set fields win.
	cfg := &Config{
		Output: Output{TCP: TCPOutputConfig{Workers: 1}},
		Outputs: []Output{
			{Type: OutputTypeTCP, TCP: TCPOutputConfig{Host: "h", Port: 5000}},
		},
	}

	outs := cfg.EffectiveOutputs()
	require.Len(t, outs, 1)
	assert.Equal(t, 1, outs[0].TCP.Workers, "unset workers should inherit the template default")
	assert.Equal(t, "h", outs[0].TCP.Host, "user-set host must be preserved")
	assert.Equal(t, 5000, outs[0].TCP.Port, "user-set port must be preserved")
}

func TestEffectiveOutputs_ListEntryUserValueOverridesDefault(t *testing.T) {
	cfg := &Config{
		Output: Output{TCP: TCPOutputConfig{Workers: 1}},
		Outputs: []Output{
			{Type: OutputTypeTCP, TCP: TCPOutputConfig{Host: "h", Port: 5000, Workers: 8}},
		},
	}

	outs := cfg.EffectiveOutputs()
	assert.Equal(t, 8, outs[0].TCP.Workers, "explicit workers must win over the template default")
}

func TestValidate_OutputsListValidatesEach(t *testing.T) {
	cfg := &Config{
		Logging: Logging{Type: LoggingTypeStdout},
		Outputs: []Output{
			{Type: OutputTypeNop},
			{Type: OutputType("bogus")},
		},
	}

	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "outputs[1]")
}
