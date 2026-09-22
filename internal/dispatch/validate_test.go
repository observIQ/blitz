package dispatch

import (
	"testing"

	"github.com/observiq/blitz/internal/config"
	"github.com/observiq/blitz/telemetry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateSignalCompat_ValidMixedPairing(t *testing.T) {
	// A: logs-only output + metrics-only output, log gen + metric gen.
	// Every generator and output has a compatible counterpart.
	err := ValidateSignalCompat(
		[]config.GeneratorType{config.GeneratorTypeJSON, config.GeneratorTypeHostMetrics},
		[]OutputSignals{
			{Name: "stdout", Signals: []telemetry.Type{telemetry.Logs}},
			{Name: "prometheus", Signals: []telemetry.Type{telemetry.Metrics}},
		},
	)
	assert.NoError(t, err)
}

func TestValidateSignalCompat_OrphanOutputFails(t *testing.T) {
	// B: same two outputs, but only a metric generator. Metrics still have a
	// sink, yet the logs-only output is orphaned, so the config fails.
	err := ValidateSignalCompat(
		[]config.GeneratorType{config.GeneratorTypeHostMetrics},
		[]OutputSignals{
			{Name: "stdout", Signals: []telemetry.Type{telemetry.Logs}},
			{Name: "prometheus", Signals: []telemetry.Type{telemetry.Metrics}},
		},
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "stdout")
}

func TestValidateSignalCompat_OrphanGeneratorAndOutputAggregated(t *testing.T) {
	// Metric generator with only a logs-only output: metrics have no sink
	// (orphan generator) AND the output receives nothing (orphan output).
	// Both surface in one error.
	err := ValidateSignalCompat(
		[]config.GeneratorType{config.GeneratorTypeHostMetrics},
		[]OutputSignals{
			{Name: "stdout", Signals: []telemetry.Type{telemetry.Logs}},
		},
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "metrics")
	assert.Contains(t, err.Error(), "stdout")
}

func TestValidateSignalCompat_AllCompatible(t *testing.T) {
	err := ValidateSignalCompat(
		[]config.GeneratorType{config.GeneratorTypeJSON},
		[]OutputSignals{{Name: "stdout", Signals: []telemetry.Type{telemetry.Logs}}},
	)
	assert.NoError(t, err)
}

func TestValidateSignalCompat_DuplicateOrphansDeduped(t *testing.T) {
	// Two metric generators, one logs-only output: the metrics-orphan message
	// should appear once, not per generator.
	err := ValidateSignalCompat(
		[]config.GeneratorType{config.GeneratorTypeHostMetrics, config.GeneratorTypeHostMetrics},
		[]OutputSignals{{Name: "stdout", Signals: []telemetry.Type{telemetry.Logs}}},
	)
	require.Error(t, err)
	// "metrics" appears once in the aggregated message.
	assert.Equal(t, 1, countSubstr(err.Error(), "no output accepts metrics"))
}

func countSubstr(s, sub string) int {
	n := 0
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			n++
		}
	}
	return n
}
