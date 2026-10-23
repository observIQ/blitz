package stdout

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/observiq/blitz/embed"
	"github.com/observiq/blitz/generator/count"
	"github.com/observiq/blitz/internal/counttest"
	"github.com/observiq/blitz/output"
	"github.com/observiq/blitz/telemetry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/metric"
	metricnoop "go.opentelemetry.io/otel/metric/noop"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

func TestNew(t *testing.T) {
	logger := zaptest.NewLogger(t)
	out, err := New(logger)

	assert.NoError(t, err)
	assert.NotNil(t, out)
	assert.Equal(t, logger.Named("output-stdout"), out.logger)

	_ = out.Stop(context.Background())
}

func TestNew_NilLogger(t *testing.T) {
	out, err := New(nil)

	assert.Error(t, err)
	assert.Nil(t, out)
	assert.Contains(t, err.Error(), "logger cannot be nil")
}

func TestNew_WithFlushInterval(t *testing.T) {
	logger := zaptest.NewLogger(t)

	out, err := New(logger, WithFlushInterval(50*time.Millisecond))
	require.NoError(t, err)
	assert.Equal(t, 50*time.Millisecond, out.flushInterval)
	_ = out.Stop(context.Background())

	_, err = New(logger, WithFlushInterval(0))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "flush interval must be > 0")

	_, err = New(logger, WithFlushInterval(-1*time.Millisecond))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "flush interval must be > 0")
}

func TestStdoutOutput_Write(t *testing.T) {
	// Redirect os.Stdout before New so bufio wraps the pipe.
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w
	defer func() { os.Stdout = oldStdout }()

	logger := zaptest.NewLogger(t)
	out, err := New(logger)
	require.NoError(t, err)

	err = out.WriteLog(context.Background(), output.LogRecord{Message: "test log message"})
	require.NoError(t, err)

	// Stop flushes the buffer before we read.
	require.NoError(t, out.Stop(context.Background()))
	w.Close()

	var buf bytes.Buffer
	_, err = buf.ReadFrom(r)
	require.NoError(t, err)

	assert.Contains(t, buf.String(), "test log message")
}

func TestStdoutOutput_WriteMetric(t *testing.T) {
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w

	stdout, err := New(zaptest.NewLogger(t))
	require.NoError(t, err)

	v := 42.5
	rec := output.MetricRecord{
		Name:        "cpu.usage",
		Type:        output.MetricTypeGauge,
		DoubleValue: &v,
		Metadata: output.MetricPointMetadata{
			Timestamp:  time.Now(),
			Attributes: map[string]string{"host": "test"},
		},
	}

	err = stdout.WriteMetric(context.Background(), rec)
	require.NoError(t, err)
	require.NoError(t, stdout.Stop(context.Background()))

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, err = buf.ReadFrom(r)
	require.NoError(t, err)

	assert.Contains(t, buf.String(), "cpu.usage")
	assert.Contains(t, buf.String(), "gauge")
}

func TestStdoutOutput_WriteTrace(t *testing.T) {
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w

	stdout, err := New(zaptest.NewLogger(t))
	require.NoError(t, err)

	rec := output.TraceRecord{
		TraceID:   "abc123",
		SpanID:    "span456",
		Name:      "GET /api/users",
		Kind:      output.SpanKindServer,
		StartTime: time.Now(),
		EndTime:   time.Now().Add(100 * time.Millisecond),
	}

	err = stdout.WriteTrace(context.Background(), rec)
	require.NoError(t, err)
	require.NoError(t, stdout.Stop(context.Background()))

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, err = buf.ReadFrom(r)
	require.NoError(t, err)

	assert.Contains(t, buf.String(), "abc123")
	assert.Contains(t, buf.String(), "GET /api/users")
}

func TestStdoutOutput_SupportedTelemetry(t *testing.T) {
	logger := zaptest.NewLogger(t)
	stdout, err := New(logger)
	require.NoError(t, err)

	types := stdout.SupportedTelemetry()
	assert.Equal(t, []telemetry.Type{telemetry.Logs, telemetry.Metrics, telemetry.Traces}, types)
}

func TestStdoutOutput_FlushOnInterval(t *testing.T) {
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w
	defer func() { os.Stdout = oldStdout }()

	logger := zaptest.NewLogger(t)
	out, err := New(logger, WithFlushInterval(10*time.Millisecond))
	require.NoError(t, err)
	defer func() { _ = out.Stop(context.Background()) }()

	// Drain the pipe concurrently into a mutex-guarded buffer so we can poll
	// for the flushed data without blocking on ReadFrom (which only returns at
	// EOF). The ticker fires on its own goroutine, so we wait for its effect
	// rather than sleeping a fixed duration.
	var (
		mu  sync.Mutex
		got []byte
	)
	readerDone := make(chan struct{})
	go func() {
		defer close(readerDone)
		b := make([]byte, 1024)
		for {
			n, readErr := r.Read(b)
			if n > 0 {
				mu.Lock()
				got = append(got, b[:n]...)
				mu.Unlock()
			}
			if readErr != nil {
				return
			}
		}
	}()

	err = out.WriteLog(context.Background(), output.LogRecord{Message: "interval flush message"})
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return bytes.Contains(got, []byte("interval flush message"))
	}, 5*time.Second, 10*time.Millisecond)

	w.Close()
	os.Stdout = oldStdout
	<-readerDone
}

