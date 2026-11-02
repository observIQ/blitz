package otlpgrpc

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/observiq/blitz/embed"
	"github.com/observiq/blitz/generator/count"
	"github.com/observiq/blitz/internal/counttest"
	"github.com/observiq/blitz/output"
	"github.com/stretchr/testify/require"
	collectorlogs "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	collectormetrics "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	collectortrace "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	logspb "go.opentelemetry.io/proto/otlp/logs/v1"
	metricspb "go.opentelemetry.io/proto/otlp/metrics/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
	"go.uber.org/zap/zaptest/observer"
	"google.golang.org/grpc"
)

// held returns a hold on a fresh permit, already committed by the generator.
func held(t *testing.T) (func(error), count.Permit) {
	t.Helper()
	ctx, p := counttest.Held(t)
	sent := count.Hold(ctx)
	p.Commit()
	return sent, p
}

func requireSettled(t *testing.T, p count.Permit, delivered bool) {
	t.Helper()
	require.True(t, counttest.Settled(p))
	require.Equal(t, delivered, p.Delivered())
}

// TestOTLPGrpc_ExportSettlesBatches pins that a batch's records count when
// the export succeeds and are released when it fails, for every signal.
func TestOTLPGrpc_ExportSettlesBatches(t *testing.T) {
	o := testOTLP(t)
	boom := errors.New("unavailable")
	for _, fail := range []bool{false, true} {
		var lErr, mErr, tErr error
		if fail {
			lErr, mErr, tErr = boom, boom, boom
		}
		ls, lp := held(t)
		lb := newLogBatch(10, 0)
		lb.add(&logspb.LogRecord{}, ls)
		_ = o.sendBatch(mockLogsClient{err: lErr}, lb)
		requireSettled(t, lp, !fail)

		ms, mp := held(t)
		mb := newMetricBatch(10, 0)
		mb.add(&metricspb.Metric{}, ms)
		_ = o.sendMetricBatch(mockMetricsClient{err: mErr}, mb)
		requireSettled(t, mp, !fail)

		ts, tp := held(t)
		tb := newTraceBatch(10, 0)
		tb.add(&tracepb.Span{}, ts)
		_ = o.sendTraceBatch(mockTraceClient{err: tErr}, tb)
		requireSettled(t, tp, !fail)
	}
}

// TestOTLPGrpc_BatchFailReleasesUnsent pins that records still batched when a
// worker exits are released.
func TestOTLPGrpc_BatchFailReleasesUnsent(t *testing.T) {
	ls, lp := held(t)
	ms, mp := held(t)
	ts, tp := held(t)
	lb, mb, tb := newLogBatch(10, 0), newMetricBatch(10, 0), newTraceBatch(10, 0)
	lb.add(&logspb.LogRecord{}, ls)
	mb.add(&metricspb.Metric{}, ms)
	tb.add(&tracepb.Span{}, ts)

	lb.fail(errStopped)
	mb.fail(errStopped)
	tb.fail(errStopped)
	for _, p := range []count.Permit{lp, mp, tp} {
		requireSettled(t, p, false)
	}
}

// TestOTLPGrpc_StopReleasesQueued pins that records still queued when the
// output stops against a destination it cannot reach are released.
func TestOTLPGrpc_StopReleasesQueued(t *testing.T) {
	o, err := New(zap.NewNop(), WithHost("127.0.0.1"), WithPort("1"), WithInsecure(true))
	require.NoError(t, err)

	var permits []count.Permit
	for range 3 {
		ctx, p := counttest.Held(t)
		require.NoError(t, o.WriteLog(ctx, output.LogRecord{Message: "x"}))
		p.Commit()
		permits = append(permits, p)
	}
	ctx, p := counttest.Held(t)
	require.NoError(t, o.WriteMetric(ctx, output.MetricRecord{Name: "m"}))
	p.Commit()
	permits = append(permits, p)

	require.NoError(t, o.Stop(context.Background()))
	for _, p := range permits {
		requireSettled(t, p, false)
	}
}

