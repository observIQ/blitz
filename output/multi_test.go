package output

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/observiq/blitz/embed"
	"github.com/observiq/blitz/generator/count"
	"github.com/observiq/blitz/internal/counttest"
	"github.com/observiq/blitz/telemetry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/noop"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

// fakeOutput is a configurable test double. It records the calls it
// receives and optionally returns injected errors. The signal-writer
// methods are always present; whether the MultiOutput routes to them is
// governed by supported (SupportedTelemetry) plus the interface guards.
type fakeOutput struct {
	supported []telemetry.Type

	logs    []LogRecord
	metrics []MetricRecord
	traces  []TraceRecord
	stopped bool

	writeErr       error
	writeMetricErr error
	writeTraceErr  error
	stopErr        error
}

func (f *fakeOutput) WriteLog(_ context.Context, data LogRecord) error {
	f.logs = append(f.logs, data)
	return f.writeErr
}

func (f *fakeOutput) Stop(context.Context) error {
	f.stopped = true
	return f.stopErr
}

func (f *fakeOutput) SupportedTelemetry() []telemetry.Type { return f.supported }

func (f *fakeOutput) WriteMetric(_ context.Context, data MetricRecord) error {
	f.metrics = append(f.metrics, data)
	return f.writeMetricErr
}

func (f *fakeOutput) WriteTrace(_ context.Context, data TraceRecord) error {
	f.traces = append(f.traces, data)
	return f.writeTraceErr
}

// logsOnly implements LogWriter but not MetricWriter/TraceWriter,
// used to prove interface-guard routing.
type logsOnly struct{ inner *fakeOutput }

func (l logsOnly) WriteLog(ctx context.Context, data LogRecord) error {
	return l.inner.WriteLog(ctx, data)
}
func (l logsOnly) Stop(ctx context.Context) error       { return l.inner.Stop(ctx) }
func (l logsOnly) SupportedTelemetry() []telemetry.Type { return l.inner.SupportedTelemetry() }

// drain stops m, which waits for every queued record to reach its child, so
// the fakes can be read afterwards without a race.
func drain(t *testing.T, m *MultiOutput) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, m.Stop(ctx))
}

func TestMultiOutputWriteFansToAllChildren(t *testing.T) {
	a := &fakeOutput{supported: []telemetry.Type{telemetry.Logs}}
	b := &fakeOutput{supported: []telemetry.Type{telemetry.Logs}}
	m := newMulti(t, zap.NewNop(), a, b)

	require.NoError(t, m.WriteLog(context.Background(), LogRecord{Message: "hello"}))
	drain(t, m)
	require.Len(t, a.logs, 1)
	assert.Len(t, b.logs, 1)
	assert.Equal(t, "hello", a.logs[0].Message)
}

func TestMultiOutputChildWriteErrorDoesNotStopOthers(t *testing.T) {
	// A child's own write error happens on its forwarder, after the caller
	// has returned, so it is logged rather than returned.
	a := &fakeOutput{supported: []telemetry.Type{telemetry.Logs, telemetry.Metrics}, writeErr: errors.New("boom"), writeMetricErr: errors.New("metric boom")}
	b := &fakeOutput{supported: []telemetry.Type{telemetry.Logs, telemetry.Metrics}}
	m := newMulti(t, zap.NewNop(), a, b)

	v := int64(1)
	for range 3 {
		require.NoError(t, m.WriteLog(context.Background(), LogRecord{Message: "x"}))
		require.NoError(t, m.WriteMetric(context.Background(), MetricRecord{Name: "m", IntValue: &v}))
	}
	drain(t, m)
	assert.Len(t, a.logs, 3, "the failing child keeps receiving records after its first error")
	assert.Len(t, a.metrics, 3, "the failing child keeps receiving records after its first error")
	assert.Len(t, b.logs, 3)
	assert.Len(t, b.metrics, 3)
}

func TestMultiOutputWriteMetricRoutesBySupportAndInterface(t *testing.T) {
	// declares Metrics and implements MetricWriter -> receives.
	metricChild := &fakeOutput{supported: []telemetry.Type{telemetry.Logs, telemetry.Metrics}}
	// implements MetricWriter but does NOT declare Metrics -> skipped.
	undeclared := &fakeOutput{supported: []telemetry.Type{telemetry.Logs}}
	// declares Metrics but does not implement MetricWriter -> skipped.
	noWriter := logsOnly{inner: &fakeOutput{supported: []telemetry.Type{telemetry.Logs, telemetry.Metrics}}}

	m := newMulti(t, zap.NewNop(), metricChild, undeclared, noWriter)
	v := int64(1)
	require.NoError(t, m.WriteMetric(context.Background(), MetricRecord{Name: "m", Type: MetricTypeGauge, IntValue: &v}))
	drain(t, m)
	assert.Len(t, metricChild.metrics, 1)
	assert.Empty(t, undeclared.metrics)
	assert.Empty(t, noWriter.inner.metrics)
}

func TestMultiOutputWriteTraceRoutesBySupportAndInterface(t *testing.T) {
	traceChild := &fakeOutput{supported: []telemetry.Type{telemetry.Logs, telemetry.Traces}}
	undeclared := &fakeOutput{supported: []telemetry.Type{telemetry.Logs}}
	noWriter := logsOnly{inner: &fakeOutput{supported: []telemetry.Type{telemetry.Logs, telemetry.Traces}}}

	m := newMulti(t, zap.NewNop(), traceChild, undeclared, noWriter)
	require.NoError(t, m.WriteTrace(context.Background(), TraceRecord{Name: "s"}))
	drain(t, m)
	assert.Len(t, traceChild.traces, 1)
	assert.Empty(t, undeclared.traces)
	assert.Empty(t, noWriter.inner.traces)
}

