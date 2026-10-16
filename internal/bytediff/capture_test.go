package bytediff_test

import (
	"context"
	"sync"
	"testing"

	"github.com/observiq/blitz/embed"
	"github.com/observiq/blitz/internal/bytediff"
	"github.com/observiq/blitz/output"
	"github.com/observiq/blitz/telemetry"
)

func TestLogCaptureRecordsAllWrites(t *testing.T) {
	cap := bytediff.NewLogCapture()
	ctx := context.Background()

	want := []string{"first", "second", "third"}
	for _, msg := range want {
		if err := cap.WriteLog(ctx, output.LogRecord{Message: msg}); err != nil {
			t.Fatalf("write %q: %v", msg, err)
		}
	}

	got := cap.Messages()
	if len(got) != len(want) {
		t.Fatalf("expected %d messages, got %d", len(want), len(got))
	}
	for i, msg := range want {
		if got[i] != msg {
			t.Errorf("message %d: want %q, got %q", i, msg, got[i])
		}
	}
}

func TestLogCaptureReset(t *testing.T) {
	cap := bytediff.NewLogCapture()
	_ = cap.WriteLog(context.Background(), output.LogRecord{Message: "a"})
	cap.Reset()
	if got := len(cap.Messages()); got != 0 {
		t.Fatalf("expected empty after reset, got %d messages", got)
	}
}

func TestLogCaptureAcceptsLogsAndStopsCleanly(t *testing.T) {
	cap := bytediff.NewLogCapture()
	got := cap.SupportedTelemetry()
	if len(got) != 1 || got[0] != telemetry.Logs {
		t.Fatalf("expected [logs], got %v", got)
	}
	if err := cap.Stop(context.Background()); err != nil {
		t.Fatalf("stop: %v", err)
	}
}

func TestLogCaptureConcurrent(t *testing.T) {
	cap := bytediff.NewLogCapture()
	var wg sync.WaitGroup
	const workers = 8
	const writes = 100
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx := context.Background()
			for i := 0; i < writes; i++ {
				_ = cap.WriteLog(ctx, output.LogRecord{Message: "x"})
			}
		}()
	}
	wg.Wait()
	if got, want := len(cap.Records()), workers*writes; got != want {
		t.Fatalf("expected %d records, got %d", want, got)
	}
}

// Verify the capture satisfies the output.LogWriter interface so any
// generator wired against output.LogWriter can target it.
var _ output.LogWriter = (*bytediff.LogCapture)(nil)

// Verify the canonical embed.LogRecord type is what the capture stores
// — the alias chain output.LogRecord = embed.LogRecord must hold for
// PRs #2-11 to be able to swap one for the other transparently.
var _ embed.LogRecord = output.LogRecord{}