// TestOTLPGrpc_WorkerExitReleasesOtherBatches pins that when a failed export
// ends a worker, the records it still held in its other batches are released
// rather than lost.
func TestOTLPGrpc_WorkerExitReleasesOtherBatches(t *testing.T) {
	o, err := New(zap.NewNop(), WithHost("127.0.0.1"), WithPort("1"), WithInsecure(true),
		WithMaxExportBatchSize(2), WithBatchTimeout(time.Hour), WithRequestTimeout(time.Second))
	require.NoError(t, err)
	defer func() { _ = o.Stop(context.Background()) }()

	ctx, metric := counttest.Held(t)
	require.NoError(t, o.WriteMetric(ctx, output.MetricRecord{Name: "m"}))
	metric.Commit()
	require.Eventually(t, func() bool { return len(o.metricChan) == 0 }, 5*time.Second, time.Millisecond)
	for range 2 {
		require.NoError(t, o.WriteLog(context.Background(), output.LogRecord{Message: "x"}))
	}

	requireSettled(t, metric, false)
}

// TestOTLPGrpc_WorkerExitReleasesBatchedLogs pins the same for a log waiting
// in its partial batch when a metric export ends the worker.
func TestOTLPGrpc_WorkerExitReleasesBatchedLogs(t *testing.T) {
	o, err := New(zap.NewNop(), WithHost("127.0.0.1"), WithPort("1"), WithInsecure(true),
		WithMaxExportBatchSize(2), WithBatchTimeout(time.Hour), WithRequestTimeout(time.Second))
	require.NoError(t, err)
	defer func() { _ = o.Stop(context.Background()) }()

	ctx, log := counttest.Held(t)
	require.NoError(t, o.WriteLog(ctx, output.LogRecord{Message: "x"}))
	log.Commit()
	require.Eventually(t, func() bool { return len(o.dataChan) == 0 }, 5*time.Second, time.Millisecond)
	for range 2 {
		require.NoError(t, o.WriteMetric(context.Background(), output.MetricRecord{Name: "m"}))
	}

	requireSettled(t, log, false)
}

type okLogsServer struct {
	collectorlogs.UnimplementedLogsServiceServer
}

func (okLogsServer) Export(context.Context, *collectorlogs.ExportLogsServiceRequest) (*collectorlogs.ExportLogsServiceResponse, error) {
	return &collectorlogs.ExportLogsServiceResponse{}, nil
}

type okMetricsServer struct {
	collectormetrics.UnimplementedMetricsServiceServer
}

func (okMetricsServer) Export(context.Context, *collectormetrics.ExportMetricsServiceRequest) (*collectormetrics.ExportMetricsServiceResponse, error) {
	return &collectormetrics.ExportMetricsServiceResponse{}, nil
}

type okTraceServer struct {
	collectortrace.UnimplementedTraceServiceServer
}

func (okTraceServer) Export(context.Context, *collectortrace.ExportTraceServiceRequest) (*collectortrace.ExportTraceServiceResponse, error) {
	return &collectortrace.ExportTraceServiceResponse{}, nil
}

type failingLogsServer struct {
	collectorlogs.UnimplementedLogsServiceServer
}

func (failingLogsServer) Export(context.Context, *collectorlogs.ExportLogsServiceRequest) (*collectorlogs.ExportLogsServiceResponse, error) {
	return nil, errors.New("unavailable")
}

type failingMetricsServer struct {
	collectormetrics.UnimplementedMetricsServiceServer
}

func (failingMetricsServer) Export(context.Context, *collectormetrics.ExportMetricsServiceRequest) (*collectormetrics.ExportMetricsServiceResponse, error) {
	return nil, errors.New("unavailable")
}

type failingTraceServer struct {
	collectortrace.UnimplementedTraceServiceServer
}

func (failingTraceServer) Export(context.Context, *collectortrace.ExportTraceServiceRequest) (*collectortrace.ExportTraceServiceResponse, error) {
	return nil, errors.New("unavailable")
}

// okServer starts a collector that accepts every signal and returns its port.
func okServer(t *testing.T) string {
	t.Helper()
	return startServer(t, okLogsServer{}, okMetricsServer{}, okTraceServer{})
}

