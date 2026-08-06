package runtime_test

import (
	"context"
	"testing"

	"github.com/observiq/blitz/internal/runtime"
	"github.com/stretchr/testify/require"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

type spanTestModule struct{ n string }

func (m spanTestModule) Name() string                { return m.n }
func (m spanTestModule) Start(context.Context) error { return nil }
func (m spanTestModule) Stop(context.Context) error  { return nil }

// TestRuntime_emitsSessionAndGeneratorSpans confirms the runtime emits a single
// root session span plus one lifecycle span per module, tagged with the
// generator name, through the provided TracerProvider.
func TestRuntime_emitsSessionAndGeneratorSpans(t *testing.T) {
	exp := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exp))

	rt, err := runtime.New(nil, []runtime.Module{
		spanTestModule{n: "json"},
		spanTestModule{n: "apache"},
	}, tp, nil)
	require.NoError(t, err)
	require.NoError(t, rt.Start(context.Background()))
	require.NoError(t, rt.Stop(context.Background()))

	byName := map[string]int{}
	genNames := map[string]int{}
	for _, s := range exp.GetSpans() {
		byName[s.Name]++
		for _, a := range s.Attributes {
			if string(a.Key) == "blitz.generator.name" {
				genNames[a.Value.AsString()]++
			}
		}
	}

	require.Equal(t, 1, byName["blitz.session"], "exactly one session span")
	require.Equal(t, 2, byName["blitz.generator.run"], "one lifecycle span per module")
	require.Equal(t, 1, genNames["json"])
	require.Equal(t, 1, genNames["apache"])
}
