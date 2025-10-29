package output

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/goccy/go-json"
	"github.com/observiq/blitz/internal/workermanager"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	collectorlogs "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	logspb "go.opentelemetry.io/proto/otlp/logs/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
)

const (
	// DefaultOTLPGrpcChannelSize is the default size of the data channel
	DefaultOTLPGrpcChannelSize = 100

	// DefaultOTLPGrpcWorkers is the default number of worker goroutines
	DefaultOTLPGrpcWorkers = 1

	// DefaultOTLPGrpcHost is the default host for OTLP gRPC connections
	DefaultOTLPGrpcHost = "localhost"

	// DefaultOTLPGrpcPort is the default port for OTLP gRPC connections
	DefaultOTLPGrpcPort = "4317"

	// DefaultOTLPGrpcBatchTimeout is the default timeout for batching log records
	DefaultOTLPGrpcBatchTimeout = 5 * time.Second

	// DefaultOTLPGrpcMaxQueueSize is the default maximum queue size for batching
	DefaultOTLPGrpcMaxQueueSize = 2048

	// DefaultOTLPGrpcMaxExportBatchSize is the default maximum batch size for export
	DefaultOTLPGrpcMaxExportBatchSize = 512

	// DefaultOTLPGrpcStopTimeout is the default timeout for graceful shutdown
	DefaultOTLPGrpcStopTimeout = 30 * time.Second
)

// jsonLog represents a parsed JSON log entry
type jsonLog struct {
	Timestamp   time.Time `json:"timestamp"`
	Level       string    `json:"level"`
	Environment string    `json:"environment"`
	Location    string    `json:"location"`
	Message     string    `json:"message"`
}

// OTLPGrpcOption is a functional option for configuring OTLP gRPC output
type OTLPGrpcOption func(*OTLPGrpcConfig) error

// OTLPGrpcConfig holds configuration for OTLP gRPC output
type OTLPGrpcConfig struct {
	host               string
	port               string
	workers            int
	batchTimeout       time.Duration
	maxQueueSize       int
	maxExportBatchSize int
}

// WithHost sets the host for OTLP gRPC connections
func WithHost(host string) OTLPGrpcOption {
	return func(cfg *OTLPGrpcConfig) error {
		cfg.host = host
		return nil
	}
}

// WithPort sets the port for OTLP gRPC connections
func WithPort(port string) OTLPGrpcOption {
	return func(cfg *OTLPGrpcConfig) error {
		cfg.port = port
		return nil
	}
}

// WithWorkers sets the number of worker goroutines
func WithWorkers(workers int) OTLPGrpcOption {
	return func(cfg *OTLPGrpcConfig) error {
		cfg.workers = workers
		return nil
	}
}

// WithBatchTimeout sets the timeout for batching log records
func WithBatchTimeout(timeout time.Duration) OTLPGrpcOption {
	return func(cfg *OTLPGrpcConfig) error {
		cfg.batchTimeout = timeout
		return nil
	}
}

// WithMaxQueueSize sets the maximum queue size for batching
func WithMaxQueueSize(size int) OTLPGrpcOption {
	return func(cfg *OTLPGrpcConfig) error {
		cfg.maxQueueSize = size
		return nil
	}
}

// WithMaxExportBatchSize sets the maximum batch size for export
func WithMaxExportBatchSize(size int) OTLPGrpcOption {
	return func(cfg *OTLPGrpcConfig) error {
		cfg.maxExportBatchSize = size
		return nil
	}
}

// OTLPGrpc implements the Output interface for OTLP gRPC connections
type OTLPGrpc struct {
	logger        *zap.Logger
	host          string
	port          string
	workers       int
	dataChan      chan []byte
	ctx           context.Context
	cancel        context.CancelFunc
	workerManager *workermanager.WorkerManager
	meter         metric.Meter

	// Metrics
	otlpLogsReceived     metric.Int64Counter
	otlpActiveWorkers    metric.Int64Gauge
	otlpLogRate          metric.Float64Counter
	otlpRequestSizeBytes metric.Int64Histogram
	otlpRequestLatency   metric.Float64Histogram
	otlpSendErrors       metric.Int64Counter

	// Configuration
	batchTimeout       time.Duration
	maxQueueSize       int
	maxExportBatchSize int
}

