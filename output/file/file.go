package file

import (
	"context"
	"fmt"
	"time"

	"github.com/observiq/blitz/internal/workermanager"
	"github.com/observiq/blitz/output"
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

// File implements the Output interface for file writes
type File struct {
	logger        *zap.Logger
	path          string
	workers       int
	dataChan      chan string
	ctx           context.Context
	cancel        context.CancelFunc
	workerManager *workermanager.WorkerManager
	writer        *lumberjack.Logger
}

// New creates a new File output instance
func New(logger *zap.Logger, path string, workers int, rotation RotationOptions) (*File, error) {
	if logger == nil {
		return nil, fmt.Errorf("logger cannot be nil")
	}
	if path == "" {
		return nil, fmt.Errorf("path cannot be empty")
	}
	if workers <= 0 {
		workers = DefaultFileWorkers
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
		path:     path,
		workers:  workers,
		dataChan: make(chan string, DefaultFileChannelSize),
		ctx:      ctx,
		cancel:   cancel,
		writer:   writer,
	}

	f.logger.Info("Starting File output",
		zap.String("path", f.path),
		zap.Int("workers", f.workers),
		zap.Int("channel_size", DefaultFileChannelSize),
	)

	// Register observable metrics (queue_size)
	output.InitObservableMetrics(f)

	// Worker manager
	f.workerManager = workermanager.NewWorkerManager(f.logger, workers, f.fileWorker)

	// Record initial active workers count
	output.BlitzOutputActiveWorkersGauge.Record(context.Background(), int64(workers), outputType)

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
	case f.dataChan <- data.Message:
		output.BlitzOutputEntriesReceivedCounter.Add(ctx, 1, outputType, "logs")
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

	output.BlitzOutputActiveWorkersGauge.Record(ctx, 0, outputType)

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
		case data, ok := <-f.dataChan:
			if !ok {
				f.logger.Info("File worker exiting - channel closed", zap.Int("worker_id", id))
				return
			}

			if err := f.writeData(data); err != nil {
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
	output.BlitzOutputEntryRateCounter.Add(context.Background(), 1.0, outputType, "logs")
	output.BlitzOutputRequestSizeHistogram.Record(context.Background(), int64(bytesWritten), outputType, "logs")

	// Record latency as a histogram using Float64Histogram like TCP for symmetry
	// Use a separate metric name if needed in the future; omitted here to reduce metric cardinality
	_ = latency

	return nil
}

func (f *File) recordWriteError(_ string, _ error) {
	output.BlitzOutputSendErrorsCounter.Add(context.Background(), 1, outputType, "logs")
}
