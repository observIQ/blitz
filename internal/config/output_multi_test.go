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

// Loading rejects a config with both forms, so this only pins what
// EffectiveOutputs returns when handed a struct with both set.
func TestEffectiveOutputs_ReturnsListWhenSet(t *testing.T) {
	cfg := &Config{
		Output:  Output{Type: OutputTypeStdout},
		Outputs: []Output{{Type: OutputTypeNop}, {Type: OutputTypeFile}},
	}

	outs := cfg.EffectiveOutputs()
	require.Len(t, outs, 2, "EffectiveOutputs returns the outputs list when it is set")
	assert.Equal(t, OutputTypeNop, outs[0].Type)
	assert.Equal(t, OutputTypeFile, outs[1].Type)
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