// startServer starts a collector with the given services and returns its port.
func startServer(t *testing.T, logs collectorlogs.LogsServiceServer, metrics collectormetrics.MetricsServiceServer, traces collectortrace.TraceServiceServer) string {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	srv := grpc.NewServer()
	collectorlogs.RegisterLogsServiceServer(srv, logs)
	collectormetrics.RegisterMetricsServiceServer(srv, metrics)
	collectortrace.RegisterTraceServiceServer(srv, traces)
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)
	_, port, err := net.SplitHostPort(lis.Addr().String())
	require.NoError(t, err)
	return port
}

// TestOTLPGrpc_StopFlushesPartialBatches pins that Stop exports the partial
// batches of every signal, whichever closed channel the worker sees first.
func TestOTLPGrpc_StopFlushesPartialBatches(t *testing.T) {
	port := okServer(t)
	for range 20 {
		o, err := New(zaptest.NewLogger(t), WithHost("127.0.0.1"), WithPort(port), WithInsecure(true),
			WithWorkers(1), WithBatchTimeout(time.Hour), WithMaxExportBatchSize(100))
		require.NoError(t, err)

		var permits []count.Permit
		for _, write := range []func(context.Context) error{
			func(ctx context.Context) error { return o.WriteLog(ctx, output.LogRecord{Message: "x"}) },
			func(ctx context.Context) error { return o.WriteMetric(ctx, output.MetricRecord{Name: "m"}) },
			func(ctx context.Context) error { return o.WriteTrace(ctx, output.TraceRecord{Name: "s"}) },
		} {
			ctx, p := counttest.Held(t)
			require.NoError(t, write(ctx))
			p.Commit()
			permits = append(permits, p)
		}
		require.Eventually(t, func() bool {
			return len(o.dataChan)+len(o.metricChan)+len(o.traceChan) == 0
		}, 5*time.Second, time.Millisecond, "the worker batched every record")

		require.NoError(t, o.Stop(context.Background()))
		for _, p := range permits {
			requireSettled(t, p, true)
		}
	}
}

// TestOTLPGrpc_UnqueuedWriteReleasesPermit pins that a record of any signal
// that never reaches its queue is released.
func TestOTLPGrpc_UnqueuedWriteReleasesPermit(t *testing.T) {
	unread := func(outputCtx context.Context) *OTLPGrpc {
		return &OTLPGrpc{
			dataChan:   make(chan queued[*logspb.LogRecord]),
			metricChan: make(chan queued[*metricspb.Metric]),
			traceChan:  make(chan queued[*tracepb.Span]),
			ctx:        outputCtx,
			logger:     zap.NewNop(),
		}
	}
	t.Run("logs", func(t *testing.T) {
		counttest.ReleasesUnqueued(t, func(writeCtx, outputCtx context.Context) error {
			return unread(outputCtx).WriteLog(writeCtx, output.LogRecord{Message: "x"})
		})
	})
	t.Run("metrics", func(t *testing.T) {
		counttest.ReleasesUnqueued(t, func(writeCtx, outputCtx context.Context) error {
			return unread(outputCtx).WriteMetric(writeCtx, output.MetricRecord{Name: "m"})
		})
	})
	t.Run("traces", func(t *testing.T) {
		counttest.ReleasesUnqueued(t, func(writeCtx, outputCtx context.Context) error {
			return unread(outputCtx).WriteTrace(writeCtx, output.TraceRecord{Name: "s"})
		})
	})
}

