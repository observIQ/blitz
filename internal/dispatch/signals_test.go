package dispatch

import (
	"slices"
	"strings"
	"testing"

	"github.com/observiq/blitz/embed"
	"github.com/observiq/blitz/internal/config"
	"github.com/observiq/blitz/telemetry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
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
		{config.GeneratorTypeFlow, []telemetry.Type{telemetry.Flows}},
		{config.GeneratorTypeNop, nil},
		{"bogus", nil},
	}
	for _, c := range cases {
		assert.ElementsMatch(t, c.want, SignalsFor(c.typ), "type %q", c.typ)
	}
}

// TestSignalsForMapsEveryGeneratorType keeps SignalsFor in step with
// Generator.Validate: every type Validate accepts except nop emits a signal,
// so a new generator type cannot be silently treated as emitting nothing.
func TestSignalsForMapsEveryGeneratorType(t *testing.T) {
	err := (&config.Generator{Type: "no-such-type"}).Validate()
	require.Error(t, err)
	_, accepted, ok := strings.Cut(err.Error(), "must be one of: ")
	require.True(t, ok, "unexpected Validate error: %v", err)
	for _, typ := range strings.Split(accepted, ", ") {
		if typ == string(config.GeneratorTypeNop) {
			continue
		}
		assert.NotEmpty(t, SignalsFor(config.GeneratorType(typ)), "type %q has no signal", typ)
	}
}

// TestSignalsForMatchesForEmbedRequirements ties SignalsFor to the consumers
// ForEmbed requires: for every embed-eligible type, ForEmbed demands exactly
// the consumers of the signals SignalsFor names. winevt is not embed-eligible.
func TestSignalsForMatchesForEmbedRequirements(t *testing.T) {
	err := (&config.Generator{Type: "no-such-type"}).Validate()
	_, accepted, _ := strings.Cut(err.Error(), "must be one of: ")
	consumer := map[telemetry.Type]string{
		telemetry.Logs: "LogConsumer", telemetry.Metrics: "MetricConsumer", telemetry.Traces: "TraceConsumer",
		telemetry.Flows: "FlowConsumer",
	}
	for _, typ := range strings.Split(accepted, ", ") {
		gt := config.GeneratorType(typ)
		if gt == config.GeneratorTypeNop || gt == config.GeneratorTypeWinevt {
			continue
		}
		emits := SignalsFor(gt)
		for sig, name := range consumer {
			all := EmbedConsumers{LogConsumer: noopConsumer{}, MetricConsumer: noopMetricConsumer{}, TraceConsumer: noopTraceConsumer{}, FlowConsumer: noopFlowConsumer{}}
			switch sig {
			case telemetry.Logs:
				all.LogConsumer = nil
			case telemetry.Metrics:
				all.MetricConsumer = nil
			case telemetry.Traces:
				all.TraceConsumer = nil
			case telemetry.Flows:
				all.FlowConsumer = nil
			}
			_, err := ForEmbed(zap.NewNop(), config.Generator{Type: gt}, all, nil, nil, embed.NopTelemetry())
			requires := err != nil && strings.Contains(err.Error(), "requires EmbedConsumers."+name)
			assert.Equal(t, slices.Contains(emits, sig), requires, "type %q without %s: %v", typ, name, err)
		}
	}
}