// NewOTLPGrpc creates a new OTLP gRPC output instance using functional options
func NewOTLPGrpc(logger *zap.Logger, opts ...OTLPGrpcOption) (*OTLPGrpc, error) {
	var err error

	if logger == nil {
		return nil, fmt.Errorf("logger cannot be nil")
	}

	// Initialize config with defaults
	cfg := &OTLPGrpcConfig{
		host:               DefaultOTLPGrpcHost,
		port:               DefaultOTLPGrpcPort,
		workers:            DefaultOTLPGrpcWorkers,
		batchTimeout:       DefaultOTLPGrpcBatchTimeout,
		maxQueueSize:       DefaultOTLPGrpcMaxQueueSize,
		maxExportBatchSize: DefaultOTLPGrpcMaxExportBatchSize,
	}

	// Apply options
	for _, opt := range opts {
		if err := opt(cfg); err != nil {
			return nil, fmt.Errorf("apply option: %w", err)
		}
	}

	// Validate config
	if cfg.host == "" {
		return nil, fmt.Errorf("host cannot be empty")
	}
	if cfg.port == "" {
		return nil, fmt.Errorf("port cannot be empty")
	}
	if cfg.workers <= 0 {
		cfg.workers = DefaultOTLPGrpcWorkers
	}
	if cfg.maxQueueSize <= 0 {
		cfg.maxQueueSize = DefaultOTLPGrpcMaxQueueSize
	}
	if cfg.maxExportBatchSize <= 0 {
		cfg.maxExportBatchSize = DefaultOTLPGrpcMaxExportBatchSize
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		if err != nil {
			cancel()
		}
	}()

	meter := otel.Meter("blitz-otlp-grpc-output")

	// Initialize metrics
	otlpLogsReceived, err := meter.Int64Counter(
		"blitz.otlp_grpc.logs.received",
		metric.WithDescription("Number of logs received from the write channel"),
	)
	if err != nil {
		return nil, fmt.Errorf("create logs received counter: %w", err)
	}

	otlpActiveWorkers, err := meter.Int64Gauge(
		"blitz.otlp_grpc.workers.active",
		metric.WithDescription("Number of active worker goroutines"),
	)
	if err != nil {
		return nil, fmt.Errorf("create active workers gauge: %w", err)
	}

	otlpLogRate, err := meter.Float64Counter(
		"blitz.otlp_grpc.log.rate",
		metric.WithDescription("Rate at which logs are successfully sent to the configured host"),
	)
	if err != nil {
		return nil, fmt.Errorf("create log rate counter: %w", err)
	}

	otlpRequestSizeBytes, err := meter.Int64Histogram(
		"blitz.otlp_grpc.request.size.bytes",
		metric.WithDescription("Size of requests in bytes"),
	)
	if err != nil {
		return nil, fmt.Errorf("create request size histogram: %w", err)
	}

	otlpRequestLatency, err := meter.Float64Histogram(
		"blitz.otlp_grpc.request.latency",
		metric.WithDescription("Request latency in seconds"),
	)
	if err != nil {
		return nil, fmt.Errorf("create request latency histogram: %w", err)
	}

	otlpSendErrors, err := meter.Int64Counter(
		"blitz.otlp_grpc.send.errors",
		metric.WithDescription("Total number of send errors"),
	)
	if err != nil {
		return nil, fmt.Errorf("create send errors counter: %w", err)
	}

	otlp := &OTLPGrpc{
		logger:               logger.Named("output-otlp-grpc"),
		host:                 cfg.host,
		port:                 cfg.port,
		workers:              cfg.workers,
		dataChan:             make(chan []byte, DefaultOTLPGrpcChannelSize),
		ctx:                  ctx,
		cancel:               cancel,
		meter:                meter,
		otlpLogsReceived:     otlpLogsReceived,
		otlpActiveWorkers:    otlpActiveWorkers,
		otlpLogRate:          otlpLogRate,
		otlpRequestSizeBytes: otlpRequestSizeBytes,
		otlpRequestLatency:   otlpRequestLatency,
		otlpSendErrors:       otlpSendErrors,
		batchTimeout:         cfg.batchTimeout,
		maxQueueSize:         cfg.maxQueueSize,
		maxExportBatchSize:   cfg.maxExportBatchSize,
	}

	otlp.logger.Info("Starting OTLP gRPC output",
		zap.String("host", otlp.host),
		zap.String("port", otlp.port),
		zap.Int("workers", otlp.workers),
		zap.Int("channel_size", DefaultOTLPGrpcChannelSize),
		zap.Duration("batch_timeout", otlp.batchTimeout),
		zap.Int("max_queue_size", otlp.maxQueueSize),
		zap.Int("max_export_batch_size", otlp.maxExportBatchSize),
	)

	// Create channel size gauge
	_, err = meter.Int64ObservableGauge(
		"blitz.otlp_grpc.channel.size",
		metric.WithDescription("Current size of the data channel"),
		metric.WithInt64Callback(func(_ context.Context, io metric.Int64Observer) error {
			io.Observe(int64(len(otlp.dataChan)))
			return nil
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("create channel size gauge: %w", err)
	}

	// Create worker manager
	otlp.workerManager = workermanager.NewWorkerManager(otlp.logger, cfg.workers, otlp.otlpWorker)

	// Record initial active workers count
	otlp.otlpActiveWorkers.Record(context.Background(), int64(cfg.workers),
		metric.WithAttributeSet(
			attribute.NewSet(
				attribute.String("component", "output_otlp_grpc"),
			),
		),
	)

	// Start the workers
	otlp.workerManager.Start()

	return otlp, nil
}

// Write sends data to the OTLP gRPC output channel for processing by workers.
// Write shall not be called after Stop is called.
// If the provided context is done, Write will return immediately
// even if the data is not written to the channel.
func (o *OTLPGrpc) Write(ctx context.Context, data []byte) error {
	select {
	case o.dataChan <- data:
		// Record logs received
		o.otlpLogsReceived.Add(ctx, 1,
			metric.WithAttributeSet(
				attribute.NewSet(
					attribute.String("component", "output_otlp_grpc"),
				),
			),
		)
		return nil
	case <-ctx.Done():
		return fmt.Errorf("context cancelled while waiting to write data: %w", ctx.Err())
	case <-o.ctx.Done():
		return fmt.Errorf("OTLP gRPC output is shutting down")
	}
}

// Stop gracefully shuts down all workers and closes OTLP gRPC connections
// Stop shall not be called more than once.
// If the provided context is done, Stop will return immediately
// even if workers are still shutting down.
func (o *OTLPGrpc) Stop(ctx context.Context) error {
	o.logger.Info("Stopping OTLP gRPC output")

	// Record zero active workers
	o.otlpActiveWorkers.Record(ctx, 0,
		metric.WithAttributeSet(
			attribute.NewSet(
				attribute.String("component", "output_otlp_grpc"),
			),
		),
	)

	// Close the channel to ensure workers do not
	// process new data.
	close(o.dataChan)

	// Signal the workers to stop.
	o.cancel()

	// Stop the worker manager
	o.workerManager.Stop()

	o.logger.Info("OTLP gRPC output stopped successfully")
	return nil
}

// otlpWorker processes OTLP gRPC data from the channel and sends it to the configured host and port.
// This function is designed to work with the worker manager, which handles automatic restart
// with exponential backoff when the worker exits due to connection failures or errors.
// The worker should return immediately on any failure - the worker manager will handle
// reconnection attempts with appropriate backoff delays.
func (o *OTLPGrpc) otlpWorker(id int) {
	o.logger.Info("Starting OTLP gRPC worker", zap.Int("worker_id", id))

	conn, err := o.connect()
	if err != nil {
		o.logger.Error("Failed to establish initial OTLP gRPC connection",
			zap.Int("worker_id", id),
			zap.Error(err))
		return
	}
	defer conn.Close()

	client := collectorlogs.NewLogsServiceClient(conn)

	batch := newLogBatch(o.maxExportBatchSize, o.batchTimeout)

	for {
		select {
		case data, ok := <-o.dataChan:
			if !ok {
				o.logger.Info("OTLP gRPC worker exiting - channel closed", zap.Int("worker_id", id))
				// Flush remaining logs
				if err := o.flushBatch(client, batch); err != nil {
					o.logger.Error("Failed to flush final batch", zap.Int("worker_id", id), zap.Error(err))
				}
				return
			}

			// Add to batch
			batch.add(data)

			// Send batch if it's full
			if batch.isFull() {
				if !batch.timer.Stop() {
					select {
					case <-batch.timer.C:
					default:
					}
				}
				if err := o.sendBatch(client, batch); err != nil {
					o.logger.Error("Failed to send OTLP gRPC batch",
						zap.Int("worker_id", id),
						zap.Error(err))
					return
				}
				batch = newLogBatch(o.maxExportBatchSize, o.batchTimeout)
			}

		case <-batch.timer.C:
			// Batch timeout reached, send batch
			if !batch.isEmpty() {
				if err := o.sendBatch(client, batch); err != nil {
					o.logger.Error("Failed to send OTLP gRPC batch",
						zap.Int("worker_id", id),
						zap.Error(err))
					return
				}
			}
			// Create new batch with new timer
			batch = newLogBatch(o.maxExportBatchSize, o.batchTimeout)

		case <-o.ctx.Done():
			o.logger.Info("OTLP gRPC worker exiting - context cancelled", zap.Int("worker_id", id))
			// Flush remaining logs
			if err := o.flushBatch(client, batch); err != nil {
				o.logger.Error("Failed to flush final batch", zap.Int("worker_id", id), zap.Error(err))
			}
			return
		}
	}
}

// connect establishes a gRPC connection to the configured host and port
func (o *OTLPGrpc) connect() (*grpc.ClientConn, error) {
	endpoint := fmt.Sprintf("%s:%s", o.host, o.port)

	conn, err := grpc.NewClient(
		endpoint,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create gRPC client for %s: %w", endpoint, err)
	}

	return conn, nil
}

// logBatch holds a batch of logs to be sent
type logBatch struct {
	logs    []jsonLog
	maxSize int
	timer   *time.Timer
	mu      sync.Mutex
}

// newLogBatch creates a new log batch
func newLogBatch(maxSize int, timeout time.Duration) *logBatch {
	return &logBatch{
		logs:    make([]jsonLog, 0, maxSize),
		maxSize: maxSize,
		timer:   time.NewTimer(timeout),
	}
}

// add adds a log to the batch
func (b *logBatch) add(data []byte) {
	b.mu.Lock()
	defer b.mu.Unlock()

	var jsonLog jsonLog
	if err := json.Unmarshal(data, &jsonLog); err != nil {
		// Skip invalid JSON
		return
	}
	b.logs = append(b.logs, jsonLog)
}

// isFull returns true if the batch is full
func (b *logBatch) isFull() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.logs) >= b.maxSize
}

