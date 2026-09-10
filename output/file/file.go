package file

import (
	"context"
	"fmt"
	"time"

	"github.com/observiq/blitz/embed"
	"github.com/observiq/blitz/internal/workermanager"
	"github.com/observiq/blitz/output"
	"github.com/observiq/blitz/telemetry"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"
	"gopkg.in/natefinch/lumberjack.v2"
)

const (
	// DefaultFileChannelSize is the default size of the data channel
	DefaultFileChannelSize = 100

	// DefaultFileWorkers is the default number of worker goroutines
	DefaultFileWorkers = 1

	// outputType is the output_type attribute value for file metrics.
	outputType = "file"
)

// RotationOptions contains file rotation settings
type RotationOptions struct {
	MaxSizeMB  int
	MaxBackups int
	MaxAgeDays int
	Compress   bool
	LocalTime  bool
}

// fileItem is one queued line plus the emit-span context it was written under,
// so the worker can parent its write span to the emit span.
type fileItem struct {
	ctx context.Context
	msg string
}

// File implements the Output interface for file writes
type File struct {
	logger        *zap.Logger
	tel           embed.TelemetrySettings
	path          string
	workers       int
	dataChan      chan fileItem
	ctx           context.Context
	cancel        context.CancelFunc
	workerManager *workermanager.WorkerManager
	writer        *lumberjack.Logger
	metrics       *output.Metrics
}

// New creates a new File output instance
func New(logger *zap.Logger, path string, workers int, rotation RotationOptions, tel embed.TelemetrySettings) (*File, error) {
	if logger == nil {
		return nil, fmt.Errorf("logger cannot be nil")
	}
	if path == "" {
		return nil, fmt.Errorf("path cannot be empty")
	}
	if workers <= 0 {
		workers = DefaultFileWorkers
	}

	m, err := output.NewMetrics(tel.MeterProvider)
	if err != nil {
		return nil, fmt.Errorf("build output metrics: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	writer := &lumberjack.Logger{
		Filename:   path,
		MaxSize:    rotation.MaxSizeMB,
		MaxBackups: rotation.MaxBackups,
		MaxAge:     rotation.MaxAgeDays,
		Compress:   rotation.Compress,
		LocalTime:  rotation.LocalTime,
	}

	f := &File{
		logger:   logger.Named("output-file"),
		tel:      tel,
		path:     path,
		workers:  workers,
		dataChan: make(chan fileItem, DefaultFileChannelSize),
		ctx:      ctx,
		cancel:   cancel,
		writer:   writer,
		metrics:  m,
	}

	f.logger.Info("Starting File output",
		zap.String("path", f.path),
		zap.Int("workers", f.workers),
		zap.Int("channel_size", DefaultFileChannelSize),
	)

	// Register observable metrics (queue_size)
	if err := f.metrics.InitObservable(f); err != nil {
		return nil, fmt.Errorf("init observable metrics: %w", err)
	}

	// Worker manager
	f.workerManager = workermanager.NewWorkerManager(f.logger, workers, f.fileWorker)

	// Record initial active workers count
	f.metrics.BlitzOutputActiveWorkersGauge.Record(context.Background(), int64(workers), outputType)

	f.workerManager.Start()

	return f, nil
}

// ObserveBlitzOutputQueueSize implements the output.ObservableCallbacks interface
func (f *File) ObserveBlitzOutputQueueSize(_ context.Context, observer metric.Int64Observer) error {
	observer.Observe(int64(len(f.dataChan)))
	return nil
}

// Write enqueues data for file workers.
func (f *File) Write(ctx context.Context, data output.LogRecord) error {
	select {
	case f.dataChan <- fileItem{ctx: ctx, msg: data.Message}:
		f.metrics.BlitzOutputEntriesReceivedCounter.Add(ctx, 1, outputType, "logs")
		return nil
	case <-ctx.Done():
		return fmt.Errorf("context cancelled while waiting to write data: %w", ctx.Err())
	case <-f.ctx.Done():
		return fmt.Errorf("file output is shutting down")
	}
}

// Stop gracefully stops workers and closes the writer
func (f *File) Stop(ctx context.Context) error {
	f.logger.Info("Stopping File output")

	f.metrics.BlitzOutputActiveWorkersGauge.Record(ctx, 0, outputType)

	close(f.dataChan)
	f.cancel()
	f.workerManager.Stop()

	if err := f.writer.Close(); err != nil {
		return fmt.Errorf("close file writer: %w", err)
	}

	f.logger.Info("File output stopped successfully")
	return nil
}

// fileWorker processes data from the channel and writes to the configured file
func (f *File) fileWorker(id int) {
	f.logger.Info("Starting File worker", zap.Int("worker_id", id))

	for {
		select {
		case item, ok := <-f.dataChan:
			if !ok {
				f.logger.Info("File worker exiting - channel closed", zap.Int("worker_id", id))
				return
			}

			// The write span covers the lumberjack write, which transparently
			// absorbs any file rotation that fires during it.
			_, span := output.StartSendSpan(item.ctx, f.tel, "blitz.output.file.write")
			err := f.writeData(item.msg)
			if err != nil {
				span.RecordError(err)
			}
			span.End()
			if err != nil {
				f.logger.Error("Failed to write file data", zap.Int("worker_id", id), zap.Error(err))
				return
			}

		case <-f.ctx.Done():
			f.logger.Info("File worker exiting - context cancelled", zap.Int("worker_id", id))
			return
		}
	}
}

// writeData writes a single line to the file and records metrics.
// It is safe to call concurrently from multiple workers: the underlying
// lumberjack.Logger implementation serializes writes with an internal mutex,
// so concurrent Write calls will not interleave. Note that while writes are
// serialized, overall line ordering across workers is not guaranteed.
func (f *File) writeData(data string) error {
	start := time.Now()

	// Append newline to maintain line-based logs
	bytesWritten, err := f.writer.Write(append([]byte(data), '\n'))
	if err != nil {
		f.recordWriteError("write", err)
		return fmt.Errorf("write to file: %w", err)
	}

	latency := time.Since(start).Seconds()
	f.metrics.BlitzOutputEntryRateCounter.Add(context.Background(), 1.0, outputType, "logs")
	f.metrics.BlitzOutputRequestSizeHistogram.Record(context.Background(), int64(bytesWritten), outputType, "logs")

	// Record latency as a histogram using Float64Histogram like TCP for symmetry
	// Use a separate metric name if needed in the future; omitted here to reduce metric cardinality
	_ = latency

	return nil
}

func (f *File) recordWriteError(_ string, _ error) {
	f.metrics.BlitzOutputSendErrorsCounter.Add(context.Background(), 1, outputType, "logs")
}

// SupportedTelemetry returns the telemetry types this output can consume.
func (f *File) SupportedTelemetry() []telemetry.Type {
	return []telemetry.Type{telemetry.Logs}
}