func TestMultiOutputSupportedTelemetryIsUnion(t *testing.T) {
	a := &fakeOutput{supported: []telemetry.Type{telemetry.Logs, telemetry.Metrics}}
	b := &fakeOutput{supported: []telemetry.Type{telemetry.Logs, telemetry.Traces}}
	m := newMulti(t, zap.NewNop(), a, b)

	got := m.SupportedTelemetry()
	assert.ElementsMatch(t, []telemetry.Type{telemetry.Logs, telemetry.Metrics, telemetry.Traces}, got)
}

// declaresOnly declares signals but implements no writer interface, like an
// output still on the pre-reshape Write(LogRecord) method.
type declaresOnly struct{ supported []telemetry.Type }

func (d declaresOnly) Stop(context.Context) error           { return nil }
func (d declaresOnly) SupportedTelemetry() []telemetry.Type { return d.supported }

// TestMultiOutputSupportsOnlyRoutedSignals pins that the fan-out advertises a
// signal only when some child can write it, so the signal check rejects a
// fan-out that would drop every record, as it rejects the single output.
func TestMultiOutputSupportsOnlyRoutedSignals(t *testing.T) {
	all := []telemetry.Type{telemetry.Logs, telemetry.Metrics, telemetry.Traces}
	m := newMulti(t, zap.NewNop(), declaresOnly{supported: all}, logsOnly{inner: &fakeOutput{supported: all}})

	assert.Equal(t, []telemetry.Type{telemetry.Logs}, m.SupportedTelemetry())
}

func TestMultiOutputStopStopsAllAndAggregates(t *testing.T) {
	boom := errors.New("stop boom")
	a := &fakeOutput{supported: []telemetry.Type{telemetry.Logs}, stopErr: boom}
	b := &fakeOutput{supported: []telemetry.Type{telemetry.Logs}}
	m := newMulti(t, zap.NewNop(), a, b)

	err := m.Stop(context.Background())
	require.Error(t, err)
	assert.ErrorIs(t, err, boom)
	// Both stopped even though the first errored.
	assert.True(t, a.stopped)
	assert.True(t, b.stopped)
}

func TestMultiOutputImplementsWriterInterfaces(t *testing.T) {
	m := newMulti(t, zap.NewNop())
	var _ Output = m
	var _ LogWriter = m
	var _ MetricWriter = m
	var _ TraceWriter = m
}

// countingOutput counts its calls without allocating, so it can sit under
// testing.AllocsPerRun. It declares every signal.
type countingOutput struct {
	supportedCalls atomic.Int64
	writes         atomic.Int64
}

func (c *countingOutput) WriteLog(context.Context, LogRecord) error {
	c.writes.Add(1)
	return nil
}

func (c *countingOutput) WriteMetric(context.Context, MetricRecord) error {
	c.writes.Add(1)
	return nil
}

func (c *countingOutput) WriteTrace(context.Context, TraceRecord) error {
	c.writes.Add(1)
	return nil
}

func (c *countingOutput) Stop(context.Context) error { return nil }

func (c *countingOutput) SupportedTelemetry() []telemetry.Type {
	c.supportedCalls.Add(1)
	return []telemetry.Type{telemetry.Logs, telemetry.Metrics, telemetry.Traces}
}

func TestMultiOutputWritePathNeverCallsSupportedTelemetry(t *testing.T) {
	a, b := &countingOutput{}, &countingOutput{}
	m := newMulti(t, zap.NewNop(), a, b)
	t.Cleanup(func() { _ = m.Stop(context.Background()) })
	before := a.supportedCalls.Load() + b.supportedCalls.Load()

	ctx := context.Background()
	v := int64(1)
	require.NoError(t, m.WriteLog(ctx, LogRecord{Message: "x"}))
	require.NoError(t, m.WriteMetric(ctx, MetricRecord{Name: "m", IntValue: &v}))
	require.NoError(t, m.WriteTrace(ctx, TraceRecord{Name: "s"}))
	_ = m.SupportedTelemetry()

	assert.Equal(t, before, a.supportedCalls.Load()+b.supportedCalls.Load(),
		"SupportedTelemetry must be read once at construction, never on the write path")
}

func TestMultiOutputWriteLogDoesNotAllocate(t *testing.T) {
	a, b := &countingOutput{}, &countingOutput{}
	m := newMulti(t, zap.NewNop(), a, b)
	t.Cleanup(func() { _ = m.Stop(context.Background()) })
	ctx := context.Background()
	rec := LogRecord{Message: "x"}

	// 50 runs stays under the per-child queue capacity.
	allocs := testing.AllocsPerRun(50, func() { _ = m.WriteLog(ctx, rec) })
	assert.Zero(t, allocs, "WriteLog must not allocate per record")
}

// blockingOutput models a dead destination whose queue is full: every write
// blocks until its context is done.
type blockingOutput struct{ countingOutput }

func (b *blockingOutput) WriteLog(ctx context.Context, _ LogRecord) error {
	<-ctx.Done()
	return ctx.Err()
}

// ctxOutput models a healthy destination that, like the real outputs, fails a
// write whose context is already done.
type ctxOutput struct{ countingOutput }

func (c *ctxOutput) WriteLog(ctx context.Context, rec LogRecord) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return c.countingOutput.WriteLog(ctx, rec)
}

func TestMultiOutputDeadChildDoesNotStarveHealthyChild(t *testing.T) {
	dead, healthy := &blockingOutput{}, &ctxOutput{}
	m := newMulti(t, zap.NewNop(), dead, healthy)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		_ = m.Stop(ctx)
	})

	const n = 5
	for range n {
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		_ = m.WriteLog(ctx, LogRecord{Message: "x"})
		cancel()
	}

	require.Eventually(t, func() bool { return healthy.writes.Load() == n }, time.Second, 10*time.Millisecond,
		"healthy child must receive every record while a sibling is dead")
}