// TestOTLPGrpc_FlowsSettleOnTheirResult pins that a counted flow holds its
// permit until its queued log record is sent, and is released when it never
// reaches the queue.
func TestOTLPGrpc_FlowsSettleOnTheirResult(t *testing.T) {
	o := testOTLP(t)
	o.dataChan = make(chan queued[*logspb.LogRecord], 1)
	ctx, p := counttest.Held(t)
	require.NoError(t, o.ConsumeFlows(ctx, []embed.FlowRecord{{Packets: 1}}))
	p.Commit()
	select {
	case <-p.Settled():
		t.Fatal("a queued flow settled before it was sent")
	default:
	}
	item := <-o.dataChan
	item.sent(nil)
	requireSettled(t, p, true)

	counttest.ReleasesUnqueued(t, func(writeCtx, outputCtx context.Context) error {
		o := testOTLP(t)
		o.dataChan = make(chan queued[*logspb.LogRecord])
		o.ctx = outputCtx
		return o.ConsumeFlows(writeCtx, []embed.FlowRecord{{Packets: 1}})
	})
}

// TestOTLPGrpc_FailQueuedReleasesEverySignal pins that the records Stop finds
// still queued, of every signal, are released.
func TestOTLPGrpc_FailQueuedReleasesEverySignal(t *testing.T) {
	o := &OTLPGrpc{
		dataChan:   make(chan queued[*logspb.LogRecord], 1),
		metricChan: make(chan queued[*metricspb.Metric], 1),
		traceChan:  make(chan queued[*tracepb.Span], 1),
	}
	ls, lp := held(t)
	ms, mp := held(t)
	ts, tp := held(t)
	o.dataChan <- queued[*logspb.LogRecord]{v: &logspb.LogRecord{}, sent: ls}
	o.metricChan <- queued[*metricspb.Metric]{v: &metricspb.Metric{}, sent: ms}
	o.traceChan <- queued[*tracepb.Span]{v: &tracepb.Span{}, sent: ts}
	close(o.dataChan)
	close(o.metricChan)
	close(o.traceChan)

	o.failQueued()
	for _, p := range []count.Permit{lp, mp, tp} {
		requireSettled(t, p, false)
	}
}

// TestOTLPGrpc_PartialSuccessReleasesBatch pins delivery at least once: when
// the collector rejects part of a batch, which records it rejected is
// unknown, so the whole batch is released for replacement. The worker keeps
// running, since the connection is healthy.
func TestOTLPGrpc_PartialSuccessReleasesBatch(t *testing.T) {
	o := testOTLP(t)

	ls, lp := held(t)
	other, op := held(t)
	lb := newLogBatch(10, 0)
	lb.add(&logspb.LogRecord{}, ls)
	lb.add(&logspb.LogRecord{}, other)
	require.NoError(t, o.sendBatch(mockLogsClient{rejected: 1}, lb))
	requireSettled(t, lp, false)
	requireSettled(t, op, false)

	ms, mp := held(t)
	mb := newMetricBatch(10, 0)
	mb.add(&metricspb.Metric{}, ms)
	require.NoError(t, o.sendMetricBatch(mockMetricsClient{rejected: 1}, mb))
	requireSettled(t, mp, false)

	ts, tp := held(t)
	tb := newTraceBatch(10, 0)
	tb.add(&tracepb.Span{}, ts)
	require.NoError(t, o.sendTraceBatch(mockTraceClient{rejected: 1}, tb))
	requireSettled(t, tp, false)
}

// queuedOutput returns a test output whose closed queues hold n records of
// each signal, with their permits, as Stop leaves them for the workers.
func queuedOutput(t *testing.T, n int) (*OTLPGrpc, []count.Permit) {
	t.Helper()
	o := testOTLP(t)
	o.logger = zap.NewNop()
	o.dataChan = make(chan queued[*logspb.LogRecord], n)
	o.metricChan = make(chan queued[*metricspb.Metric], n)
	o.traceChan = make(chan queued[*tracepb.Span], n)
	var permits []count.Permit
	for range n {
		ls, lp := held(t)
		ms, mp := held(t)
		ts, tp := held(t)
		o.dataChan <- queued[*logspb.LogRecord]{v: &logspb.LogRecord{}, sent: ls}
		o.metricChan <- queued[*metricspb.Metric]{v: &metricspb.Metric{}, sent: ms}
		o.traceChan <- queued[*tracepb.Span]{v: &tracepb.Span{}, sent: ts}
		permits = append(permits, lp, mp, tp)
	}
	close(o.dataChan)
	close(o.metricChan)
	close(o.traceChan)
	return o, permits
}

