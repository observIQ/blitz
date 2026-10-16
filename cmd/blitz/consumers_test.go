package main

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/observiq/blitz/embed"
	"github.com/observiq/blitz/output"
	flowout "github.com/observiq/blitz/output/flow"
	"github.com/observiq/blitz/output/nop"
	"github.com/observiq/blitz/output/stdout"
)

// TestFanOutTakesWholeBatches pins that a generator writing to a fan-out
// hands it whole batches: every consumer is the MultiOutput itself, not a
// per-record adapter, so a hostmetrics scrape is one queue slot per child.
func TestFanOutTakesWholeBatches(t *testing.T) {
	child, err := nop.New(zap.NewNop(), embed.NopTelemetry())
	require.NoError(t, err)
	m, err := output.NewMultiOutput(zap.NewNop(), embed.NopTelemetry(), output.FanOutChild{Type: "nop", Output: child})
	require.NoError(t, err)

	c := consumersFor(m, embed.NopTelemetry())
	require.Same(t, m, c.LogConsumer)
	require.Same(t, m, c.MetricConsumer)
	require.Same(t, m, c.TraceConsumer)
}

// TestSingleOutputConsumersFollowItsWriters pins that a single output gets a
// consumer for each signal it writes, and none for the others.
func TestSingleOutputConsumersFollowItsWriters(t *testing.T) {
	logOnly, err := nop.New(zap.NewNop(), embed.NopTelemetry())
	require.NoError(t, err)
	c := consumersFor(logOnly, embed.NopTelemetry())
	require.NotNil(t, c.LogConsumer)
	require.Nil(t, c.MetricConsumer)
	require.Nil(t, c.TraceConsumer)

	all, err := stdout.New(zap.NewNop())
	require.NoError(t, err)
	t.Cleanup(func() { _ = all.Stop(t.Context()) })
	c = consumersFor(all, embed.NopTelemetry())
	require.NotNil(t, c.LogConsumer)
	require.NotNil(t, c.MetricConsumer)
	require.NotNil(t, c.TraceConsumer)
}

// TestFlowOutputIsTheFlowConsumer pins that a generator writing to the flow
// output gets that output as its FlowConsumer, so a flow generator can run.
func TestFlowOutputIsTheFlowConsumer(t *testing.T) {
	o, err := flowout.New(zap.NewNop(), "127.0.0.1", "9", flowout.ProtocolNetFlowV5, "", "", embed.NopTelemetry())
	require.NoError(t, err)
	t.Cleanup(func() { _ = o.Stop(context.Background()) })
	require.Same(t, o, consumersFor(o, embed.NopTelemetry()).FlowConsumer)
}
