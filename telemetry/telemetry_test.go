package telemetry

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTypeValid(t *testing.T) {
	assert.True(t, Logs.Valid())
	assert.True(t, Metrics.Valid())
	assert.False(t, Type("unknown").Valid())
	assert.False(t, Type("").Valid())
}

func TestSupports(t *testing.T) {
	logsOnly := []Type{Logs}
	both := []Type{Logs, Metrics}

	assert.True(t, Supports(logsOnly, Logs))
	assert.False(t, Supports(logsOnly, Metrics))
	assert.True(t, Supports(both, Logs))
	assert.True(t, Supports(both, Metrics))
	assert.False(t, Supports(nil, Logs))
}

func TestCompatible(t *testing.T) {
	t.Run("both support logs", func(t *testing.T) {
		common, err := Compatible([]Type{Logs}, []Type{Logs})
		require.NoError(t, err)
		assert.Equal(t, []Type{Logs}, common)
	})

	t.Run("generator logs+metrics, output logs", func(t *testing.T) {
		common, err := Compatible([]Type{Logs, Metrics}, []Type{Logs})
		require.NoError(t, err)
		assert.Equal(t, []Type{Logs}, common)
	})

	t.Run("both support logs+metrics", func(t *testing.T) {
		common, err := Compatible([]Type{Logs, Metrics}, []Type{Logs, Metrics})
		require.NoError(t, err)
		assert.Equal(t, []Type{Logs, Metrics}, common)
	})

	t.Run("no overlap", func(t *testing.T) {
		_, err := Compatible([]Type{Metrics}, []Type{Logs})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no compatible telemetry types")
	})
}
