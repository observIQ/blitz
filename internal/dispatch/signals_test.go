package dispatch

import (
	"testing"

	"github.com/observiq/blitz/internal/config"
	"github.com/observiq/blitz/telemetry"
	"github.com/stretchr/testify/assert"
)

func TestSignalsFor(t *testing.T) {
	cases := []struct {
		typ  config.GeneratorType
		want []telemetry.Type
	}{
		{config.GeneratorTypeJSON, []telemetry.Type{telemetry.Logs}},
		{config.GeneratorTypeWel, []telemetry.Type{telemetry.Logs}},
		{config.GeneratorTypeFIX, []telemetry.Type{telemetry.Logs}},
		{config.GeneratorTypeWinevt, []telemetry.Type{telemetry.Logs}},
		{config.GeneratorTypeHostMetrics, []telemetry.Type{telemetry.Metrics}},
		{config.GeneratorTypeTraces, []telemetry.Type{telemetry.Traces}},
		{config.GeneratorTypeNop, nil},
	}
	for _, c := range cases {
		assert.ElementsMatch(t, c.want, SignalsFor(c.typ), "type %q", c.typ)
	}
}