// isEmpty returns true if the batch is empty
func (b *logBatch) isEmpty() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.logs) == 0
}

// getAndClear returns all logs and clears the batch
func (b *logBatch) getAndClear() []jsonLog {
	b.mu.Lock()
	defer b.mu.Unlock()
	logs := b.logs
	b.logs = make([]jsonLog, 0, b.maxSize)
	return logs
}

// sendBatch sends a batch of logs via OTLP gRPC
func (o *OTLPGrpc) sendBatch(client collectorlogs.LogsServiceClient, batch *logBatch) error {
	startTime := time.Now()

	logs := batch.getAndClear()
	if len(logs) == 0 {
		return nil
	}

	// Build OTLP request
	request := o.buildOTLPRequest(logs)

	// Send request
	ctx, cancel := context.WithTimeout(context.Background(), o.batchTimeout)
	defer cancel()

	ctx = metadata.NewOutgoingContext(ctx, metadata.New(map[string]string{}))

	_, err := client.Export(ctx, request)
	if err != nil {
		o.recordSendError("export_error", err)
		return fmt.Errorf("failed to export logs: %w", err)
	}

	// Record successful send metrics
	latency := time.Since(startTime).Seconds()
	requestSize := int64(proto.Size(request))
	o.otlpLogRate.Add(context.Background(), float64(len(logs)),
		metric.WithAttributeSet(
			attribute.NewSet(
				attribute.String("component", "output_otlp_grpc"),
			),
		),
	)
	o.otlpRequestSizeBytes.Record(context.Background(), requestSize,
		metric.WithAttributeSet(
			attribute.NewSet(
				attribute.String("component", "output_otlp_grpc"),
			),
		),
	)
	o.otlpRequestLatency.Record(context.Background(), latency,
		metric.WithAttributeSet(
			attribute.NewSet(
				attribute.String("component", "output_otlp_grpc"),
			),
		),
	)

	return nil
}