func TestMultiOutputPartialDropIsNotACallerError(t *testing.T) {
	core, logs := observer.New(zap.WarnLevel)
	dead, healthy := &blockingOutput{}, &countingOutput{}
	m := newMulti(t, zap.New(core), dead, healthy)

	// The dead child holds one record in its blocked write and queues
	// multiQueueSize more, so later writes overflow its queue. Waiting for the
	// healthy child after each write keeps its own queue from backing up.
	const n = multiQueueSize + 10
	for i := range n {
		require.NoError(t, m.WriteLog(context.Background(), LogRecord{Message: "x"}),
			"a drop for one child is not an error while another child accepted the record")
		require.Eventually(t, func() bool { return healthy.writes.Load() == int64(i+1) }, time.Second, time.Millisecond)
	}

	drops := logs.FilterMessage("fan-out output queue full, records dropped").All()
	require.NotEmpty(t, drops, "every drop is logged at Warn immediately")
	assert.Equal(t, zap.WarnLevel, drops[0].Level)
	assert.Equal(t, "outputs[0]", drops[0].ContextMap()["output"], "the log names the child that dropped")

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	stopErr := m.Stop(ctx)
	require.ErrorIs(t, stopErr, context.DeadlineExceeded, "Stop gives up draining a dead child when ctx ends")
	assert.Equal(t, int64(n), healthy.writes.Load())
}

func TestMultiOutputAllChildrenDroppedIsAnError(t *testing.T) {
	dead := &blockingOutput{}
	m := newMulti(t, zap.NewNop(), dead)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		_ = m.Stop(ctx)
	})

	var err error
	for range multiQueueSize + 10 {
		if e := m.WriteLog(context.Background(), LogRecord{Message: "x"}); e != nil {
			err = e
		}
	}
	require.ErrorIs(t, err, ErrQueueFull, "a record no eligible child accepted is an error")
	assert.Contains(t, err.Error(), "outputs[0]")
}

func TestMultiOutputWriteAfterStopFails(t *testing.T) {
	m := newMulti(t, zap.NewNop(), &countingOutput{})
	require.NoError(t, m.Stop(context.Background()))
	require.Error(t, m.WriteLog(context.Background(), LogRecord{Message: "x"}))
	require.NoError(t, m.Stop(context.Background()), "a second Stop is a no-op")
}

// spanOutput records the span context each write sees.
type spanOutput struct {
	countingOutput
	got trace.SpanContext
}

func (s *spanOutput) WriteLog(ctx context.Context, _ LogRecord) error {
	s.got = trace.SpanContextFromContext(ctx)
	return nil
}

func TestMultiOutputCarriesCallerSpanAcrossQueue(t *testing.T) {
	child := &spanOutput{}
	m := newMulti(t, zap.NewNop(), child)
	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    trace.TraceID{1},
		SpanID:     trace.SpanID{2},
		TraceFlags: trace.FlagsSampled,
	})

	require.NoError(t, m.WriteLog(trace.ContextWithSpanContext(context.Background(), sc), LogRecord{Message: "x"}))
	drain(t, m)
	assert.Equal(t, sc.TraceID(), child.got.TraceID())
	assert.Equal(t, sc.SpanID(), child.got.SpanID())
}

// newMulti builds a MultiOutput over outs, labelling each child "fake".
func newMulti(t *testing.T, logger *zap.Logger, outs ...Output) *MultiOutput {
	t.Helper()
	kids := make([]FanOutChild, len(outs))
	for i, o := range outs {
		kids[i] = FanOutChild{Type: "fake", Output: o}
	}
	m, err := NewMultiOutput(logger, embed.NopTelemetry(), kids...)
	require.NoError(t, err)
	return m
}

func TestMultiOutputDropsAreCountedPerOutput(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	tel := embed.NopTelemetry()
	tel.MeterProvider = sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	healthy := &countingOutput{}
	m, err := NewMultiOutput(zap.NewNop(), tel,
		FanOutChild{Type: "tcp", Output: &blockingOutput{}},
		FanOutChild{Type: "file", Output: healthy})
	require.NoError(t, err)

	const n = multiQueueSize + 10
	for i := range n {
		require.NoError(t, m.WriteLog(context.Background(), LogRecord{Message: "x"}))
		require.Eventually(t, func() bool { return healthy.writes.Load() == int64(i+1) }, time.Second, time.Millisecond)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_ = m.Stop(ctx)

	var rm metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(context.Background(), &rm))
	var got int64
	for _, sm := range rm.ScopeMetrics {
		for _, mm := range sm.Metrics {
			if mm.Name != "blitz.output.fanout_dropped" {
				continue
			}
			sum, ok := mm.Data.(metricdata.Sum[int64])
			require.True(t, ok)
			for _, dp := range sum.DataPoints {
				out, _ := dp.Attributes.Value("output")
				typ, _ := dp.Attributes.Value("output_type")
				sig, _ := dp.Attributes.Value("telemetry_type")
				assert.Equal(t, "outputs[0]", out.AsString(), "only the dead child drops")
				assert.Equal(t, "tcp", typ.AsString())
				assert.Equal(t, "logs", sig.AsString())
				got += dp.Value
			}
		}
	}
	// The dead child holds one record in its blocked write and queues
	// multiQueueSize more, so exactly the rest are dropped.
	assert.Equal(t, int64(n-multiQueueSize-1), got)
}

// chanOutput is shaped like the real outputs: WriteLog hands the record to
// an internal channel (nobody reads it here, as with a dead destination), and
// Stop closes that channel. A WriteLog that races Stop panics on the closed
// channel, so Stop must not run while a forwarder can still write.
type chanOutput struct {
	countingOutput
	ch        chan LogRecord
	stopErrAt atomic.Value // ctx.Err() seen on entry to Stop
}

func newChanOutput() *chanOutput { return &chanOutput{ch: make(chan LogRecord)} }

