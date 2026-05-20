package output_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/observiq/blitz/embed"
	"github.com/observiq/blitz/output"
)

type recordingWriter struct {
	mu      sync.Mutex
	records []output.LogRecord
	err     error
}

func (w *recordingWriter) Write(_ context.Context, rec output.LogRecord) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.err != nil {
		return w.err
	}
	w.records = append(w.records, rec)
	return nil
}

func TestWriterAsLogConsumerPushesEachRecord(t *testing.T) {
	w := &recordingWriter{}
	c := output.WriterAsLogConsumer(w)

	batch := []embed.LogRecord{
		{Message: "one"},
		{Message: "two"},
		{Message: "three"},
	}
	if err := c.ConsumeLogs(context.Background(), batch); err != nil {
		t.Fatalf("ConsumeLogs: %v", err)
	}
	if got, want := len(w.records), 3; got != want {
		t.Fatalf("recorded %d, want %d", got, want)
	}
	for i, want := range []string{"one", "two", "three"} {
		if w.records[i].Message != want {
			t.Errorf("record %d: %q, want %q", i, w.records[i].Message, want)
		}
	}
}

func TestWriterAsLogConsumerStopsOnFirstError(t *testing.T) {
	wantErr := errors.New("boom")
	w := &recordingWriter{err: wantErr}
	c := output.WriterAsLogConsumer(w)

	err := c.ConsumeLogs(context.Background(), []embed.LogRecord{
		{Message: "one"},
		{Message: "two"},
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected wrapped wantErr, got %v", err)
	}
}