// TestOTLPGrpc_ShutdownDrainsQueued pins that a worker shutting down sends
// every record still queued, in full batches and then the partial ones.
func TestOTLPGrpc_ShutdownDrainsQueued(t *testing.T) {
	o, permits := queuedOutput(t, 3)
	o.shutdown(mockLogsClient{}, mockMetricsClient{}, mockTraceClient{},
		newLogBatch(2, time.Hour), newMetricBatch(2, time.Hour), newTraceBatch(2, time.Hour), 0)
	for _, p := range permits {
		requireSettled(t, p, true)
	}
}

// TestOTLPGrpc_ShutdownStopsOnFailedDrain pins that a failed send during the
// drain releases its batch and stops, leaving the rest queued for Stop's
// failQueued, so every record still settles.
func TestOTLPGrpc_ShutdownStopsOnFailedDrain(t *testing.T) {
	o, permits := queuedOutput(t, 3)
	boom := errors.New("unavailable")
	o.shutdown(mockLogsClient{err: boom}, mockMetricsClient{}, mockTraceClient{},
		newLogBatch(1, time.Hour), newMetricBatch(1, time.Hour), newTraceBatch(1, time.Hour), 0)
	require.True(t, counttest.Settled(permits[0]), "the failed log batch is released")
	require.False(t, permits[0].Delivered())
	select {
	case <-permits[1].Settled():
		t.Fatal("the drain stopped at the failed log batch, so the metric is still queued")
	default:
	}

	o.failQueued()
	for _, p := range permits {
		requireSettled(t, p, false)
	}

	// A failed metric or trace send stops the drain the same way.
	for name, fail := range map[string][2]error{"metrics": {boom, nil}, "traces": {nil, boom}} {
		o, permits := queuedOutput(t, 2)
		o.shutdown(mockLogsClient{}, mockMetricsClient{err: fail[0]}, mockTraceClient{err: fail[1]},
			newLogBatch(1, time.Hour), newMetricBatch(1, time.Hour), newTraceBatch(1, time.Hour), 0)
		o.failQueued()
		for _, p := range permits {
			require.True(t, counttest.Settled(p), name)
		}
	}
}

// TestOTLPGrpc_FailedFinalFlushReleasesAndLogs pins that when the final
// flush of a signal fails, its records are released and the failure is
// logged, and the other signals still flush.
func TestOTLPGrpc_FailedFinalFlushReleasesAndLogs(t *testing.T) {
	boom := errors.New("unavailable")
	for name, fail := range map[string][3]error{
		"logs": {boom, nil, nil}, "metrics": {nil, boom, nil}, "traces": {nil, nil, boom},
	} {
		t.Run(name, func(t *testing.T) {
			o := testOTLP(t)
			core, logs := observer.New(zap.ErrorLevel)
			o.logger = zap.New(core)
			lb, mb, tb := newLogBatch(2, time.Hour), newMetricBatch(2, time.Hour), newTraceBatch(2, time.Hour)
			ls, lp := held(t)
			ms, mp := held(t)
			ts, tp := held(t)
			lb.add(&logspb.LogRecord{}, ls)
			mb.add(&metricspb.Metric{}, ms)
			tb.add(&tracepb.Span{}, ts)

			o.flushAll(mockLogsClient{err: fail[0]}, mockMetricsClient{err: fail[1]}, mockTraceClient{err: fail[2]},
				lb, mb, tb, 0)
			for i, p := range []count.Permit{lp, mp, tp} {
				requireSettled(t, p, fail[i] == nil)
			}
			require.Equal(t, 1, logs.FilterMessageSnippet("Failed to flush final").Len())
		})
	}
}