// flushBatch flushes any remaining logs in the batch
func (o *OTLPGrpc) flushBatch(client collectorlogs.LogsServiceClient, batch *logBatch) error {
	if !batch.timer.Stop() {
		select {
		case <-batch.timer.C:
		default:
		}
	}
	if batch.isEmpty() {
		return nil
	}
	return o.sendBatch(client, batch)
}

// buildOTLPRequest builds an OTLP ExportLogsServiceRequest from JSON logs
func (o *OTLPGrpc) buildOTLPRequest(logs []jsonLog) *collectorlogs.ExportLogsServiceRequest {
	resourceLogs := &logspb.ResourceLogs{
		Resource: &resourcepb.Resource{
			Attributes: []*commonpb.KeyValue{
				{
					Key: "service.name",
					Value: &commonpb.AnyValue{
						Value: &commonpb.AnyValue_StringValue{
							StringValue: "blitz",
						},
					},
				},
			},
		},
		ScopeLogs: []*logspb.ScopeLogs{
			{
				LogRecords: make([]*logspb.LogRecord, 0, len(logs)),
			},
		},
	}

	for _, jsonLog := range logs {
		logRecord := &logspb.LogRecord{
			TimeUnixNano:         timeToUnixNanoUint64(jsonLog.Timestamp),
			ObservedTimeUnixNano: timeToUnixNanoUint64(time.Now()),
			SeverityNumber:       o.mapSeverityNumber(jsonLog.Level),
			SeverityText:         jsonLog.Level,
			Body: &commonpb.AnyValue{
				Value: &commonpb.AnyValue_StringValue{
					StringValue: jsonLog.Message,
				},
			},
			Attributes: []*commonpb.KeyValue{
				{
					Key: "environment",
					Value: &commonpb.AnyValue{
						Value: &commonpb.AnyValue_StringValue{
							StringValue: jsonLog.Environment,
						},
					},
				},
				{
					Key: "location",
					Value: &commonpb.AnyValue{
						Value: &commonpb.AnyValue_StringValue{
							StringValue: jsonLog.Location,
						},
					},
				},
			},
		}
		resourceLogs.ScopeLogs[0].LogRecords = append(resourceLogs.ScopeLogs[0].LogRecords, logRecord)
	}

	return &collectorlogs.ExportLogsServiceRequest{
		ResourceLogs: []*logspb.ResourceLogs{resourceLogs},
	}
}

// mapSeverityNumber maps string log levels to OTLP severity numbers
func (o *OTLPGrpc) mapSeverityNumber(level string) logspb.SeverityNumber {
	switch level {
	case "DEBUG":
		return logspb.SeverityNumber_SEVERITY_NUMBER_DEBUG
	case "INFO":
		return logspb.SeverityNumber_SEVERITY_NUMBER_INFO
	case "WARN":
		return logspb.SeverityNumber_SEVERITY_NUMBER_WARN
	case "ERROR":
		return logspb.SeverityNumber_SEVERITY_NUMBER_ERROR
	case "FATAL":
		return logspb.SeverityNumber_SEVERITY_NUMBER_FATAL2
	default:
		return logspb.SeverityNumber_SEVERITY_NUMBER_INFO
	}
}

// recordSendError records metrics for send errors
func (o *OTLPGrpc) recordSendError(errorType string, err error) {
	ctx := context.Background()

	o.otlpSendErrors.Add(ctx, 1,
		metric.WithAttributeSet(
			attribute.NewSet(
				attribute.String("component", "output_otlp_grpc"),
				attribute.String("error_type", errorType),
			),
		),
	)
}
