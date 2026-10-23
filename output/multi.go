package output

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"time"

	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/observiq/blitz/embed"
	"github.com/observiq/blitz/generator/count"
	"github.com/observiq/blitz/telemetry"
)

// multiQueueSize is each child's forwarding-queue capacity.
const multiQueueSize = 100

// ErrQueueFull is returned (wrapped with each child's outputs[i] position)
// when every eligible child's forwarding queue is full, so no child accepted
// the record.
var ErrQueueFull = errors.New("queue full, record dropped")

// ErrNoOutput is returned when no child takes a counted record's signal.
var ErrNoOutput = errors.New("no output takes this signal")

// Drop logs are sampled per child: the first drop in each window logs at
// once, then one in every dropLogThereafter until the window ends.
const (
	dropLogWindow     = time.Second
	dropLogThereafter = 100
)

// Without a ctx deadline, Stop bounds its phases itself: stopDrainAfter for
// the queues to drain before it cancels in-flight child writes, then
// stopAbandonAfter for those writes to return. Vars so tests can shorten them.
var (
	stopDrainAfter   = 5 * time.Second
	stopAbandonAfter = 5 * time.Second
	// stopGrace is how long a forwarder that has not exited by the end of
	// the abandon phase still gets: an idle one may not yet have seen its
	// queue close, notably when ctx's deadline had already passed.
	stopGrace = 250 * time.Millisecond
)

// errMultiStopped is returned by writes after Stop.
var errMultiStopped = errors.New("multi output stopped")

// MultiOutput fans one generated stream out to several child outputs,
// implementing Output, LogWriter, MetricWriter, TraceWriter, the
// ConsumeLogs/ConsumeMetrics/ConsumeTraces batch consumers, and
// embed.FlowConsumer so callers treat it like a single output. Each child
// still emits its own output_type-tagged metrics; the fan-out itself only
// records what no child can see: drops and write errors on its forwarding
// queues.
//
// Each child has its own bounded queue drained by one forwarding goroutine, so
// a slow or dead child never paces the others: a write enqueues without
// blocking, and a full queue drops the record for that child only. A drop is
// logged at Warn right away (sampled under a flood) and counted in
// blitz.output.fanout_dropped, but it is not a caller error while another
// child accepted the record: the write fails with ErrQueueFull only when every
// eligible child dropped it. A child's own write errors happen on its
// forwarding goroutine, after the caller has moved on, so they are logged at
// Warn (same sampling) and counted in blitz.output.fanout_write_errors.
type MultiOutput struct {
	logger    *zap.Logger
	tel       embed.TelemetrySettings
	counters  *Metrics
	children  []*multiChild
	logs      []*multiChild
	metrics   []*multiChild
	traces    []*multiChild
	flows     []*multiChild
	supported []telemetry.Type

	// ctx is handed to child writes; Stop cancels it to unblock them.
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// mu guards stopped and the queue close in Stop against concurrent sends.
	mu      sync.RWMutex
	stopped bool
}

type multiChild struct {
	name    string // outputs[i]
	typ     string // output type, for metrics
	out     Output
	log     LogWriter
	metric  MetricWriter
	trace   TraceWriter
	flow    embed.FlowConsumer
	q       chan multiItem
	errFull error
	// warn logs this child's drops and write errors, sampled per child.
	warn *zap.Logger
	// done closes when the child's forwarder exits.
	done chan struct{}
}

// FanOutChild is one output a MultiOutput fans out to. Type is the output
// type (tcp, file, ...), used to label the child's drop metric.
type FanOutChild struct {
	Type   string
	Output Output
}

