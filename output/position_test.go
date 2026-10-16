package output

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/noop"
)

// TestWithOutputPositionLeavesCallerOptions pins that adding the position
// attribute never writes into the caller's option slice, even one with spare
// capacity.
func TestWithOutputPositionLeavesCallerOptions(t *testing.T) {
	opts := make([]metric.MeterOption, 1, 4)
	opts[0] = metric.WithInstrumentationVersion("v1")
	WithOutputPosition(noop.NewMeterProvider(), "outputs[1]").Meter("x", opts...)
	require.Nil(t, opts[:2][1], "the caller's backing array is untouched")
}

// TestWithOutputPositionNilUsesGlobal pins that a nil MeterProvider falls back
// to the global one rather than panicking on first use.
func TestWithOutputPositionNilUsesGlobal(t *testing.T) {
	require.NotNil(t, WithOutputPosition(nil, "outputs[0]").Meter("blitz"))
}