func (c *chanOutput) WriteLog(ctx context.Context, rec LogRecord) error {
	select {
	case c.ch <- rec:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (c *chanOutput) Stop(ctx context.Context) error {
	c.stopErrAt.Store(fmt.Sprint(ctx.Err()))
	close(c.ch)
	return nil
}

func TestMultiOutputStopWaitsForForwardersBeforeStoppingChildren(t *testing.T) {
	child := newChanOutput()
	m := newMulti(t, zap.NewNop(), child)
	for range multiQueueSize + 1 {
		_ = m.WriteLog(context.Background(), LogRecord{Message: "x"})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	require.ErrorIs(t, m.Stop(ctx), context.DeadlineExceeded, "the dead child could not drain in time")

	assert.Equal(t, "<nil>", child.stopErrAt.Load(), "the child's Stop must get a ctx with budget left for its own drain")

	done := make(chan struct{})
	go func() {
		m.wg.Wait()
		close(done)
	}()
	require.Eventually(t, func() bool {
		select {
		case <-done:
			return true
		default:
			return false
		}
	}, 100*time.Millisecond, time.Millisecond, "forwarders must have exited by the time Stop returns")
}

// TestMultiOutputChildWriteErrorsAreVisible pins that a child failing its own
// writes is logged at Warn and counted, since the caller has already moved on
// and would otherwise never see it.
func TestMultiOutputChildWriteErrorsAreVisible(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	tel := embed.NopTelemetry()
	tel.MeterProvider = sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	core, logs := observer.New(zap.WarnLevel)
	failing := &fakeOutput{supported: []telemetry.Type{telemetry.Logs}, writeErr: errors.New("connection refused")}
	m, err := NewMultiOutput(zap.New(core), tel, FanOutChild{Type: "tcp", Output: failing})
	require.NoError(t, err)

	const n = 3
	for range n {
		require.NoError(t, m.WriteLog(context.Background(), LogRecord{Message: "x"}))
	}
	drain(t, m)

	warns := logs.FilterMessage("fan-out output write failed").All()
	require.NotEmpty(t, warns, "a failing child's write error must be logged at Warn")
	assert.Equal(t, "outputs[0]", warns[0].ContextMap()["output"])

	var rm metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(context.Background(), &rm))
	var got int64
	for _, sm := range rm.ScopeMetrics {
		for _, mm := range sm.Metrics {
			if mm.Name != "blitz.output.fanout_write_errors" {
				continue
			}
			for _, dp := range mm.Data.(metricdata.Sum[int64]).DataPoints {
				out, _ := dp.Attributes.Value("output")
				assert.Equal(t, "outputs[0]", out.AsString())
				got += dp.Value
			}
		}
	}
	assert.Equal(t, int64(n), got)
}

// stuckOutput's WriteLog ignores ctx and never returns, breaking the Output
// contract; Stop must still return rather than hang shutdown.
type stuckOutput struct {
	countingOutput
	entered chan struct{}
	stopped atomic.Bool
}

func (s *stuckOutput) WriteLog(context.Context, LogRecord) error {
	close(s.entered)
	select {} // never returns
}

func (s *stuckOutput) Stop(context.Context) error {
	s.stopped.Store(true)
	return nil
}

func TestMultiOutputStopReturnsWhenChildIgnoresCancel(t *testing.T) {
	core, logs := observer.New(zap.ErrorLevel)
	stuck := &stuckOutput{entered: make(chan struct{})}
	healthy := &fakeOutput{supported: []telemetry.Type{telemetry.Logs}}
	m := newMulti(t, zap.New(core), stuck, healthy)
	require.NoError(t, m.WriteLog(context.Background(), LogRecord{Message: "x"}))
	<-stuck.entered

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- m.Stop(ctx) }()

	var err error
	select {
	case err = <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Stop hung on a child that ignores cancellation")
	}
	require.ErrorContains(t, err, "outputs[0]")
	assert.False(t, stuck.stopped.Load(), "a stuck child is still writing, so its Stop must not run")
	assert.True(t, healthy.stopped, "healthy children are still stopped")
	assert.NotEmpty(t, logs.FilterMessage("fan-out output did not stop writing; abandoning it").All())
}

// gatedOutput blocks every write until released, then counts it.
type gatedOutput struct {
	countingOutput
	release chan struct{}
}

func (g *gatedOutput) WriteMetric(ctx context.Context, rec MetricRecord) error {
	<-g.release
	return g.countingOutput.WriteMetric(ctx, rec)
}

// TestMultiOutputBatchTakesOneQueueSlot pins that a batch (one hostmetrics
// scrape) takes one queue slot per child, so a burst larger than the queue
// never drops points on a healthy child that is momentarily busy.
func TestMultiOutputBatchTakesOneQueueSlot(t *testing.T) {
	healthy := &gatedOutput{release: make(chan struct{})}
	m := newMulti(t, zap.NewNop(), healthy)
	mc, ok := any(m).(embed.MetricConsumer)
	require.True(t, ok, "MultiOutput must accept whole metric batches")

	const n = 5 * multiQueueSize
	batch := make([]embed.MetricPoint, n)
	require.NoError(t, mc.ConsumeMetrics(context.Background(), batch))
	close(healthy.release)
	require.Eventually(t, func() bool { return healthy.writes.Load() == n }, 2*time.Second, time.Millisecond,
		"every point of the batch reaches the healthy child")
	drain(t, m)
}

// TestMultiOutputStopWithoutDeadlineCancelsBlockedWrites pins that Stop with
// no deadline still bounds the drain: it cancels a child blocked in a write
// (one that honors cancel) and returns, instead of hanging forever.
func TestMultiOutputStopWithoutDeadlineCancelsBlockedWrites(t *testing.T) {
	defer func(d, a time.Duration) { stopDrainAfter, stopAbandonAfter = d, a }(stopDrainAfter, stopAbandonAfter)
	stopDrainAfter, stopAbandonAfter = 50*time.Millisecond, 50*time.Millisecond

	blocked := &blockingOutput{}
	m := newMulti(t, zap.NewNop(), blocked)
	require.NoError(t, m.WriteLog(context.Background(), LogRecord{Message: "x"}))

	done := make(chan error, 1)
	go func() { done <- m.Stop(context.Background()) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Stop without a deadline hung on a blocked child")
	}
}

// gatedRecorder holds every write until released, then records it, so a test
// can change the caller's batch while it is still queued.
type gatedRecorder struct {
	fakeOutput
	release chan struct{}
}

func (g *gatedRecorder) WriteLog(ctx context.Context, rec LogRecord) error {
	<-g.release
	return g.fakeOutput.WriteLog(ctx, rec)
}

func (g *gatedRecorder) WriteTrace(ctx context.Context, rec TraceRecord) error {
	<-g.release
	return g.fakeOutput.WriteTrace(ctx, rec)
}

// TestMultiOutputLogAndTraceBatchesFanOut pins the batch path the CLI wires
// every log and trace generator to: each child gets the whole batch in one
// queue slot, in order, and the batch is copied so the caller may reuse its
// slice.
func TestMultiOutputLogAndTraceBatchesFanOut(t *testing.T) {
	const n = 5 * multiQueueSize
	t.Run("logs", func(t *testing.T) {
		g := &gatedRecorder{fakeOutput: fakeOutput{supported: []telemetry.Type{telemetry.Logs}}, release: make(chan struct{})}
		m := newMulti(t, zap.NewNop(), g)
		batch := make([]embed.LogRecord, n)
		for i := range batch {
			batch[i].Message = fmt.Sprint(i)
		}
		require.NoError(t, m.ConsumeLogs(context.Background(), batch))
		batch[0].Message = "reused"
		close(g.release)
		drain(t, m)
		require.Len(t, g.logs, n, "a batch larger than the queue fits in one slot")
		assert.Equal(t, "0", g.logs[0].Message, "the caller's slice is copied")
		assert.Equal(t, fmt.Sprint(n-1), g.logs[n-1].Message, "records arrive in order")
	})
	t.Run("traces", func(t *testing.T) {
		g := &gatedRecorder{fakeOutput: fakeOutput{supported: []telemetry.Type{telemetry.Traces}}, release: make(chan struct{})}
		m := newMulti(t, zap.NewNop(), g)
		batch := make([]embed.Span, n)
		for i := range batch {
			batch[i].Name = fmt.Sprint(i)
		}
		require.NoError(t, m.ConsumeTraces(context.Background(), batch))
		batch[0].Name = "reused"
		close(g.release)
		drain(t, m)
		require.Len(t, g.traces, n, "a batch larger than the queue fits in one slot")
		assert.Equal(t, "0", g.traces[0].Name, "the caller's slice is copied")
		assert.Equal(t, fmt.Sprint(n-1), g.traces[n-1].Name, "records arrive in order")
	})
}

// TestMultiOutputBatchSpans pins that a fan-out keeps the per-batch span the
// single-output adapters create, and hands it to the children.
func TestMultiOutputBatchSpans(t *testing.T) {
	for _, on := range []bool{false, true} {
		rec := tracetest.NewSpanRecorder()
		tel := embed.NopTelemetry()
		tel.TracerProvider = sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(rec))
		tel.PerBatchSpans = on
		child := &spanOutput{}
		m, err := NewMultiOutput(zap.NewNop(), tel, FanOutChild{Type: "fake", Output: child})
		require.NoError(t, err)

		ctx := context.Background()
		require.NoError(t, m.ConsumeLogs(ctx, []embed.LogRecord{{Message: "x"}}))
		require.NoError(t, m.ConsumeMetrics(ctx, []embed.MetricPoint{{Name: "m"}}))
		require.NoError(t, m.ConsumeTraces(ctx, []embed.Span{{Name: "s"}}))
		drain(t, m)

		var names []string
		var logSpan trace.SpanContext
		for _, s := range rec.Ended() {
			names = append(names, s.Name())
			if s.Name() == "blitz.emit.logs" {
				logSpan = s.SpanContext()
			}
		}
		if !on {
			assert.Empty(t, names, "no span when PerBatchSpans is off")
			continue
		}
		assert.ElementsMatch(t, []string{"blitz.emit.logs", "blitz.emit.metrics", "blitz.emit.traces"}, names)
		assert.Equal(t, logSpan.SpanID(), child.got.SpanID(), "the child writes under the batch span")
	}
}

