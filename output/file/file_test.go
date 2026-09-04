package file

import (
	"bufio"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/observiq/blitz/embed"
	"github.com/observiq/blitz/output"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
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
	if err := f.Write(ctx, output.LogRecord{Message: msg}); err != nil {
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
		if err := f.Write(ctx, output.LogRecord{Message: m}); err != nil {
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