// TestOTLPGrpc_FailedFullBatchReleasesAndLogs pins that when a signal's full
// batch fails to send, its record is released and the failure is logged.
func TestOTLPGrpc_FailedFullBatchReleasesAndLogs(t *testing.T) {
	for name, tc := range map[string]struct {
		port  func() string
		write func(context.Context, *OTLPGrpc) error
	}{
		"log": {
			func() string { return startServer(t, failingLogsServer{}, okMetricsServer{}, okTraceServer{}) },
			func(ctx context.Context, o *OTLPGrpc) error { return o.WriteLog(ctx, output.LogRecord{Message: "x"}) },
		},
		"metric": {
			func() string { return startServer(t, okLogsServer{}, failingMetricsServer{}, okTraceServer{}) },
			func(ctx context.Context, o *OTLPGrpc) error {
				return o.WriteMetric(ctx, output.MetricRecord{Name: "m"})
			},
		},
		"trace": {
			func() string { return startServer(t, okLogsServer{}, okMetricsServer{}, failingTraceServer{}) },
			func(ctx context.Context, o *OTLPGrpc) error { return o.WriteTrace(ctx, output.TraceRecord{Name: "s"}) },
		},
	} {
		t.Run(name, func(t *testing.T) {
			core, logs := observer.New(zap.ErrorLevel)
			o, err := New(zap.New(core), WithHost("127.0.0.1"), WithPort(tc.port()), WithInsecure(true),
				WithWorkers(1), WithBatchTimeout(time.Hour), WithMaxExportBatchSize(1))
			require.NoError(t, err)
			t.Cleanup(func() { _ = o.Stop(context.Background()) })

			ctx, p := counttest.Held(t)
			require.NoError(t, tc.write(ctx, o))
			p.Commit()
			requireSettled(t, p, false)
			require.Eventually(t, func() bool {
				return logs.FilterMessage("Failed to send "+name+" batch").Len() == 1
			}, 5*time.Second, time.Millisecond)
		})
	}
}

// TestOTLPGrpc_FullBatchesCount pins that a worker sends each signal's batch
// as it fills and counts its records then, not at Stop.
func TestOTLPGrpc_FullBatchesCount(t *testing.T) {
	o, err := New(zap.NewNop(), WithHost("127.0.0.1"), WithPort(okServer(t)), WithInsecure(true),
		WithWorkers(1), WithBatchTimeout(time.Hour), WithMaxExportBatchSize(1))
	require.NoError(t, err)
	t.Cleanup(func() { _ = o.Stop(context.Background()) })

	for _, write := range []func(context.Context) error{
		func(ctx context.Context) error { return o.WriteLog(ctx, output.LogRecord{Message: "x"}) },
		func(ctx context.Context) error { return o.WriteMetric(ctx, output.MetricRecord{Name: "m"}) },
		func(ctx context.Context) error { return o.WriteTrace(ctx, output.TraceRecord{Name: "s"}) },
	} {
		ctx, p := counttest.Held(t)
		require.NoError(t, write(ctx))
		p.Commit()
		requireSettled(t, p, true)
	}
}

// writeHeld writes one record of each signal with a held permit and returns
// the permits in log, metric, trace order.
func writeHeld(t *testing.T, o *OTLPGrpc) []count.Permit {
	t.Helper()
	var permits []count.Permit
	for _, write := range []func(context.Context) error{
		func(ctx context.Context) error { return o.WriteLog(ctx, output.LogRecord{Message: "x"}) },
		func(ctx context.Context) error { return o.WriteMetric(ctx, output.MetricRecord{Name: "m"}) },
		func(ctx context.Context) error { return o.WriteTrace(ctx, output.TraceRecord{Name: "s"}) },
	} {
		ctx, p := counttest.Held(t)
		require.NoError(t, write(ctx))
		p.Commit()
		permits = append(permits, p)
	}
	return permits
}

// TestOTLPGrpc_BatchTimeoutFlushCounts pins that the batch timeout sends the
// partial batches of every signal and counts their records, without Stop: a
// count run needs this to deliver its last records and end.
func TestOTLPGrpc_BatchTimeoutFlushCounts(t *testing.T) {
	o, err := New(zap.NewNop(), WithHost("127.0.0.1"), WithPort(okServer(t)), WithInsecure(true),
		WithWorkers(1), WithBatchTimeout(50*time.Millisecond), WithMaxExportBatchSize(100))
	require.NoError(t, err)
	t.Cleanup(func() { _ = o.Stop(context.Background()) })

	for _, p := range writeHeld(t, o) {
		requireSettled(t, p, true)
	}
}

