package file

import (
	"bufio"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/observiq/blitz/embed"
	"github.com/observiq/blitz/generator/count"
	"github.com/observiq/blitz/internal/counttest"
	"github.com/observiq/blitz/internal/workermanager"
	"github.com/observiq/blitz/output"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"gopkg.in/natefinch/lumberjack.v2"
)

func TestFileOutputWriteSingleLine(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "blitz.log")

	logger := zap.NewNop()
	rot := RotationOptions{MaxSizeMB: 100, MaxBackups: 1, MaxAgeDays: 1, Compress: false, LocalTime: false}
	f, err := New(logger, path, 1, rot, embed.NopTelemetry())
	if err != nil {
		t.Fatalf("new file output: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	msg := "hello world"
	if err := f.WriteLog(ctx, output.LogRecord{Message: msg}); err != nil {
		t.Fatalf("write: %v", err)
	}

	// allow worker to process
	waitForLines(t, path, 1, 3*time.Second)

	if err := f.Stop(context.Background()); err != nil {
		t.Fatalf("stop: %v", err)
	}

	lines := readLines(t, path)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	if lines[0] != msg {
		t.Fatalf("unexpected line content: %q", lines[0])
	}
}

func TestFileOutputWriteMultipleLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "blitz.log")

	logger := zap.NewNop()
	rot := RotationOptions{MaxSizeMB: 100, MaxBackups: 2, MaxAgeDays: 2, Compress: false, LocalTime: false}
	f, err := New(logger, path, 2, rot, embed.NopTelemetry())
	if err != nil {
		t.Fatalf("new file output: %v", err)
	}

	ctx := context.Background()
	msgs := []string{"one", "two", "three"}
	for _, m := range msgs {
		if err := f.WriteLog(ctx, output.LogRecord{Message: m}); err != nil {
			t.Fatalf("write: %v", err)
		}
	}

	// allow workers to process
	waitForLines(t, path, len(msgs), 3*time.Second)

	if err := f.Stop(context.Background()); err != nil {
		t.Fatalf("stop: %v", err)
	}

	lines := readLines(t, path)
	if len(lines) != len(msgs) {
		t.Fatalf("expected %d lines, got %d", len(msgs), len(lines))
	}
}

func waitForLines(t *testing.T, path string, want int, timeout time.Duration) {
	t.Helper()
	require.Eventually(t, func() bool {
		return len(readLines(t, path)) >= want
	}, timeout, 50*time.Millisecond)
}

func readLines(t *testing.T, path string) []string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatalf("open file: %v", err)
	}
	defer func() { _ = f.Close() }()

	var lines []string
	s := bufio.NewScanner(f)
	for s.Scan() {
		lines = append(lines, s.Text())
	}
	return lines
}

func newTestFile(t *testing.T, path string) *File {
	t.Helper()
	f, err := New(zap.NewNop(), path, 1, RotationOptions{MaxSizeMB: 100}, embed.NopTelemetry())
	require.NoError(t, err)
	return f
}

// TestFile_PermitSettlesWhenWritten pins that a queued line's permit counts
// once the line is written to the file.
func TestFile_PermitSettlesWhenWritten(t *testing.T) {
	path := filepath.Join(t.TempDir(), "blitz.log")
	f := newTestFile(t, path)
	defer func() { _ = f.Stop(context.Background()) }()

	ctx, p := counttest.Held(t)
	require.NoError(t, f.WriteLog(ctx, output.LogRecord{Message: "held"}))
	p.Commit()
	require.True(t, counttest.Settled(p))
	require.True(t, p.Delivered())
	require.Equal(t, []string{"held"}, readLines(t, path))
}

// TestFile_WriteFailureReleasesPermit pins that a line the file cannot take
// is not counted.
func TestFile_WriteFailureReleasesPermit(t *testing.T) {
	notDir := filepath.Join(t.TempDir(), "plain")
	require.NoError(t, os.WriteFile(notDir, nil, 0o600))
	f := newTestFile(t, filepath.Join(notDir, "blitz.log"))
	defer func() { _ = f.Stop(context.Background()) }()

	ctx, p := counttest.Held(t)
	require.NoError(t, f.WriteLog(ctx, output.LogRecord{Message: "lost"}))
	p.Commit()
	require.True(t, counttest.Settled(p))
	require.False(t, p.Delivered())
}

// TestFile_StopWritesQueuedLines pins that lines still queued at Stop are
// written and counted rather than dropped.
func TestFile_StopWritesQueuedLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "blitz.log")
	m, err := output.NewMetrics(embed.NopTelemetry().MeterProvider)
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	// No workers, so both lines are still queued when Stop runs.
	f := &File{logger: zap.NewNop(), tel: embed.NopTelemetry(), metrics: m, ctx: ctx, cancel: cancel,
		writer: &lumberjack.Logger{Filename: path}, dataChan: make(chan fileItem, 2),
		workerManager: workermanager.NewWorkerManager(zap.NewNop(), 0, nil)}
	ctx1, p1 := counttest.Held(t)
	ctx2, p2 := counttest.Held(t)
	require.NoError(t, f.WriteLog(ctx1, output.LogRecord{Message: "one"}))
	require.NoError(t, f.WriteLog(ctx2, output.LogRecord{Message: "two"}))
	p1.Commit()
	p2.Commit()

	require.NoError(t, f.Stop(context.Background()))

	for _, p := range []count.Permit{p1, p2} {
		require.True(t, counttest.Settled(p))
		require.True(t, p.Delivered())
	}
	require.Equal(t, []string{"one", "two"}, readLines(t, path))
}

// TestFile_UnqueuedWriteReleasesPermit pins that a record that never reaches
// the queue is released.
func TestFile_UnqueuedWriteReleasesPermit(t *testing.T) {
	counttest.ReleasesUnqueued(t, func(writeCtx, outputCtx context.Context) error {
		f := &File{dataChan: make(chan fileItem), ctx: outputCtx}
		return f.WriteLog(writeCtx, output.LogRecord{Message: "x"})
	})
}
