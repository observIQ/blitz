package stdout

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/observiq/blitz/output"
	"github.com/observiq/blitz/telemetry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

	err = out.Write(context.Background(), output.LogRecord{Message: "test log message"})
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
	logger := zaptest.NewLogger(t)
	stdout, err := New(logger)
	require.NoError(t, err)

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w

	v := 42.5
	rec := output.MetricRecord{
		Name:        "cpu.usage",
		Type:        output.MetricTypeGauge,
		DoubleValue: &v,
		Timestamp:   time.Now(),
		Attributes:  map[string]string{"host": "test"},
	}

	err = stdout.WriteMetric(context.Background(), rec)
	require.NoError(t, err)

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, err = buf.ReadFrom(r)
	require.NoError(t, err)

	assert.Contains(t, buf.String(), "cpu.usage")
	assert.Contains(t, buf.String(), "gauge")
}

func TestStdoutOutput_WriteTrace(t *testing.T) {
	logger := zaptest.NewLogger(t)
	stdout, err := New(logger)
	require.NoError(t, err)

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w

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

	err = out.Write(context.Background(), output.LogRecord{Message: "interval flush message"})
	require.NoError(t, err)

	// Wait long enough for the ticker to fire without calling Stop.
	time.Sleep(100 * time.Millisecond)

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, err = buf.ReadFrom(r)
	require.NoError(t, err)

	assert.Contains(t, buf.String(), "interval flush message")
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

	err = out.Write(context.Background(), output.LogRecord{Message: "stop flush message"})
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
				require.NoError(t, out.Write(context.Background(), output.LogRecord{Message: msg}))
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
