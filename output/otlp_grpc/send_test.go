package otlpgrpc

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/observiq/blitz/embed"
	"github.com/observiq/blitz/output"
	"github.com/stretchr/testify/require"
	collectorlogs "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	collectormetrics "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	collectortrace "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	logspb "go.opentelemetry.io/proto/otlp/logs/v1"
	metricspb "go.opentelemetry.io/proto/otlp/metrics/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
	"google.golang.org/grpc"
)

type mockLogsClient struct{ err error }

func (m mockLogsClient) Export(context.Context, *collectorlogs.ExportLogsServiceRequest, ...grpc.CallOption) (*collectorlogs.ExportLogsServiceResponse, error) {
	return &collectorlogs.ExportLogsServiceResponse{}, m.err
}

type mockMetricsClient struct{ err error }

func (m mockMetricsClient) Export(context.Context, *collectormetrics.ExportMetricsServiceRequest, ...grpc.CallOption) (*collectormetrics.ExportMetricsServiceResponse, error) {
	return &collectormetrics.ExportMetricsServiceResponse{}, m.err
}

type mockTraceClient struct{ err error }

func (m mockTraceClient) Export(context.Context, *collectortrace.ExportTraceServiceRequest, ...grpc.CallOption) (*collectortrace.ExportTraceServiceResponse, error) {
	return &collectortrace.ExportTraceServiceResponse{}, m.err
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
	lb.add(&logspb.LogRecord{})
	require.NoError(t, o.sendBatch(mockLogsClient{}, lb))

	lbErr := newLogBatch(10, time.Second)
	lbErr.add(&logspb.LogRecord{})
	require.Error(t, o.sendBatch(mockLogsClient{err: errors.New("boom")}, lbErr))

	mb := newMetricBatch(10, time.Second)
	mb.add(&metricspb.Metric{})
	require.NoError(t, o.sendMetricBatch(mockMetricsClient{}, mb))

	mbErr := newMetricBatch(10, time.Second)
	mbErr.add(&metricspb.Metric{})
	require.Error(t, o.sendMetricBatch(mockMetricsClient{err: errors.New("boom")}, mbErr))

	tb := newTraceBatch(10, time.Second)
	tb.add(&tracepb.Span{})
	require.NoError(t, o.sendTraceBatch(mockTraceClient{}, tb))

	tbErr := newTraceBatch(10, time.Second)
	tbErr.add(&tracepb.Span{})
	require.Error(t, o.sendTraceBatch(mockTraceClient{err: errors.New("boom")}, tbErr))
}