// TestMultiOutputShutdownLossIsNotAWriteError pins that batches still queued
// when Stop cancels writes are reported once as shutdown loss, not as one
// write error per batch.
func TestMultiOutputShutdownLossIsNotAWriteError(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	tel := embed.NopTelemetry()
	tel.MeterProvider = sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	core, logs := observer.New(zap.WarnLevel)
	m, err := NewMultiOutput(zap.New(core), tel, FanOutChild{Type: "tcp", Output: &blockingOutput{}})
	require.NoError(t, err)

	const n = 10
	for range n {
		require.NoError(t, m.WriteLog(context.Background(), LogRecord{Message: "x"}))
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	require.ErrorIs(t, m.Stop(ctx), context.DeadlineExceeded)

	assert.Empty(t, logs.FilterMessage("fan-out output write failed").All(), "cancelled writes are not write failures")
	lost := logs.FilterMessage("fan-out output abandoned queued records at shutdown").All()
	require.Len(t, lost, 1)
	assert.Equal(t, int64(n), lost[0].ContextMap()["batches"])

	var rm metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(context.Background(), &rm))
	for _, sm := range rm.ScopeMetrics {
		for _, mm := range sm.Metrics {
			assert.NotEqual(t, "blitz.output.fanout_write_errors", mm.Name, "shutdown loss is not counted as write errors")
		}
	}
}

