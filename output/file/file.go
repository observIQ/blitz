package file

import (
	"context"
	"fmt"
	"time"

	"github.com/observiq/blitz/internal/workermanager"
	"github.com/observiq/blitz/output"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"
	"gopkg.in/natefinch/lumberjack.v2"
)

const (
	// DefaultFileChannelSize is the default size of the data channel
	DefaultFileChannelSize = 100

	// DefaultFileWorkers is the default number of worker goroutines
	DefaultFileWorkers = 1

	// metric attribute keys/values
	metricAttrComponent       = "component"
	metricComponentOutputFile = "output_file"
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
	meter         metric.Meter
	writer        *lumberjack.Logger

	// Metrics
	fileLogsReceived     metric.Int64Counter
	fileActiveWorkers    metric.Int64Gauge
	fileLogRate          metric.Float64Counter
	fileRequestSizeBytes metric.Int64Histogram
	fileWriteErrors      metric.Int64Counter
}

// New creates a new File output instance
func New(logger *zap.Logger, path string, workers int, rotation RotationOptions) (*File, error) {
	var err error

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
	defer func() {
		if err != nil {
			cancel()
		}
	}()

	meter := otel.Meter("blitz-file-output")

	// Initialize metrics
	fileLogsReceived, err := meter.Int64Counter(
		"blitz.file.logs.received",
		metric.WithDescription("Number of logs received from the write channel"),
	)
	if err != nil {
		return nil, fmt.Errorf("create logs received counter: %w", err)
	}

	fileActiveWorkers, err := meter.Int64Gauge(
		"blitz.file.workers.active",
		metric.WithDescription("Number of active worker goroutines"),
	)
	if err != nil {
		return nil, fmt.Errorf("create active workers gauge: %w", err)
	}

	fileLogRate, err := meter.Float64Counter(
		"blitz.file.log.rate",
		metric.WithDescription("Rate at which logs are successfully written to file"),
	)
	if err != nil {
		return nil, fmt.Errorf("create log rate counter: %w", err)
	}

	fileRequestSizeBytes, err := meter.Int64Histogram(
		"blitz.file.request.size.bytes",
		metric.WithDescription("Size of write requests in bytes"),
	)
	if err != nil {
		return nil, fmt.Errorf("create request size histogram: %w", err)
	}

	fileWriteErrors, err := meter.Int64Counter(
		"blitz.file.write.errors",
		metric.WithDescription("Total number of file write errors"),
	)
	if err != nil {
		return nil, fmt.Errorf("create write errors counter: %w", err)
	}

	writer := &lumberjack.Logger{
		Filename:   path,
		MaxSize:    rotation.MaxSizeMB,
		MaxBackups: rotation.MaxBackups,
		MaxAge:     rotation.MaxAgeDays,
		Compress:   rotation.Compress,
		LocalTime:  rotation.LocalTime,
	}

	f := &File{
		logger:               logger.Named("output-file"),
		path:                 path,
		workers:              workers,
		dataChan:             make(chan string, DefaultFileChannelSize),
		ctx:                  ctx,
		cancel:               cancel,
		meter:                meter,
		writer:               writer,
		fileLogsReceived:     fileLogsReceived,
		fileActiveWorkers:    fileActiveWorkers,
		fileLogRate:          fileLogRate,
		fileRequestSizeBytes: fileRequestSizeBytes,
		fileWriteErrors:      fileWriteErrors,
	}

	f.logger.Info("Starting File output",
		zap.String("path", f.path),
		zap.Int("workers", f.workers),
		zap.Int("channel_size", DefaultFileChannelSize),
	)

	// Channel size gauge
	_, err = meter.Int64ObservableGauge(
		"blitz.file.channel.size",
		metric.WithDescription("Current size of the data channel"),
		metric.WithInt64Callback(func(_ context.Context, io metric.Int64Observer) error {
			io.Observe(int64(len(f.dataChan)))
			return nil
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("create channel size gauge: %w", err)
	}

	// Worker manager
	f.workerManager = workermanager.NewWorkerManager(f.logger, workers, f.fileWorker)

	// Record initial active workers count
	f.fileActiveWorkers.Record(context.Background(), int64(workers),
		metric.WithAttributeSet(
			attribute.NewSet(
				attribute.String(metricAttrComponent, metricComponentOutputFile),
			),
		),
	)

	f.workerManager.Start()

	return f, nil
}

// Write enqueues data for file workers.
func (f *File) Write(ctx context.Context, data output.LogRecord) error {
	select {
	case f.dataChan <- data.Message:
		f.fileLogsReceived.Add(ctx, 1,
			metric.WithAttributeSet(
				attribute.NewSet(
					attribute.String(metricAttrComponent, metricComponentOutputFile),
				),
			),
		)
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

	f.fileActiveWorkers.Record(ctx, 0,
		metric.WithAttributeSet(
			attribute.NewSet(
				attribute.String(metricAttrComponent, metricComponentOutputFile),
			),
		),
	)

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
	f.fileLogRate.Add(context.Background(), 1.0,
		metric.WithAttributeSet(
			attribute.NewSet(
				attribute.String(metricAttrComponent, metricComponentOutputFile),
			),
		),
	)
	f.fileRequestSizeBytes.Record(context.Background(), int64(bytesWritten),
		metric.WithAttributeSet(
			attribute.NewSet(
				attribute.String(metricAttrComponent, metricComponentOutputFile),
			),
		),
	)

	// Record latency as a histogram using Float64Histogram like TCP for symmetry
	// Use a separate metric name if needed in the future; omitted here to reduce metric cardinality
	_ = latency

	return nil
}

func (f *File) recordWriteError(errorType string, err error) {
	ctx := context.Background()
	f.fileWriteErrors.Add(ctx, 1,
		metric.WithAttributeSet(
			attribute.NewSet(
				attribute.String(metricAttrComponent, metricComponentOutputFile),
				attribute.String("error_type", errorType),
			),
		),
	)
}