func TestStdoutOutput_FlushOnStop(t *testing.T) {
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w
	defer func() { os.Stdout = oldStdout }()

	logger := zaptest.NewLogger(t)
	// Use a very long flush interval so no automatic flush fires.
	out, err := New(logger, WithFlushInterval(10*time.Second))
	require.NoError(t, err)

	err = out.WriteLog(context.Background(), output.LogRecord{Message: "stop flush message"})
	require.NoError(t, err)

	// Stop performs a final flush before returning.
	require.NoError(t, out.Stop(context.Background()))
	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, err = buf.ReadFrom(r)
	require.NoError(t, err)

	assert.Contains(t, buf.String(), "stop flush message")
}

func TestStdoutOutput_ConcurrentWrites(t *testing.T) {
	const workers = 8
	const msgsPerWorker = 50

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w
	defer func() { os.Stdout = oldStdout }()

	logger := zaptest.NewLogger(t)
	out, err := New(logger)
	require.NoError(t, err)

	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < msgsPerWorker; j++ {
				msg := fmt.Sprintf("worker-%d-msg-%d", id, j)
				require.NoError(t, out.WriteLog(context.Background(), output.LogRecord{Message: msg}))
			}
		}(i)
	}
	wg.Wait()

	require.NoError(t, out.Stop(context.Background()))
	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, err = buf.ReadFrom(r)
	require.NoError(t, err)

	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	assert.Len(t, lines, workers*msgsPerWorker)
}

func TestStdoutOutput_Stop(t *testing.T) {
	logger := zaptest.NewLogger(t)
	out, err := New(logger)
	require.NoError(t, err)

	err = out.Stop(context.Background())
	assert.NoError(t, err)
}

// failingMeter overrides the first instrument the output registry builds (an
// Int64Gauge) to error, so output.NewMetrics fails.
type failingMeter struct{ metric.Meter }

func (failingMeter) Int64Gauge(string, ...metric.Int64GaugeOption) (metric.Int64Gauge, error) {
	return nil, fmt.Errorf("instrument error")
}

type failingMeterProvider struct{ metric.MeterProvider }

func (failingMeterProvider) Meter(string, ...metric.MeterOption) metric.Meter {
	return failingMeter{Meter: metricnoop.NewMeterProvider().Meter("test")}
}

func TestStdout_MetricsError(t *testing.T) {
	_, err := New(zap.NewNop(), WithTelemetry(embed.TelemetrySettings{MeterProvider: failingMeterProvider{}}))
	require.Error(t, err)
	require.Contains(t, err.Error(), "build output metrics")
}

// TestStdout_SelfTelemetry confirms stdout records its entries metric through
// the injected MeterProvider and emits a gated flush span through the injected
// TracerProvider.
func TestStdout_SelfTelemetry(t *testing.T) {
	oldStdout := os.Stdout
	_, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w
	defer func() { os.Stdout = oldStdout; _ = w.Close() }()

	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	spanRec := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(spanRec))
	tel := embed.TelemetrySettings{MeterProvider: mp, TracerProvider: tp, PerBatchSpans: true}

	out, err := New(zap.NewNop(), WithFlushInterval(10*time.Millisecond), WithTelemetry(tel))
	require.NoError(t, err)
	require.NoError(t, out.WriteLog(context.Background(), output.LogRecord{Message: "x"}))

	require.Eventually(t, func() bool {
		for _, s := range spanRec.Ended() {
			if s.Name() == "blitz.output.stdout.flush" {
				return true
			}
		}
		return false
	}, 2*time.Second, 10*time.Millisecond, "expected a flush span")

	require.NoError(t, out.Stop(context.Background()))

	var rm metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(context.Background(), &rm))
	found := false
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name == "blitz.output.entries_received" {
				found = true
			}
		}
	}
	require.True(t, found, "expected blitz.output.entries_received counter")
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, fmt.Errorf("stdout closed") }

// newBufferedOutput builds an output over w with no flush loop, so the test
// decides when the buffer flushes.
func newBufferedOutput(t *testing.T, w interface{ Write([]byte) (int, error) }) *StdoutOutput {
	t.Helper()
	m, err := output.NewMetrics(embed.NopTelemetry().MeterProvider)
	require.NoError(t, err)
	return &StdoutOutput{logger: zap.NewNop(), tel: embed.NopTelemetry(), metrics: m, writer: bufio.NewWriter(w)}
}