// failingMeterProvider's meters fail to create instruments.
type failingMeterProvider struct{ noop.MeterProvider }

func (failingMeterProvider) Meter(string, ...metric.MeterOption) metric.Meter { return failingMeter{} }

type failingMeter struct{ noop.Meter }

func (failingMeter) Int64Gauge(string, ...metric.Int64GaugeOption) (metric.Int64Gauge, error) {
	return nil, errors.New("instrument refused")
}

// TestNewMultiOutputConstruction pins the constructor's edges: a nil logger
// gets a no-op logger, and a metrics failure is returned, not ignored.
func TestNewMultiOutputConstruction(t *testing.T) {
	child := &fakeOutput{supported: []telemetry.Type{telemetry.Logs}}
	m, err := NewMultiOutput(nil, embed.NopTelemetry(), FanOutChild{Type: "fake", Output: child})
	require.NoError(t, err)
	require.NoError(t, m.WriteLog(context.Background(), LogRecord{Message: "x"}))
	drain(t, m)
	require.Len(t, child.logs, 1)

	tel := embed.NopTelemetry()
	tel.MeterProvider = failingMeterProvider{}
	_, err = NewMultiOutput(zap.NewNop(), tel, FanOutChild{Type: "fake", Output: child})
	require.ErrorContains(t, err, "build fan-out metrics")
}

// TestMultiOutputEmptyBatchesWriteNothing pins that an empty batch enqueues
// nothing, rather than an item a child would write as one zero record.
func TestMultiOutputEmptyBatchesWriteNothing(t *testing.T) {
	child := &fakeOutput{supported: []telemetry.Type{telemetry.Logs, telemetry.Metrics, telemetry.Traces}}
	flows := &flowOutput{}
	m := newMulti(t, zap.NewNop(), child, flows)
	ctx := context.Background()
	require.NoError(t, m.ConsumeLogs(ctx, nil))
	require.NoError(t, m.ConsumeMetrics(ctx, nil))
	require.NoError(t, m.ConsumeTraces(ctx, nil))
	require.NoError(t, m.ConsumeFlows(ctx, nil))
	drain(t, m)
	assert.Empty(t, child.logs)
	assert.Empty(t, child.metrics)
	assert.Empty(t, child.traces)
	assert.Zero(t, flows.calls, "an empty flow batch reaches no child")
}

// stopCtxOutput records whether its Stop got a live context.
type stopCtxOutput struct {
	fakeOutput
	stopCtxErr atomic.Value
}

func (s *stopCtxOutput) Stop(ctx context.Context) error {
	s.stopCtxErr.Store(fmt.Sprint(ctx.Err()))
	return nil
}

// TestMultiOutputStuckChildLeavesHealthyStopBudget pins that a child ignoring
// cancellation does not use up the shutdown deadline: healthy children still
// get a live context for their own Stop.
func TestMultiOutputStuckChildLeavesHealthyStopBudget(t *testing.T) {
	stuck := &stuckOutput{entered: make(chan struct{})}
	healthy := &stopCtxOutput{fakeOutput: fakeOutput{supported: []telemetry.Type{telemetry.Logs}}}
	m := newMulti(t, zap.NewNop(), stuck, healthy)
	require.NoError(t, m.WriteLog(context.Background(), LogRecord{Message: "x"}))
	<-stuck.entered

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	require.ErrorContains(t, m.Stop(ctx), "outputs[0]")
	assert.Equal(t, "<nil>", healthy.stopCtxErr.Load(), "a healthy child's Stop keeps budget for its own flush")
}

// slowStopOutput's Stop holds until its context ends, as an output flushing
// to an unresponsive endpoint does.
type slowStopOutput struct{ fakeOutput }

func (s *slowStopOutput) Stop(ctx context.Context) error {
	<-ctx.Done()
	return ctx.Err()
}

// TestMultiOutputSlowStopLeavesOthersTheirBudget pins that one child's Stop
// running out the deadline does not leave the next child an expired context:
// children stop concurrently, each with the caller's context.
func TestMultiOutputSlowStopLeavesOthersTheirBudget(t *testing.T) {
	slow := &slowStopOutput{fakeOutput{supported: []telemetry.Type{telemetry.Logs}}}
	healthy := &stopCtxOutput{fakeOutput: fakeOutput{supported: []telemetry.Type{telemetry.Logs}}}
	m := newMulti(t, zap.NewNop(), slow, healthy)
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	require.ErrorIs(t, m.Stop(ctx), context.DeadlineExceeded)
	assert.Equal(t, "<nil>", healthy.stopCtxErr.Load(), "a slow child's Stop must not use up the next child's budget")
}

// TestMultiOutputStopsHealthyChildrenPastTheDeadline pins that a healthy
// child is still stopped (flushed and closed) when Stop's ctx deadline has
// already passed: an idle forwarder exits as soon as its queue closes, so it
// must not be mistaken for a stuck one and abandoned.
func TestMultiOutputStopsHealthyChildrenPastTheDeadline(t *testing.T) {
	for i := range 300 {
		child := &fakeOutput{supported: []telemetry.Type{telemetry.Logs}}
		m := newMulti(t, zap.NewNop(), child)
		ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
		_ = m.Stop(ctx)
		cancel()
		require.True(t, child.stopped, "run %d: a healthy child was abandoned", i)
	}
}

// cancelWaitOutput's writes block until their context is cancelled.
type cancelWaitOutput struct {
	countingOutput
	entered chan struct{}
}

func (c *cancelWaitOutput) WriteLog(ctx context.Context, _ LogRecord) error {
	close(c.entered)
	<-ctx.Done()
	return ctx.Err()
}

