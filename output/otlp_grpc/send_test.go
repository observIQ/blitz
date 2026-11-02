package otlpgrpc

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/observiq/blitz/embed"
	"github.com/observiq/blitz/output"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	collectorlogs "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	collectormetrics "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	collectortrace "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	logspb "go.opentelemetry.io/proto/otlp/logs/v1"
	metricspb "go.opentelemetry.io/proto/otlp/metrics/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
	"google.golang.org/grpc"
)

type mockLogsClient struct {
	err      error
	rejected int64
}

func (m mockLogsClient) Export(context.Context, *collectorlogs.ExportLogsServiceRequest, ...grpc.CallOption) (*collectorlogs.ExportLogsServiceResponse, error) {
	resp := &collectorlogs.ExportLogsServiceResponse{}
	if m.rejected > 0 {
		resp.PartialSuccess = &collectorlogs.ExportLogsPartialSuccess{RejectedLogRecords: m.rejected, ErrorMessage: "memory limit"}
	}
	return resp, m.err
}

type mockMetricsClient struct {
	err      error
	rejected int64
}

func (m mockMetricsClient) Export(context.Context, *collectormetrics.ExportMetricsServiceRequest, ...grpc.CallOption) (*collectormetrics.ExportMetricsServiceResponse, error) {
	resp := &collectormetrics.ExportMetricsServiceResponse{}
	if m.rejected > 0 {
		resp.PartialSuccess = &collectormetrics.ExportMetricsPartialSuccess{RejectedDataPoints: m.rejected, ErrorMessage: "memory limit"}
	}
	return resp, m.err
}

type mockTraceClient struct {
	err      error
	rejected int64
}

func (m mockTraceClient) Export(context.Context, *collectortrace.ExportTraceServiceRequest, ...grpc.CallOption) (*collectortrace.ExportTraceServiceResponse, error) {
	resp := &collectortrace.ExportTraceServiceResponse{}
	if m.rejected > 0 {
		resp.PartialSuccess = &collectortrace.ExportTracePartialSuccess{RejectedSpans: m.rejected, ErrorMessage: "memory limit"}
	}
	return resp, m.err
}

// testOTLP builds a minimal OTLPGrpc for exercising the send methods directly,
// without standing up workers or a live collector.
func testOTLP(t *testing.T) *OTLPGrpc {
	t.Helper()
	m, err := output.NewMetrics(nil)
	require.NoError(t, err)
	return &OTLPGrpc{
		ctx:            context.Background(),
		tel:            embed.TelemetrySettings{PerBatchSpans: true},
		metrics:        m,
		requestTimeout: time.Second,
		batchTimeout:   time.Second,
	}
}

// TestOTLPGrpc_sendBatchesEmitSpans exercises the three batch-send methods on
// both the success and error paths, covering the gated send span (including
// span.RecordError on failure) and the batch-size attribute.
func TestOTLPGrpc_sendBatchesEmitSpans(t *testing.T) {
	o := testOTLP(t)

	lb := newLogBatch(10, time.Second)
	lb.add(&logspb.LogRecord{}, func(error) {})
	require.NoError(t, o.sendBatch(mockLogsClient{}, lb))

	lbErr := newLogBatch(10, time.Second)
	lbErr.add(&logspb.LogRecord{}, func(error) {})
	require.Error(t, o.sendBatch(mockLogsClient{err: errors.New("boom")}, lbErr))

	mb := newMetricBatch(10, time.Second)
	mb.add(&metricspb.Metric{}, func(error) {})
	require.NoError(t, o.sendMetricBatch(mockMetricsClient{}, mb))

	mbErr := newMetricBatch(10, time.Second)
	mbErr.add(&metricspb.Metric{}, func(error) {})
	require.Error(t, o.sendMetricBatch(mockMetricsClient{err: errors.New("boom")}, mbErr))

	tb := newTraceBatch(10, time.Second)
	tb.add(&tracepb.Span{}, func(error) {})
	require.NoError(t, o.sendTraceBatch(mockTraceClient{}, tb))

	tbErr := newTraceBatch(10, time.Second)
	tbErr.add(&tracepb.Span{}, func(error) {})
	require.Error(t, o.sendTraceBatch(mockTraceClient{err: errors.New("boom")}, tbErr))
}

// TestOTLPGrpc_ConsumeFlows pins the flow path into the OTLP worker queue: each
// record is queued as one log record, a cancelled caller is told so, and an
// output shutting down refuses the write.
func TestOTLPGrpc_ConsumeFlows(t *testing.T) {
	recs := []embed.FlowRecord{{Packets: 1}, {Packets: 2}}

	o := testOTLP(t)
	o.dataChan = make(chan queued[*logspb.LogRecord], len(recs))
	require.NoError(t, o.ConsumeFlows(context.Background(), recs))
	require.Len(t, o.dataChan, len(recs), "one log record per flow")

	o = testOTLP(t)
	o.dataChan = make(chan queued[*logspb.LogRecord]) // never drained
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	require.ErrorContains(t, o.ConsumeFlows(ctx, recs), "context cancelled while waiting to write flow")

	o = testOTLP(t)
	o.dataChan = make(chan queued[*logspb.LogRecord])
	octx, ocancel := context.WithCancel(context.Background())
	ocancel()
	o.ctx = octx
	require.ErrorContains(t, o.ConsumeFlows(context.Background(), recs), "shutting down")
}

// TestOTLPGrpc_ExportErrorRecordedOnSpan pins that a failed export records
// the error on its send span, for every signal.
func TestOTLPGrpc_ExportErrorRecordedOnSpan(t *testing.T) {
	rec := tracetest.NewSpanRecorder()
	o := testOTLP(t)
	o.tel.TracerProvider = sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(rec))
	boom := errors.New("boom")

	lb := newLogBatch(10, time.Second)
	lb.add(&logspb.LogRecord{}, func(error) {})
	require.Error(t, o.sendBatch(mockLogsClient{err: boom}, lb))
	mb := newMetricBatch(10, time.Second)
	mb.add(&metricspb.Metric{}, func(error) {})
	require.Error(t, o.sendMetricBatch(mockMetricsClient{err: boom}, mb))
	tb := newTraceBatch(10, time.Second)
	tb.add(&tracepb.Span{}, func(error) {})
	require.Error(t, o.sendTraceBatch(mockTraceClient{err: boom}, tb))

	spans := rec.Ended()
	require.Len(t, spans, 3)
	for _, s := range spans {
		var signal string
		for _, a := range s.Attributes() {
			if a.Key == "blitz.signal" {
				signal = a.Value.AsString()
			}
		}
		events := s.Events()
		require.NotEmpty(t, events, "%s export error not recorded on its span", signal)
		assert.Equal(t, "exception", events[0].Name, signal)
	}
}