// multiItem is one queued batch: a single Write* call is a batch of one, and
// a Consume* call (a generator's whole batch, e.g. one hostmetrics scrape)
// takes one queue slot however large it is, so a burst never overflows a
// healthy child's queue. sc carries the caller's span so a child's send span
// keeps its parent across the queue hop.
type multiItem struct {
	sig telemetry.Type
	sc  trace.SpanContext
	// pctx carries the caller's count permit across the queue hop, nil for an
	// uncounted record, and sent settles this child's hold on it with the
	// write result.
	pctx context.Context
	sent func(error)
	// A single Write* keeps its record inline (no per-record allocation);
	// a Consume* batch uses the slice for its signal.
	log     LogRecord
	metric  MetricRecord
	trace   TraceRecord
	logs    []LogRecord
	metrics []MetricRecord
	traces  []TraceRecord
	flows   []embed.FlowRecord
}

// size is the number of records in the item: its batch length, or one.
func (it multiItem) size() int {
	if n := len(it.logs) + len(it.metrics) + len(it.traces) + len(it.flows); n > 0 {
		return n
	}
	return 1
}

// batched reports whether the item carries a Consume* batch.
func (it multiItem) batched() bool {
	return it.logs != nil || it.metrics != nil || it.traces != nil || it.flows != nil
}