// TestOTLPGrpc_FailedTimeoutFlushReleasesAll pins that when the timed flush
// of any signal fails, its records are released and the worker's other
// records all settle. Each signal has its own timer, so another signal may
// flush before the failure or be released when the worker exits.
func TestOTLPGrpc_FailedTimeoutFlushReleasesAll(t *testing.T) {
	for name, port := range map[string]func() string{
		"logs":    func() string { return startServer(t, failingLogsServer{}, okMetricsServer{}, okTraceServer{}) },
		"metrics": func() string { return startServer(t, okLogsServer{}, failingMetricsServer{}, okTraceServer{}) },
		"traces":  func() string { return startServer(t, okLogsServer{}, okMetricsServer{}, failingTraceServer{}) },
	} {
		t.Run(name, func(t *testing.T) {
			o, err := New(zap.NewNop(), WithHost("127.0.0.1"), WithPort(port()), WithInsecure(true),
				WithWorkers(1), WithBatchTimeout(200*time.Millisecond), WithMaxExportBatchSize(100))
			require.NoError(t, err)
			t.Cleanup(func() { _ = o.Stop(context.Background()) })

			permits := writeHeld(t, o)
			for _, p := range permits {
				require.True(t, counttest.Settled(p))
			}
			failed := map[string]int{"logs": 0, "metrics": 1, "traces": 2}[name]
			require.False(t, permits[failed].Delivered(), "the failed signal's record is released")
		})
	}
}

// TestOTLPGrpc_EachSignalFlushesOnItsOwnTimeout pins that each signal's batch
// times out on its own: steady log traffic that fills log batches must not
// hold back a partial metric or trace batch.
func TestOTLPGrpc_EachSignalFlushesOnItsOwnTimeout(t *testing.T) {
	o, err := New(zap.NewNop(), WithHost("127.0.0.1"), WithPort(okServer(t)), WithInsecure(true),
		WithWorkers(1), WithBatchTimeout(200*time.Millisecond), WithMaxExportBatchSize(2))
	require.NoError(t, err)
	t.Cleanup(func() { _ = o.Stop(context.Background()) })

	// Cleanups run last first, so the writer exits before Stop closes its
	// channel.
	done, exited := make(chan struct{}), make(chan struct{})
	t.Cleanup(func() { close(done); <-exited })
	go func() {
		defer close(exited)
		tick := time.NewTicker(20 * time.Millisecond)
		defer tick.Stop()
		for {
			select {
			case <-done:
				return
			case <-tick.C:
				_ = o.WriteLog(context.Background(), output.LogRecord{Message: "steady"})
			}
		}
	}()

	for _, write := range []func(context.Context) error{
		func(ctx context.Context) error { return o.WriteMetric(ctx, output.MetricRecord{Name: "m"}) },
		func(ctx context.Context) error { return o.WriteTrace(ctx, output.TraceRecord{Name: "s"}) },
	} {
		ctx, p := counttest.Held(t)
		require.NoError(t, write(ctx))
		p.Commit()
		requireSettled(t, p, true)
	}
}

// TestOTLPGrpc_NonPositiveTimeoutsUseDefault pins that a batch or request
// timeout of 0 or less falls back to the default, like the queue and batch
// sizes.
func TestOTLPGrpc_NonPositiveTimeoutsUseDefault(t *testing.T) {
	o, err := New(zap.NewNop(), WithHost("127.0.0.1"), WithPort("1"), WithInsecure(true),
		WithBatchTimeout(0), WithRequestTimeout(0))
	require.NoError(t, err)
	t.Cleanup(func() { _ = o.Stop(context.Background()) })
	require.Equal(t, DefaultOTLPGrpcBatchTimeout, o.batchTimeout)
	require.Equal(t, DefaultOTLPGrpcRequestTimeout, o.requestTimeout)
}
