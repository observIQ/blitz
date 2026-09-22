package output

import (
	"context"
	"errors"
	"testing"

	"github.com/observiq/blitz/telemetry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

	// omit the metric/trace writer interfaces from this double when set,
	// simulating an output that cannot consume that signal at all.
	noMetricWriter bool
	noTraceWriter  bool
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

func TestMultiOutputWriteFansToAllChildren(t *testing.T) {
	a := &fakeOutput{supported: []telemetry.Type{telemetry.Logs}}
	b := &fakeOutput{supported: []telemetry.Type{telemetry.Logs}}
	m := NewMultiOutput(a, b)

	err := m.WriteLog(context.Background(), LogRecord{Message: "hello"})
	require.NoError(t, err)
	assert.Len(t, a.logs, 1)
	assert.Len(t, b.logs, 1)
	assert.Equal(t, "hello", a.logs[0].Message)
}

func TestMultiOutputWriteBestEffortAggregatesErrors(t *testing.T) {
	boom := errors.New("boom")
	a := &fakeOutput{supported: []telemetry.Type{telemetry.Logs}, writeErr: boom}
	b := &fakeOutput{supported: []telemetry.Type{telemetry.Logs}}
	m := NewMultiOutput(a, b)

	err := m.WriteLog(context.Background(), LogRecord{Message: "x"})
	require.Error(t, err)
	assert.ErrorIs(t, err, boom)
	// The healthy child still received the record despite the other failing.
	assert.Len(t, b.logs, 1)
}

func TestMultiOutputWriteMetricRoutesBySupportAndInterface(t *testing.T) {
	// declares Metrics and implements MetricWriter -> receives.
	metricChild := &fakeOutput{supported: []telemetry.Type{telemetry.Logs, telemetry.Metrics}}
	// implements MetricWriter but does NOT declare Metrics -> skipped.
	undeclared := &fakeOutput{supported: []telemetry.Type{telemetry.Logs}}
	// declares Metrics but does not implement MetricWriter -> skipped.
	noWriter := logsOnly{inner: &fakeOutput{supported: []telemetry.Type{telemetry.Logs, telemetry.Metrics}}}

	m := NewMultiOutput(metricChild, undeclared, noWriter)
	v := int64(1)
	err := m.WriteMetric(context.Background(), MetricRecord{Name: "m", Type: MetricTypeGauge, IntValue: &v})
	require.NoError(t, err)
	assert.Len(t, metricChild.metrics, 1)
	assert.Empty(t, undeclared.metrics)
	assert.Empty(t, noWriter.inner.metrics)
}

func TestMultiOutputWriteTraceRoutesBySupportAndInterface(t *testing.T) {
	traceChild := &fakeOutput{supported: []telemetry.Type{telemetry.Logs, telemetry.Traces}}
	undeclared := &fakeOutput{supported: []telemetry.Type{telemetry.Logs}}
	noWriter := logsOnly{inner: &fakeOutput{supported: []telemetry.Type{telemetry.Logs, telemetry.Traces}}}

	m := NewMultiOutput(traceChild, undeclared, noWriter)
	err := m.WriteTrace(context.Background(), TraceRecord{Name: "s"})
	require.NoError(t, err)
	assert.Len(t, traceChild.traces, 1)
	assert.Empty(t, undeclared.traces)
	assert.Empty(t, noWriter.inner.traces)
}

func TestMultiOutputWriteMetricAggregatesErrors(t *testing.T) {
	boom := errors.New("metric boom")
	a := &fakeOutput{supported: []telemetry.Type{telemetry.Metrics}, writeMetricErr: boom}
	b := &fakeOutput{supported: []telemetry.Type{telemetry.Metrics}}
	m := NewMultiOutput(a, b)

	v := int64(1)
	err := m.WriteMetric(context.Background(), MetricRecord{Name: "m", IntValue: &v})
	require.Error(t, err)
	assert.ErrorIs(t, err, boom)
	assert.Len(t, b.metrics, 1)
}

func TestMultiOutputSupportedTelemetryIsUnion(t *testing.T) {
	a := &fakeOutput{supported: []telemetry.Type{telemetry.Logs, telemetry.Metrics}}
	b := &fakeOutput{supported: []telemetry.Type{telemetry.Logs, telemetry.Traces}}
	m := NewMultiOutput(a, b)

	got := m.SupportedTelemetry()
	assert.ElementsMatch(t, []telemetry.Type{telemetry.Logs, telemetry.Metrics, telemetry.Traces}, got)
}

func TestMultiOutputStopStopsAllAndAggregates(t *testing.T) {
	boom := errors.New("stop boom")
	a := &fakeOutput{supported: []telemetry.Type{telemetry.Logs}, stopErr: boom}
	b := &fakeOutput{supported: []telemetry.Type{telemetry.Logs}}
	m := NewMultiOutput(a, b)

	err := m.Stop(context.Background())
	require.Error(t, err)
	assert.ErrorIs(t, err, boom)
	// Both stopped even though the first errored.
	assert.True(t, a.stopped)
	assert.True(t, b.stopped)
}

func TestMultiOutputImplementsWriterInterfaces(t *testing.T) {
	m := NewMultiOutput()
	var _ Output = m
	var _ LogWriter = m
	var _ MetricWriter = m
	var _ TraceWriter = m
}