// TestMultiOutputDrainErrorKeepsTheCancelCause pins that when Stop's ctx is
// cancelled (not timed out) before the queues drain, the error says so.
func TestMultiOutputDrainErrorKeepsTheCancelCause(t *testing.T) {
	child := &cancelWaitOutput{entered: make(chan struct{})}
	m := newMulti(t, zap.NewNop(), child)
	require.NoError(t, m.WriteLog(context.Background(), LogRecord{Message: "x"}))
	<-child.entered

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := m.Stop(ctx)
	require.ErrorIs(t, err, context.Canceled)
	require.NotErrorIs(t, err, context.DeadlineExceeded)
}

// flowOutput is a flow-capable child: it implements embed.FlowConsumer and
// declares Flows.
type flowOutput struct {
	countingOutput
	mu    sync.Mutex
	recs  []embed.FlowRecord
	calls int
}

func (f *flowOutput) ConsumeFlows(_ context.Context, recs []embed.FlowRecord) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.recs = append(f.recs, recs...)
	f.calls++
	return nil
}

func (f *flowOutput) SupportedTelemetry() []telemetry.Type { return []telemetry.Type{telemetry.Flows} }

func (f *flowOutput) got() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.recs)
}

// TestMultiOutputFansFlowsToFlowChildren pins that a fan-out with a flow
// output routes flow batches to it (and only to flow-capable children), so a
// flow generator works with an outputs: list.
func TestMultiOutputFansFlowsToFlowChildren(t *testing.T) {
	flows := &flowOutput{}
	logs := &fakeOutput{supported: []telemetry.Type{telemetry.Logs}}
	m := newMulti(t, zap.NewNop(), flows, logs)

	var fc embed.FlowConsumer = m
	batch := []embed.FlowRecord{{Packets: 1}, {Packets: 2}}
	require.NoError(t, fc.ConsumeFlows(context.Background(), batch))
	batch[0].Packets = 99 // the caller reusing its slice must not reach the child
	require.Eventually(t, func() bool { return flows.got() == 2 }, time.Second, time.Millisecond)
	drain(t, m)
	assert.Equal(t, uint64(1), flows.recs[0].Packets)
	assert.Contains(t, m.SupportedTelemetry(), telemetry.Flows)
	assert.Empty(t, logs.logs)
}

// errFlowOutput is a flow child whose every write fails.
type errFlowOutput struct{ flowOutput }

func (e *errFlowOutput) ConsumeFlows(context.Context, []embed.FlowRecord) error {
	return errors.New("connection refused")
}

// blockingFlowOutput is a flow child that blocks each write until ctx ends.
type blockingFlowOutput struct{ flowOutput }

func (b *blockingFlowOutput) ConsumeFlows(ctx context.Context, _ []embed.FlowRecord) error {
	<-ctx.Done()
	return ctx.Err()
}

// TestMultiOutputFlowWriteErrorsAreVisible pins that a flow child failing its
// writes is logged at Warn and counted per record, like the other signals.
func TestMultiOutputFlowWriteErrorsAreVisible(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	tel := embed.NopTelemetry()
	tel.MeterProvider = sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	core, logs := observer.New(zap.WarnLevel)
	m, err := NewMultiOutput(zap.New(core), tel, FanOutChild{Type: "flow", Output: &errFlowOutput{}})
	require.NoError(t, err)

	require.NoError(t, m.ConsumeFlows(context.Background(), []embed.FlowRecord{{Packets: 1}, {Packets: 2}}))
	drain(t, m)

	warns := logs.FilterMessage("fan-out output write failed").All()
	require.Len(t, warns, 1)
	assert.Equal(t, "flows", warns[0].ContextMap()["telemetry_type"])
	assert.Equal(t, int64(2), counterTotal(t, reader, "blitz.output.fanout_write_errors"), "one error per record of the failed batch")
}

// TestMultiOutputFlowShutdownLossIsNotAWriteError is the flow mirror of
// TestMultiOutputShutdownLossIsNotAWriteError.
func TestMultiOutputFlowShutdownLossIsNotAWriteError(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	tel := embed.NopTelemetry()
	tel.MeterProvider = sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	core, logs := observer.New(zap.WarnLevel)
	m, err := NewMultiOutput(zap.New(core), tel, FanOutChild{Type: "flow", Output: &blockingFlowOutput{}})
	require.NoError(t, err)

	const n = 10
	for range n {
		require.NoError(t, m.ConsumeFlows(context.Background(), []embed.FlowRecord{{Packets: 1}}))
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	require.ErrorIs(t, m.Stop(ctx), context.DeadlineExceeded)

	assert.Empty(t, logs.FilterMessage("fan-out output write failed").All(), "cancelled writes are not write failures")
	lost := logs.FilterMessage("fan-out output abandoned queued records at shutdown").All()
	require.Len(t, lost, 1)
	assert.Equal(t, int64(n), lost[0].ContextMap()["batches"])
	assert.Zero(t, counterTotal(t, reader, "blitz.output.fanout_write_errors"))
}

// counterTotal sums every data point of the named int64 counter.
func counterTotal(t *testing.T, reader *sdkmetric.ManualReader, name string) int64 {
	t.Helper()
	var rm metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(context.Background(), &rm))
	var total int64
	for _, sm := range rm.ScopeMetrics {
		for _, mm := range sm.Metrics {
			if mm.Name != name {
				continue
			}
			for _, dp := range mm.Data.(metricdata.Sum[int64]).DataPoints {
				total += dp.Value
			}
		}
	}
	return total
}

// TestMultiOutputSupportsFlowsOnlyWhenRouted pins that the fan-out advertises
// Flows only when a child implements embed.FlowConsumer.
func TestMultiOutputSupportsFlowsOnlyWhenRouted(t *testing.T) {
	declared := declaresOnly{supported: []telemetry.Type{telemetry.Flows}}
	assert.Empty(t, newMulti(t, zap.NewNop(), declared).SupportedTelemetry())
	assert.Equal(t, []telemetry.Type{telemetry.Flows}, newMulti(t, zap.NewNop(), declared, &flowOutput{}).SupportedTelemetry())
}