// TestStdoutOutput_PermitSettlesOnFlush pins that a buffered line counts only
// once the buffer is flushed to stdout.
func TestStdoutOutput_PermitSettlesOnFlush(t *testing.T) {
	var buf bytes.Buffer
	o := newBufferedOutput(t, &buf)

	ctx, p := counttest.Held(t)
	require.NoError(t, o.WriteLog(ctx, output.LogRecord{Message: "held"}))
	p.Commit()
	counttest.StaysPending(t, p, "the line is still in the buffer")

	o.flush()
	require.True(t, counttest.Settled(p))
	require.True(t, p.Delivered())
	require.Equal(t, "held\n", buf.String())
}

// TestStdoutOutput_FailedFlushReleasesPermits pins that lines lost to a
// failed flush are not counted.
func TestStdoutOutput_FailedFlushReleasesPermits(t *testing.T) {
	o := newBufferedOutput(t, failingWriter{})

	ctx, p := counttest.Held(t)
	require.NoError(t, o.WriteLog(ctx, output.LogRecord{Message: "lost"}))
	p.Commit()
	o.flush()
	require.True(t, counttest.Settled(p))
	require.False(t, p.Delivered())
}

// TestStdoutOutput_WriteAfterStopReleasesPermit pins that a line written
// after Stop's final flush is released rather than left in the buffer,
// owed a flush that never comes.
func TestStdoutOutput_WriteAfterStopReleasesPermit(t *testing.T) {
	var buf bytes.Buffer
	o := newBufferedOutput(t, &buf)
	o.stopCh, o.doneCh = make(chan struct{}), make(chan struct{})
	close(o.doneCh)
	require.NoError(t, o.Stop(context.Background()))

	ctx, p := counttest.Held(t)
	require.Error(t, o.WriteLog(ctx, output.LogRecord{Message: "late"}))
	p.Commit()
	require.True(t, counttest.Settled(p))
	require.False(t, p.Delivered())
	require.Empty(t, buf.String())
}

// TestStdoutOutput_BufferWriteErrorReleasesPermit pins that a line the buffer
// cannot take, because flushing a full buffer failed, is released.
func TestStdoutOutput_BufferWriteErrorReleasesPermit(t *testing.T) {
	o := newBufferedOutput(t, failingWriter{})
	o.writer = bufio.NewWriterSize(failingWriter{}, 16)

	ctx, p := counttest.Held(t)
	require.Error(t, o.WriteLog(ctx, output.LogRecord{Message: strings.Repeat("x", 64)}))
	p.Commit()
	require.True(t, counttest.Settled(p))
	require.False(t, p.Delivered())
}

// TestStdoutOutput_NewlineWriteErrorReleasesPermit pins that a line whose
// trailing newline the buffer cannot take is released.
func TestStdoutOutput_NewlineWriteErrorReleasesPermit(t *testing.T) {
	o := newBufferedOutput(t, failingWriter{})
	o.writer = bufio.NewWriterSize(failingWriter{}, 16)

	ctx, p := counttest.Held(t)
	require.Error(t, o.WriteLog(ctx, output.LogRecord{Message: strings.Repeat("x", 16)}))
	p.Commit()
	require.True(t, counttest.Settled(p))
	require.False(t, p.Delivered())
}

// TestStdoutOutput_AllSignalsShareTheBuffer pins that metrics and spans go
// through the log buffer, so a line the buffer flushed in part is never split
// by another record, and each record counts once it is flushed.
func TestStdoutOutput_AllSignalsShareTheBuffer(t *testing.T) {
	var buf bytes.Buffer
	o := newBufferedOutput(t, &buf)
	o.writer = bufio.NewWriterSize(&buf, 16)

	long := strings.Repeat("x", 40)
	require.NoError(t, o.WriteLog(context.Background(), output.LogRecord{Message: long}))
	mctx, mp := counttest.Held(t)
	require.NoError(t, o.WriteMetric(mctx, output.MetricRecord{Name: "cpu.usage"}))
	mp.Commit()
	tctx, tp := counttest.Held(t)
	require.NoError(t, o.WriteTrace(tctx, output.TraceRecord{Name: "GET /"}))
	tp.Commit()
	counttest.StaysPending(t, mp, "the metric is still in the buffer")
	counttest.StaysPending(t, tp, "the span is still in the buffer")

	o.flush()
	lines := strings.Split(strings.TrimSuffix(buf.String(), "\n"), "\n")
	require.Len(t, lines, 3)
	assert.Equal(t, long, lines[0])
	assert.Contains(t, lines[1], "cpu.usage")
	assert.Contains(t, lines[2], "GET /")
	for _, p := range []count.Permit{mp, tp} {
		require.True(t, counttest.Settled(p))
		require.True(t, p.Delivered())
	}
}
