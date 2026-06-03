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

type recordingMetricWriter struct {
	mu      sync.Mutex
	records []output.MetricRecord
	err     error
}

func (w *recordingMetricWriter) WriteMetric(_ context.Context, rec output.MetricRecord) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.err != nil {
		return w.err
	}
	w.records = append(w.records, rec)
	return nil
}

func TestWriterAsMetricConsumerPushesEachPoint(t *testing.T) {
	w := &recordingMetricWriter{}
	c := output.WriterAsMetricConsumer(w)

	batch := []embed.MetricPoint{
		{Name: "one"},
		{Name: "two"},
		{Name: "three"},
	}
	if err := c.ConsumeMetrics(context.Background(), batch); err != nil {
		t.Fatalf("ConsumeMetrics: %v", err)
	}
	if got, want := len(w.records), 3; got != want {
		t.Fatalf("recorded %d, want %d", got, want)
	}
	for i, want := range []string{"one", "two", "three"} {
		if w.records[i].Name != want {
			t.Errorf("point %d: %q, want %q", i, w.records[i].Name, want)
		}
	}
}

func TestWriterAsMetricConsumerStopsOnFirstError(t *testing.T) {
	wantErr := errors.New("metric boom")
	w := &recordingMetricWriter{err: wantErr}
	c := output.WriterAsMetricConsumer(w)

	err := c.ConsumeMetrics(context.Background(), []embed.MetricPoint{
		{Name: "one"},
		{Name: "two"},
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected wantErr, got %v", err)
	}
}

func TestWriterAsLogConsumerPanicsOnNilWriter(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on nil writer, got none")
		}
	}()
	_ = output.WriterAsLogConsumer(nil)
}

func TestWriterAsMetricConsumerPanicsOnNilWriter(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on nil writer, got none")
		}
	}()
	_ = output.WriterAsMetricConsumer(nil)
}