// holdingOutput models a queued child output: it takes its own hold on the
// record's permit and settles it when the test says the send finished.
type holdingOutput struct {
	countingOutput
	mu   sync.Mutex
	sent []func(error)
}

func (h *holdingOutput) WriteLog(ctx context.Context, _ LogRecord) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.sent = append(h.sent, count.Hold(ctx))
	return nil
}

func (h *holdingOutput) finish(err error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, s := range h.sent {
		s(err)
	}
}

// TestMultiOutputPermitWaitsForEveryChildSend pins that a fanned-out record
// counts only once every child has sent it, which needs the permit to reach
// the children across the queue hop.
func TestMultiOutputPermitWaitsForEveryChildSend(t *testing.T) {
	a, b := &holdingOutput{}, &holdingOutput{}
	m := newMulti(t, zap.NewNop(), a, b)

	ctx, p := counttest.Held(t)
	require.NoError(t, m.WriteLog(ctx, LogRecord{Message: "x"}))
	p.Commit()
	drain(t, m)
	counttest.StaysPending(t, p, "neither child has sent the record yet")

	a.finish(nil)
	counttest.StaysPending(t, p)
	b.finish(nil)
	require.True(t, counttest.Settled(p))
	require.True(t, p.Delivered())
}

// TestMultiOutputChildWriteErrorReleasesPermit pins that one child's failed
// write fails the record, so a replacement goes out.
func TestMultiOutputChildWriteErrorReleasesPermit(t *testing.T) {
	ok := &fakeOutput{supported: []telemetry.Type{telemetry.Logs}}
	bad := &fakeOutput{supported: []telemetry.Type{telemetry.Logs}, writeErr: errors.New("refused")}
	m := newMulti(t, zap.NewNop(), ok, bad)

	ctx, p := counttest.Held(t)
	require.NoError(t, m.WriteLog(ctx, LogRecord{Message: "x"}))
	p.Commit()
	drain(t, m)
	require.True(t, counttest.Settled(p))
	require.False(t, p.Delivered())
}

// TestMultiOutputDroppedForOneChildReleasesPermit pins that a record one
// child's full queue dropped is not counted, even though another child took
// it.
func TestMultiOutputDroppedForOneChildReleasesPermit(t *testing.T) {
	dead, healthy := &blockingOutput{}, &ctxOutput{}
	m := newMulti(t, zap.NewNop(), dead, healthy)
	for range multiQueueSize + 1 {
		// Wait for room in the healthy child's queue, so a write never finds both full.
		require.Eventually(t, func() bool { return len(m.logs[1].q) < multiQueueSize }, 5*time.Second, time.Millisecond)
		require.NoError(t, m.WriteLog(context.Background(), LogRecord{Message: "fill"}))
	}
	require.Eventually(t, func() bool { return len(m.logs[0].q) == multiQueueSize && len(m.logs[1].q) == 0 },
		5*time.Second, time.Millisecond, "the dead child's queue is full and the healthy one's drained")

	ctx, p := counttest.Held(t)
	require.NoError(t, m.WriteLog(ctx, LogRecord{Message: "x"}), "the healthy child accepted it")
	p.Commit()
	stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_ = m.Stop(stopCtx)
	require.True(t, counttest.Settled(p))
	require.False(t, p.Delivered())
}

// TestMultiOutputAbandonReleasesQueuedPermits pins that records a stuck
// child abandons at shutdown are not counted.
func TestMultiOutputAbandonReleasesQueuedPermits(t *testing.T) {
	child := newChanOutput()
	m := newMulti(t, zap.NewNop(), child)
	var permits []count.Permit
	for range 3 {
		ctx, p := counttest.Held(t)
		require.NoError(t, m.WriteLog(ctx, LogRecord{Message: "x"}))
		p.Commit()
		permits = append(permits, p)
	}
	stopCtx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	_ = m.Stop(stopCtx)
	for _, p := range permits {
		require.True(t, counttest.Settled(p))
		require.False(t, p.Delivered())
	}
}

// TestMultiOutputNoEligibleChildFails pins that in a count run a record no
// child can take is a failed write, so it is not counted as delivered. An
// uncounted record is skipped, as before.
func TestMultiOutputNoEligibleChildFails(t *testing.T) {
	m := newMulti(t, zap.NewNop(), &fakeOutput{supported: []telemetry.Type{telemetry.Logs}})
	require.NoError(t, m.WriteTrace(context.Background(), TraceRecord{Name: "s"}))

	ctx, p := counttest.Held(t)
	require.ErrorIs(t, m.WriteTrace(ctx, TraceRecord{Name: "s"}), ErrNoOutput)
	// The generator releases a refused record; with no hold left behind, the
	// permit settles at once.
	p.Release()
	require.True(t, counttest.Settled(p), "the fan-out left a hold on the refused record")
}

// TestMultiOutputFlowSendSettlesPermit pins that a counted flow batch counts
// once its flow child takes it, and is released when the child's write fails.
func TestMultiOutputFlowSendSettlesPermit(t *testing.T) {
	for name, tc := range map[string]struct {
		child     Output
		delivered bool
	}{
		"sent":   {&flowOutput{}, true},
		"failed": {&errFlowOutput{}, false},
	} {
		t.Run(name, func(t *testing.T) {
			m := newMulti(t, zap.NewNop(), tc.child)
			ctx, p := counttest.Held(t)
			require.NoError(t, m.ConsumeFlows(ctx, []embed.FlowRecord{{Packets: 1}}))
			p.Commit()
			require.True(t, counttest.Settled(p), "the flow send settles the record")
			require.Equal(t, tc.delivered, p.Delivered())
			drain(t, m)
		})
	}
}