// NewMultiOutput wraps the given outputs and starts one forwarding goroutine
// per child. Each child's SupportedTelemetry is read once here to route
// signals, relying on it being fixed once the output is configured.
func NewMultiOutput(logger *zap.Logger, tel embed.TelemetrySettings, children ...FanOutChild) (*MultiOutput, error) {
	if logger == nil {
		logger = zap.NewNop()
	}
	counters, err := NewMetrics(tel.MeterProvider)
	if err != nil {
		return nil, fmt.Errorf("build fan-out metrics: %w", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	m := &MultiOutput{logger: logger, tel: tel, counters: counters, ctx: ctx, cancel: cancel}
	for i, child := range children {
		o := child.Output
		name := fmt.Sprintf("outputs[%d]", i)
		c := &multiChild{
			name:    name,
			typ:     child.Type,
			out:     o,
			q:       make(chan multiItem, multiQueueSize),
			errFull: fmt.Errorf("%s: %w", name, ErrQueueFull),
			done:    make(chan struct{}),
			// One sampler per child, so a flood on one child never hides
			// the first drop on another.
			warn: logger.WithOptions(zap.WrapCore(func(core zapcore.Core) zapcore.Core {
				return zapcore.NewSamplerWithOptions(core, dropLogWindow, 1, dropLogThereafter)
			})).With(zap.String("output", name), zap.String("output_type", child.Type)),
		}
		sup := o.SupportedTelemetry()
		if w, ok := o.(LogWriter); ok && slices.Contains(sup, telemetry.Logs) {
			c.log = w
			m.logs = append(m.logs, c)
			m.support(telemetry.Logs)
		}
		if w, ok := o.(MetricWriter); ok && slices.Contains(sup, telemetry.Metrics) {
			c.metric = w
			m.metrics = append(m.metrics, c)
			m.support(telemetry.Metrics)
		}
		if w, ok := o.(TraceWriter); ok && slices.Contains(sup, telemetry.Traces) {
			c.trace = w
			m.traces = append(m.traces, c)
			m.support(telemetry.Traces)
		}
		if w, ok := o.(embed.FlowConsumer); ok && slices.Contains(sup, telemetry.Flows) {
			c.flow = w
			m.flows = append(m.flows, c)
			m.support(telemetry.Flows)
		}
		m.children = append(m.children, c)
		m.wg.Add(1)
		go m.forward(c) // #nosec G118 -- exits when Stop closes the queue
	}
	return m, nil
}

// support adds t to the advertised signals once. A signal is advertised only
// when some child can write it, so the signal check rejects a fan-out that
// would drop every record of it.
func (m *MultiOutput) support(t telemetry.Type) {
	if !slices.Contains(m.supported, t) {
		m.supported = append(m.supported, t)
	}
}

// forward drains one child's queue into the child until the queue is closed.
func (m *MultiOutput) forward(c *multiChild) {
	defer m.wg.Done()
	defer close(c.done)
	for it := range c.q {
		ctx := m.ctx
		if it.pctx != nil {
			ctx = count.Inherit(ctx, it.pctx)
		}
		if it.sc.IsValid() {
			ctx = trace.ContextWithSpanContext(ctx, it.sc)
		}
		if it.sig == telemetry.Flows {
			// A flow consumer takes the whole batch in one call.
			if err := c.flow.ConsumeFlows(ctx, it.flows); err != nil {
				if m.ctx.Err() != nil {
					m.abandon(c, it)
					return
				}
				c.warn.Warn("fan-out output write failed", zap.String("telemetry_type", string(it.sig)), zap.Error(err))
				m.counters.BlitzOutputFanoutWriteErrorsCounter.Add(context.Background(), int64(len(it.flows)), c.typ, string(it.sig), c.name)
				it.sent(err)
				continue
			}
			it.sent(nil)
			continue
		}
		var failed error
		for i := 0; i < it.size(); i++ {
			var err error
			switch {
			case it.sig == telemetry.Logs && it.batched():
				err = c.log.WriteLog(ctx, it.logs[i])
			case it.sig == telemetry.Logs:
				err = c.log.WriteLog(ctx, it.log)
			case it.sig == telemetry.Metrics && it.batched():
				err = c.metric.WriteMetric(ctx, it.metrics[i])
			case it.sig == telemetry.Metrics:
				err = c.metric.WriteMetric(ctx, it.metric)
			case it.sig == telemetry.Traces && it.batched():
				err = c.trace.WriteTrace(ctx, it.traces[i])
			case it.sig == telemetry.Traces:
				err = c.trace.WriteTrace(ctx, it.trace)
			}
			if err != nil && m.ctx.Err() != nil {
				m.abandon(c, it)
				return
			}
			if err != nil {
				c.warn.Warn("fan-out output write failed", zap.String("telemetry_type", string(it.sig)), zap.Error(err))
				m.counters.BlitzOutputFanoutWriteErrorsCounter.Add(context.Background(), 1, c.typ, string(it.sig), c.name)
				failed = err
			}
		}
		it.sent(failed)
	}
}

// abandon reports a child's in-flight batch and the rest of its queue as
// shutdown loss once Stop has cancelled writes: logged once, not counted as
// write errors, and settled as unsent. Stop has closed the queue by then.
func (m *MultiOutput) abandon(c *multiChild, inFlight multiItem) {
	c.warn.Warn("fan-out output abandoned queued records at shutdown", zap.Int("batches", 1+len(c.q)))
	inFlight.sent(errMultiStopped)
	for it := range c.q {
		it.sent(errMultiStopped)
	}
}

// enqueue offers it to each child without blocking. A child with a full
// queue drops it, which is logged and counted; the write fails only when
// every eligible child dropped it.
func (m *MultiOutput) enqueue(ctx context.Context, children []*multiChild, it multiItem) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.stopped {
		return errMultiStopped
	}
	if len(children) == 0 {
		// Only a counted record must not pass as delivered; others are skipped.
		if count.Counted(ctx) {
			return fmt.Errorf("%w: %s", ErrNoOutput, it.sig)
		}
		return nil
	}
	if count.Counted(ctx) {
		it.pctx = ctx
	}
	accepted := false
	for _, c := range children {
		it.sent = count.Hold(ctx)
		select {
		case c.q <- it:
			accepted = true
		default:
			m.dropped(c, it)
			it.sent(c.errFull)
		}
	}
	if accepted {
		return nil
	}
	errs := make([]error, len(children))
	for i, c := range children {
		errs[i] = c.errFull
	}
	return errors.Join(errs...)
}

// dropped logs and counts a batch dropped for c, one count per record.
func (m *MultiOutput) dropped(c *multiChild, it multiItem) {
	c.warn.Warn("fan-out output queue full, records dropped", zap.String("telemetry_type", string(it.sig)), zap.Int("records", it.size()))
	m.counters.BlitzOutputFanoutDroppedCounter.Add(context.Background(), int64(it.size()), c.typ, string(it.sig), c.name)
}

// WriteLog fans a log record to children that implement LogWriter and
// declare Logs; others are skipped.
func (m *MultiOutput) WriteLog(ctx context.Context, data LogRecord) error {
	return m.enqueue(ctx, m.logs, multiItem{sig: telemetry.Logs, sc: trace.SpanContextFromContext(ctx), log: data})
}

// batchSpan starts the per-batch span the consumer adapters create when
// PerBatchSpans is on, so a fan-out keeps that telemetry.
func (m *MultiOutput) batchSpan(ctx context.Context, name string) (context.Context, func()) {
	if !m.tel.PerBatchSpans {
		return ctx, func() {}
	}
	ctx, span := m.tel.Tracer(adapterTracerScope).Start(ctx, name)
	return ctx, func() { span.End() }
}

// ConsumeLogs fans a whole log batch out as one queue slot per child. The
// batch is copied so the caller can reuse its slice once this returns.
func (m *MultiOutput) ConsumeLogs(ctx context.Context, recs []embed.LogRecord) error {
	ctx, end := m.batchSpan(ctx, "blitz.emit.logs")
	defer end()
	if len(recs) == 0 {
		return nil
	}
	return m.enqueue(ctx, m.logs, multiItem{sig: telemetry.Logs, sc: trace.SpanContextFromContext(ctx), logs: slices.Clone(recs)})
}

// WriteMetric fans a metric record to children that implement MetricWriter
// and declare Metrics; others are skipped.
func (m *MultiOutput) WriteMetric(ctx context.Context, data MetricRecord) error {
	return m.enqueue(ctx, m.metrics, multiItem{sig: telemetry.Metrics, sc: trace.SpanContextFromContext(ctx), metric: data})
}

// ConsumeMetrics fans a whole metric batch out as one queue slot per child.
// The batch is copied so the caller can reuse its slice.
func (m *MultiOutput) ConsumeMetrics(ctx context.Context, points []embed.MetricPoint) error {
	ctx, end := m.batchSpan(ctx, "blitz.emit.metrics")
	defer end()
	if len(points) == 0 {
		return nil
	}
	return m.enqueue(ctx, m.metrics, multiItem{sig: telemetry.Metrics, sc: trace.SpanContextFromContext(ctx), metrics: slices.Clone(points)})
}

// WriteTrace fans a trace record to children that implement TraceWriter and
// declare Traces; others are skipped.
func (m *MultiOutput) WriteTrace(ctx context.Context, data TraceRecord) error {
	return m.enqueue(ctx, m.traces, multiItem{sig: telemetry.Traces, sc: trace.SpanContextFromContext(ctx), trace: data})
}

// ConsumeTraces fans a whole span batch out as one queue slot per child. The
// batch is copied so the caller can reuse its slice.
func (m *MultiOutput) ConsumeTraces(ctx context.Context, spans []embed.Span) error {
	ctx, end := m.batchSpan(ctx, "blitz.emit.traces")
	defer end()
	if len(spans) == 0 {
		return nil
	}
	return m.enqueue(ctx, m.traces, multiItem{sig: telemetry.Traces, sc: trace.SpanContextFromContext(ctx), traces: slices.Clone(spans)})
}

// ConsumeFlows fans a flow batch to children that implement
// embed.FlowConsumer and declare Flows; others are skipped. The batch is
// copied so the caller can reuse its slice once this returns.
func (m *MultiOutput) ConsumeFlows(ctx context.Context, recs []embed.FlowRecord) error {
	if len(recs) == 0 {
		return nil
	}
	return m.enqueue(ctx, m.flows, multiItem{sig: telemetry.Flows, sc: trace.SpanContextFromContext(ctx), flows: slices.Clone(recs)})
}

// Stop closes every queue and lets the forwarders drain them, using at most
// half of ctx's remaining time so the children keep budget for their own
// drain. If that runs out, it cancels in-flight child writes. It then waits
// for every forwarder to exit, so no write can race a child's Stop, and only
// then stops each child with ctx, aggregating errors; one failure does not
// skip the rest. A child whose write ignores cancellation (breaking the Output
// contract) is abandoned at three quarters of ctx's remaining time, so the
// healthy children's Stop keeps the last quarter. Children whose forwarders
// have exited are stopped first; the rest get stopGrace more to exit (an idle
// forwarder may not have seen its queue close when ctx had already run out).
// A child still writing then is abandoned: logged at Error, named in the
// returned error, and its Stop skipped.
// Without a ctx deadline both phases are still bounded (stopDrainAfter, then
// stopAbandonAfter), so Stop always returns.
func (m *MultiOutput) Stop(ctx context.Context) error {
	m.mu.Lock()
	if m.stopped {
		m.mu.Unlock()
		return nil
	}
	m.stopped = true
	for _, c := range m.children {
		close(c.q)
	}
	m.mu.Unlock()

	drained := make(chan struct{})
	go func() {
		m.wg.Wait()
		close(drained)
	}()

	// With a deadline the drain gets half the remaining time and cancelled
	// writes the next quarter, leaving the last quarter for the children's own
	// Stop even when one child ignores cancellation; without one, both phases
	// are bounded here.
	drainFor, abandonFor := stopDrainAfter, stopAbandonAfter
	if dl, ok := ctx.Deadline(); ok {
		left := time.Until(dl)
		drainFor, abandonFor = left/2, left/4
	}
	var errs []error
	drainCtx, cancelDrain := context.WithTimeout(ctx, drainFor)
	select {
	case <-drained:
	case <-drainCtx.Done():
		cause := ctx.Err() // cancelled or past its deadline
		if cause == nil {
			cause = context.DeadlineExceeded // the drain's share ran out
		}
		errs = append(errs, fmt.Errorf("drain fan-out queues: %w", cause))
	}
	cancelDrain()
	m.cancel()

	// A child write that ignores cancellation would hang shutdown, so wait
	// only for the abandon phase, then abandon the stuck child: its Stop is
	// skipped, since its forwarder may still be writing and Stop would race it.
	abandon, cancelAbandon := context.WithTimeout(ctx, abandonFor)
	defer cancelAbandon()
	select {
	case <-drained:
	case <-abandon.Done():
	}

	// Stop the children concurrently, each with ctx, so one slow Stop never
	// eats another's budget. A child whose forwarder has exited stops at once;
	// the rest get until stopGrace to exit before they are abandoned.
	grace, cancelGrace := context.WithTimeout(context.Background(), stopGrace)
	defer cancelGrace()
	childErrs := make([]error, len(m.children))
	var wg sync.WaitGroup
	for i, c := range m.children {
		wg.Go(func() {
			select {
			case <-c.done:
			case <-grace.Done():
			}
			select {
			case <-c.done: // preferred when both are ready
				childErrs[i] = c.out.Stop(ctx)
			default:
				m.logger.Error("fan-out output did not stop writing; abandoning it", zap.String("output", c.name), zap.String("output_type", c.typ))
				childErrs[i] = fmt.Errorf("%s: write did not return after cancel; output abandoned", c.name)
			}
		})
	}
	wg.Wait()
	errs = append(errs, childErrs...)
	return errors.Join(errs...)
}

// SupportedTelemetry returns the signals some child can write, computed once
// at construction. It returns a copy so callers cannot mutate the routing
// state.
func (m *MultiOutput) SupportedTelemetry() []telemetry.Type {
	return slices.Clone(m.supported)
}
