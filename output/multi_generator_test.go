package output_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/observiq/blitz/embed"
	"github.com/observiq/blitz/generator/hostmetrics"
	jsongen "github.com/observiq/blitz/generator/json"
	"github.com/observiq/blitz/output"
	"github.com/observiq/blitz/telemetry"
)

// deadOutput models a destination that has stopped draining: every write
// blocks until its context is done.
type deadOutput struct{}

func (deadOutput) WriteLog(ctx context.Context, _ output.LogRecord) error {
	<-ctx.Done()
	return ctx.Err()
}

func (deadOutput) WriteMetric(ctx context.Context, _ output.MetricRecord) error {
	<-ctx.Done()
	return ctx.Err()
}

func (deadOutput) Stop(context.Context) error { return nil }

func (deadOutput) SupportedTelemetry() []telemetry.Type {
	return []telemetry.Type{telemetry.Logs, telemetry.Metrics}
}

// liveOutput counts every record it receives.
type liveOutput struct{ writes atomic.Int64 }

func (l *liveOutput) WriteLog(context.Context, output.LogRecord) error {
	l.writes.Add(1)
	return nil
}

func (l *liveOutput) WriteMetric(context.Context, output.MetricRecord) error {
	l.writes.Add(1)
	return nil
}

func (*liveOutput) Stop(context.Context) error { return nil }

func (*liveOutput) SupportedTelemetry() []telemetry.Type {
	return []telemetry.Type{telemetry.Logs, telemetry.Metrics}
}

// countingLogs counts the records the generator hands to the fan-out and the
// write errors it gets back.
type countingLogs struct {
	next   embed.LogConsumer
	sent   atomic.Int64
	failed atomic.Int64
}

func (c *countingLogs) ConsumeLogs(ctx context.Context, recs []embed.LogRecord) error {
	c.sent.Add(int64(len(recs)))
	err := c.next.ConsumeLogs(ctx, recs)
	if err != nil {
		c.failed.Add(1)
	}
	return err
}

type countingMetrics struct {
	next embed.MetricConsumer
	sent atomic.Int64
}

func (c *countingMetrics) ConsumeMetrics(ctx context.Context, pts []embed.MetricPoint) error {
	c.sent.Add(int64(len(pts)))
	return c.next.ConsumeMetrics(ctx, pts)
}

func stopFanOut(t *testing.T, m *output.MultiOutput) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_ = m.Stop(ctx) // the dead child cannot drain; only the healthy count matters
}

// A dead sibling must not slow the json generator: once the dead child's
// queue is full, writes keep succeeding, so the generator never sees an error
// to back off on, and the healthy child gets every record.
func TestFanOutJSONGeneratorKeepsHealthyRateWithDeadSibling(t *testing.T) {
	healthy := &liveOutput{}
	m := newMulti(t, deadOutput{}, healthy)
	counted := &countingLogs{next: output.WriterAsLogConsumer(m, embed.NopTelemetry())}

	gen, err := jsongen.New(zap.NewNop(), 1, time.Millisecond, jsongen.LogTypeDefault, counted, embed.NopTelemetry())
	require.NoError(t, err)
	require.NoError(t, gen.Start(context.Background()))

	// Well past the dead child's queue capacity (100). The bound is the
	// generator's rate, not the assertion, so it is generous for a loaded
	// runner; what matters is that no write failed.
	const want = 600
	require.Eventually(t, func() bool { return healthy.writes.Load() >= want }, 30*time.Second, 10*time.Millisecond)
	require.Zero(t, counted.failed.Load(), "a dead sibling must never fail the generator's writes")

	require.NoError(t, gen.Stop(context.Background()))
	stopFanOut(t, m)
	assert.Equal(t, counted.sent.Load(), healthy.writes.Load(), "healthy child receives every generated record")
}

// hostmetrics emits multi-point batches; a drop for the dead child on one
// point must not abort the rest of the batch for the healthy child.
func TestFanOutHostMetricsBatchesReachHealthyChildWithDeadSibling(t *testing.T) {
	healthy := &liveOutput{}
	m := newMulti(t, deadOutput{}, healthy)
	counted := &countingMetrics{next: m} // the CLI hands a fan-out whole batches

	gen, err := hostmetrics.New(hostmetrics.Config{
		Logger:   zap.NewNop(),
		Workers:  1,
		Rate:     time.Millisecond,
		OS:       "linux",
		Consumer: counted,
		Seed:     1,
	})
	require.NoError(t, err)
	require.NoError(t, gen.Start(context.Background()))
	require.Eventually(t, func() bool { return counted.sent.Load() >= 1000 }, 5*time.Second, 10*time.Millisecond)
	require.NoError(t, gen.Stop(context.Background()))

	stopFanOut(t, m)
	assert.Equal(t, counted.sent.Load(), healthy.writes.Load(), "healthy child receives every point of every batch")
}

func newMulti(t *testing.T, outs ...output.Output) *output.MultiOutput {
	t.Helper()
	kids := make([]output.FanOutChild, len(outs))
	for i, o := range outs {
		kids[i] = output.FanOutChild{Type: "fake", Output: o}
	}
	m, err := output.NewMultiOutput(zap.NewNop(), embed.NopTelemetry(), kids...)
	require.NoError(t, err)
	return m
}
