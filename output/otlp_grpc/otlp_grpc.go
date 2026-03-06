package otlpgrpc

import (
	"context"
	"crypto/tls"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/observiq/blitz/internal/workermanager"
	"github.com/observiq/blitz/output"
	"github.com/observiq/blitz/telemetry"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	collectorlogs "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	collectormetrics "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	logspb "go.opentelemetry.io/proto/otlp/logs/v1"
	metricspb "go.opentelemetry.io/proto/otlp/metrics/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
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
	insecure           bool
	tlsConfig          *tls.Config
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

// WithInsecure sets whether to use insecure credentials (no TLS)
func WithInsecure(insecure bool) OTLPGrpcOption {
	return func(cfg *OTLPGrpcConfig) error {
		cfg.insecure = insecure
		return nil
	}
}

// WithTLSConfig sets the TLS configuration for secure connections
func WithTLSConfig(tlsConfig *tls.Config) OTLPGrpcOption {
	return func(cfg *OTLPGrpcConfig) error {
		cfg.tlsConfig = tlsConfig
		return nil
	}
}

// OTLPGrpc implements the Output interface for OTLP gRPC connections
type OTLPGrpc struct {
	logger    *zap.Logger
	host      string
	port      string
	workers   int
	insecure  bool
	tlsConfig *tls.Config
	ctx       context.Context
	cancel    context.CancelFunc
	meter     metric.Meter

	// Log pipeline
	logDataChan      chan *logspb.LogRecord
	logWorkerManager *workermanager.WorkerManager

	// Metric pipeline
	metricDataChan      chan *metricspb.Metric
	metricWorkerManager *workermanager.WorkerManager

	// Observability metrics
	otlpLogsReceived      metric.Int64Counter
	otlpMetricsReceived   metric.Int64Counter
	otlpActiveWorkers     metric.Int64Gauge
	otlpLogRate           metric.Float64Counter
	otlpMetricRate        metric.Float64Counter
	otlpRequestSizeBytes  metric.Int64Histogram
	otlpRequestLatency    metric.Float64Histogram
	otlpSendErrors        metric.Int64Counter

	// Resource attributes from metric records, cached on first WriteMetric call.
	cachedResourceAttrs []*commonpb.KeyValue

	// Configuration
	batchTimeout       time.Duration
	maxQueueSize       int
	maxExportBatchSize int
}

// New creates a new OTLP gRPC output instance using functional options
func New(logger *zap.Logger, opts ...OTLPGrpcOption) (*OTLPGrpc, error) {
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
		insecure:           true,
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

	// Initialize observability metrics
	otlpLogsReceived, err := meter.Int64Counter(
		"blitz.otlp_grpc.logs.received",
		metric.WithDescription("Number of logs received from the write channel"),
	)
	if err != nil {
		return nil, fmt.Errorf("create logs received counter: %w", err)
	}

	otlpMetricsReceived, err := meter.Int64Counter(
		"blitz.otlp_grpc.metrics.received",
		metric.WithDescription("Number of metrics received from the write channel"),
	)
	if err != nil {
		return nil, fmt.Errorf("create metrics received counter: %w", err)
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

	otlpMetricRate, err := meter.Float64Counter(
		"blitz.otlp_grpc.metric.rate",
		metric.WithDescription("Rate at which metrics are successfully sent to the configured host"),
	)
	if err != nil {
		return nil, fmt.Errorf("create metric rate counter: %w", err)
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
		logger:              logger.Named("output-otlp-grpc"),
		host:                cfg.host,
		port:                cfg.port,
		workers:             cfg.workers,
		insecure:            cfg.insecure,
		tlsConfig:           cfg.tlsConfig,
		logDataChan:         make(chan *logspb.LogRecord, DefaultOTLPGrpcChannelSize),
		metricDataChan:      make(chan *metricspb.Metric, DefaultOTLPGrpcChannelSize),
		ctx:                 ctx,
		cancel:              cancel,
		meter:               meter,
		otlpLogsReceived:    otlpLogsReceived,
		otlpMetricsReceived: otlpMetricsReceived,
		otlpActiveWorkers:   otlpActiveWorkers,
		otlpLogRate:         otlpLogRate,
		otlpMetricRate:      otlpMetricRate,
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
		zap.Bool("insecure", cfg.insecure),
		zap.Bool("tls_enabled", cfg.tlsConfig != nil),
	)

	// Create channel size gauges
	_, err = meter.Int64ObservableGauge(
		"blitz.otlp_grpc.log_channel.size",
		metric.WithDescription("Current size of the log data channel"),
		metric.WithInt64Callback(func(_ context.Context, io metric.Int64Observer) error {
			io.Observe(int64(len(otlp.logDataChan)))
			return nil
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("create log channel size gauge: %w", err)
	}

	_, err = meter.Int64ObservableGauge(
		"blitz.otlp_grpc.metric_channel.size",
		metric.WithDescription("Current size of the metric data channel"),
		metric.WithInt64Callback(func(_ context.Context, io metric.Int64Observer) error {
			io.Observe(int64(len(otlp.metricDataChan)))
			return nil
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("create metric channel size gauge: %w", err)
	}

	// Create worker managers for logs and metrics
	otlp.logWorkerManager = workermanager.NewWorkerManager(otlp.logger, cfg.workers, otlp.logWorker)
	otlp.metricWorkerManager = workermanager.NewWorkerManager(otlp.logger, cfg.workers, otlp.metricWorker)

	// Record initial active workers count (logs + metrics workers)
	otlp.otlpActiveWorkers.Record(context.Background(), int64(cfg.workers*2),
		metric.WithAttributeSet(
			attribute.NewSet(
				attribute.String("component", "output_otlp_grpc"),
			),
		),
	)

	// Start the workers
	otlp.logWorkerManager.Start()
	otlp.metricWorkerManager.Start()

	return otlp, nil
}

// Write sends data to the OTLP gRPC output channel for processing by workers.
// Write shall not be called after Stop is called.
// If the provided context is done, Write will return immediately
// even if the data is not written to the channel.
func (o *OTLPGrpc) Write(ctx context.Context, data output.LogRecord) error {
	// Build OTLP log record before batching
	timestamp := time.Now()
	severityText := "INFO"
	severityNumber := logspb.SeverityNumber_SEVERITY_NUMBER_INFO
	environment := ""
	location := ""

	var body *commonpb.AnyValue
	if data.ParseFunc != nil {
		parsed, err := data.ParseFunc(data.Message)
		if err != nil {
			o.logger.Warn("ParseFunc error; using raw message", zap.Error(err))
			body = &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: data.Message}}
		} else {
			body = o.convertMapToAnyValue(parsed)
			if body == nil {
				o.logger.Warn("ParseFunc returned unsupported map; using raw message")
				body = &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: data.Message}}
			}
		}

	} else {
		body = &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: data.Message}}
	}

	if !data.Metadata.Timestamp.IsZero() {
		timestamp = data.Metadata.Timestamp
	}

	if data.Metadata.Severity != "" {
		severityText = data.Metadata.Severity
	}

	if severityText != "" {
		severityNumber = o.mapSeverityNumber(severityText)
	}

	record := &logspb.LogRecord{
		TimeUnixNano:         output.TimeToUnixNanoUint64(timestamp),
		ObservedTimeUnixNano: output.TimeToUnixNanoUint64(time.Now()),
		SeverityNumber:       severityNumber,
		SeverityText:         severityText,
		Body:                 body,
		Attributes: []*commonpb.KeyValue{
			{
				Key:   "environment",
				Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: environment}},
			},
			{
				Key:   "location",
				Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: location}},
			},
		},
	}

	select {
	case o.logDataChan <- record:
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

// WriteMetric sends metric data to the OTLP gRPC output channel for processing by metric workers.
// WriteMetric shall not be called after Stop is called.
func (o *OTLPGrpc) WriteMetric(ctx context.Context, data output.MetricRecord) error {
	// Cache resource attributes from the first record that carries them.
	// All records in a pipeline share the same resource, so this is safe.
	if o.cachedResourceAttrs == nil && len(data.ResourceAttributes) > 0 {
		attrs := make([]*commonpb.KeyValue, 0, len(data.ResourceAttributes))
		for k, v := range data.ResourceAttributes {
			attrs = append(attrs, &commonpb.KeyValue{
				Key:   k,
				Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: v}},
			})
		}
		o.cachedResourceAttrs = attrs
	}

	m := o.buildOTLPMetric(data)

	select {
	case o.metricDataChan <- m:
		o.otlpMetricsReceived.Add(ctx, 1,
			metric.WithAttributeSet(
				attribute.NewSet(
					attribute.String("component", "output_otlp_grpc"),
				),
			),
		)
		return nil
	case <-ctx.Done():
		return fmt.Errorf("context cancelled while waiting to write metric: %w", ctx.Err())
	case <-o.ctx.Done():
		return fmt.Errorf("OTLP gRPC output is shutting down")
	}
}

// buildOTLPMetric converts an output.MetricRecord to an OTLP Metric protobuf.
func (o *OTLPGrpc) buildOTLPMetric(data output.MetricRecord) *metricspb.Metric {
	timestamp := data.Timestamp
	if timestamp.IsZero() {
		timestamp = time.Now()
	}
	timeNano := output.TimeToUnixNanoUint64(timestamp)

	// Build attributes
	attrs := make([]*commonpb.KeyValue, 0, len(data.Attributes))
	for k, v := range data.Attributes {
		attrs = append(attrs, &commonpb.KeyValue{
			Key:   k,
			Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: v}},
		})
	}

	// Build the data point value
	dp := &metricspb.NumberDataPoint{
		TimeUnixNano: timeNano,
		Attributes:   attrs,
	}
	if data.DoubleValue != nil {
		dp.Value = &metricspb.NumberDataPoint_AsDouble{AsDouble: *data.DoubleValue}
	} else if data.IntValue != nil {
		dp.Value = &metricspb.NumberDataPoint_AsInt{AsInt: *data.IntValue}
	}

	m := &metricspb.Metric{
		Name:        data.Name,
		Description: data.Description,
		Unit:        data.Unit,
	}

	switch data.Type {
	case output.MetricTypeSum:
		m.Data = &metricspb.Metric_Sum{
			Sum: &metricspb.Sum{
				AggregationTemporality: metricspb.AggregationTemporality_AGGREGATION_TEMPORALITY_CUMULATIVE,
				IsMonotonic:           true,
				DataPoints:            []*metricspb.NumberDataPoint{dp},
			},
		}
	default:
		// Default to gauge
		m.Data = &metricspb.Metric_Gauge{
			Gauge: &metricspb.Gauge{
				DataPoints: []*metricspb.NumberDataPoint{dp},
			},
		}
	}

	return m
}

// SupportedTelemetry returns the telemetry types this output supports.
func (o *OTLPGrpc) SupportedTelemetry() []telemetry.Type {
	return []telemetry.Type{telemetry.Logs, telemetry.Metrics}
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

	// Close the channels to ensure workers do not
	// process new data.
	close(o.logDataChan)
	close(o.metricDataChan)

	// Signal the workers to stop.
	o.cancel()

	// Stop the worker managers
	o.logWorkerManager.Stop()
	o.metricWorkerManager.Stop()

	o.logger.Info("OTLP gRPC output stopped successfully")
	return nil
}

// logWorker processes OTLP gRPC log data from the channel and sends it to the configured host and port.
// This function is designed to work with the worker manager, which handles automatic restart
// with exponential backoff when the worker exits due to connection failures or errors.
// The worker should return immediately on any failure - the worker manager will handle
// reconnection attempts with appropriate backoff delays.
func (o *OTLPGrpc) logWorker(id int) {
	o.logger.Info("Starting OTLP gRPC log worker", zap.Int("worker_id", id))

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
		case rec, ok := <-o.logDataChan:
			if !ok {
				o.logger.Info("OTLP gRPC worker exiting - channel closed", zap.Int("worker_id", id))
				// Flush remaining logs
				if err := o.flushBatch(client, batch); err != nil {
					o.logger.Error("Failed to flush final batch", zap.Int("worker_id", id), zap.Error(err))
				}
				return
			}

			// Add to batch
			batch.add(rec)

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

// metricWorker processes OTLP gRPC metric data from the channel and sends it to the configured host and port.
func (o *OTLPGrpc) metricWorker(id int) {
	o.logger.Info("Starting OTLP gRPC metric worker", zap.Int("worker_id", id))

	conn, err := o.connect()
	if err != nil {
		o.logger.Error("Failed to establish initial OTLP gRPC connection for metrics",
			zap.Int("worker_id", id),
			zap.Error(err))
		return
	}
	defer conn.Close()

	client := collectormetrics.NewMetricsServiceClient(conn)

	batch := newMetricBatch(o.maxExportBatchSize, o.batchTimeout)

	for {
		select {
		case rec, ok := <-o.metricDataChan:
			if !ok {
				o.logger.Info("OTLP gRPC metric worker exiting - channel closed", zap.Int("worker_id", id))
				if err := o.flushMetricBatch(client, batch); err != nil {
					o.logger.Error("Failed to flush final metric batch", zap.Int("worker_id", id), zap.Error(err))
				}
				return
			}

			batch.add(rec)

			if batch.isFull() {
				if !batch.timer.Stop() {
					select {
					case <-batch.timer.C:
					default:
					}
				}
				if err := o.sendMetricBatch(client, batch); err != nil {
					o.logger.Error("Failed to send OTLP gRPC metric batch",
						zap.Int("worker_id", id),
						zap.Error(err))
					return
				}
				batch = newMetricBatch(o.maxExportBatchSize, o.batchTimeout)
			}

		case <-batch.timer.C:
			if !batch.isEmpty() {
				if err := o.sendMetricBatch(client, batch); err != nil {
					o.logger.Error("Failed to send OTLP gRPC metric batch",
						zap.Int("worker_id", id),
						zap.Error(err))
					return
				}
			}
			batch = newMetricBatch(o.maxExportBatchSize, o.batchTimeout)

		case <-o.ctx.Done():
			o.logger.Info("OTLP gRPC metric worker exiting - context cancelled", zap.Int("worker_id", id))
			if err := o.flushMetricBatch(client, batch); err != nil {
				o.logger.Error("Failed to flush final metric batch", zap.Int("worker_id", id), zap.Error(err))
			}
			return
		}
	}
}

// connect establishes a gRPC connection to the configured host and port
func (o *OTLPGrpc) connect() (*grpc.ClientConn, error) {
	endpoint := fmt.Sprintf("%s:%s", o.host, o.port)

	var opts []grpc.DialOption

	// Configure transport credentials based on insecure flag and TLS config
	if o.insecure || o.tlsConfig == nil {
		// Use insecure credentials (no TLS)
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	} else {
		// Use TLS credentials
		tlsCreds := credentials.NewTLS(o.tlsConfig)
		opts = append(opts, grpc.WithTransportCredentials(tlsCreds))
	}

	conn, err := grpc.NewClient(endpoint, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create gRPC client for %s: %w", endpoint, err)
	}

	return conn, nil
}

// logBatch holds a batch of logs to be sent
type logBatch struct {
	logs    []*logspb.LogRecord
	maxSize int
	timer   *time.Timer
	mu      sync.Mutex
}

// newLogBatch creates a new log batch
func newLogBatch(maxSize int, timeout time.Duration) *logBatch {
	return &logBatch{
		logs:    make([]*logspb.LogRecord, 0, maxSize),
		maxSize: maxSize,
		timer:   time.NewTimer(timeout),
	}
}

// add adds a log to the batch
func (b *logBatch) add(data *logspb.LogRecord) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.logs = append(b.logs, data)
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
func (b *logBatch) getAndClear() []*logspb.LogRecord {
	b.mu.Lock()
	defer b.mu.Unlock()
	logs := b.logs
	b.logs = make([]*logspb.LogRecord, 0, b.maxSize)
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

// metricBatch holds a batch of metrics to be sent
type metricBatch struct {
	metrics []*metricspb.Metric
	maxSize int
	timer   *time.Timer
	mu      sync.Mutex
}

// newMetricBatch creates a new metric batch
func newMetricBatch(maxSize int, timeout time.Duration) *metricBatch {
	return &metricBatch{
		metrics: make([]*metricspb.Metric, 0, maxSize),
		maxSize: maxSize,
		timer:   time.NewTimer(timeout),
	}
}

func (b *metricBatch) add(data *metricspb.Metric) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.metrics = append(b.metrics, data)
}

func (b *metricBatch) isFull() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.metrics) >= b.maxSize
}

func (b *metricBatch) isEmpty() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.metrics) == 0
}

func (b *metricBatch) getAndClear() []*metricspb.Metric {
	b.mu.Lock()
	defer b.mu.Unlock()
	metrics := b.metrics
	b.metrics = make([]*metricspb.Metric, 0, b.maxSize)
	return metrics
}

// sendMetricBatch sends a batch of metrics via OTLP gRPC
func (o *OTLPGrpc) sendMetricBatch(client collectormetrics.MetricsServiceClient, batch *metricBatch) error {
	startTime := time.Now()

	metrics := batch.getAndClear()
	if len(metrics) == 0 {
		return nil
	}

	request := o.buildOTLPMetricsRequest(metrics)

	ctx, cancel := context.WithTimeout(context.Background(), o.batchTimeout)
	defer cancel()

	ctx = metadata.NewOutgoingContext(ctx, metadata.New(map[string]string{}))

	_, err := client.Export(ctx, request)
	if err != nil {
		o.recordSendError("metrics_export_error", err)
		return fmt.Errorf("failed to export metrics: %w", err)
	}

	latency := time.Since(startTime).Seconds()
	requestSize := int64(proto.Size(request))
	o.otlpMetricRate.Add(context.Background(), float64(len(metrics)),
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
				attribute.String("signal", "metrics"),
			),
		),
	)
	o.otlpRequestLatency.Record(context.Background(), latency,
		metric.WithAttributeSet(
			attribute.NewSet(
				attribute.String("component", "output_otlp_grpc"),
				attribute.String("signal", "metrics"),
			),
		),
	)

	return nil
}

// flushMetricBatch flushes any remaining metrics in the batch
func (o *OTLPGrpc) flushMetricBatch(client collectormetrics.MetricsServiceClient, batch *metricBatch) error {
	if !batch.timer.Stop() {
		select {
		case <-batch.timer.C:
		default:
		}
	}
	if batch.isEmpty() {
		return nil
	}
	return o.sendMetricBatch(client, batch)
}

// buildOTLPMetricsRequest builds an OTLP ExportMetricsServiceRequest.
// Resource attributes are taken from the batch. If no metrics carry resource
// attributes, a default service.name=blitz is used.
func (o *OTLPGrpc) buildOTLPMetricsRequest(metrics []*metricspb.Metric) *collectormetrics.ExportMetricsServiceRequest {
	scopeMetrics := &metricspb.ScopeMetrics{
		Metrics: make([]*metricspb.Metric, 0, len(metrics)),
	}
	for _, m := range metrics {
		if m == nil {
			continue
		}
		scopeMetrics.Metrics = append(scopeMetrics.Metrics, m)
	}

	// Use cached resource attributes if available, otherwise default.
	resourceAttrs := o.cachedResourceAttrs
	if len(resourceAttrs) == 0 {
		resourceAttrs = []*commonpb.KeyValue{
			{
				Key: "service.name",
				Value: &commonpb.AnyValue{
					Value: &commonpb.AnyValue_StringValue{
						StringValue: "blitz",
					},
				},
			},
		}
	}

	return &collectormetrics.ExportMetricsServiceRequest{
		ResourceMetrics: []*metricspb.ResourceMetrics{
			{
				Resource: &resourcepb.Resource{
					Attributes: resourceAttrs,
				},
				ScopeMetrics: []*metricspb.ScopeMetrics{scopeMetrics},
			},
		},
	}
}

// buildOTLPRequest builds an OTLP ExportLogsServiceRequest from prepared LogRecord entries
func (o *OTLPGrpc) buildOTLPRequest(logs []*logspb.LogRecord) *collectorlogs.ExportLogsServiceRequest {
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

	for _, logRecord := range logs {
		if logRecord == nil {
			continue
		}
		resourceLogs.ScopeLogs[0].LogRecords = append(resourceLogs.ScopeLogs[0].LogRecords, logRecord)
	}

	return &collectorlogs.ExportLogsServiceRequest{
		ResourceLogs: []*logspb.ResourceLogs{resourceLogs},
	}
}

// convertMapToAnyValue converts map[string]any to OTLP AnyValue (kvlist)
func (o *OTLPGrpc) convertMapToAnyValue(m map[string]any) *commonpb.AnyValue {
	kvs := make([]*commonpb.KeyValue, 0, len(m))
	for k, v := range m {
		av := o.toAnyValue(v)
		if av == nil {
			o.logger.Warn("Unsupported value in map; dropping key", zap.String("key", k), zap.String("type", fmt.Sprintf("%T", v)))
			continue
		}
		kvs = append(kvs, &commonpb.KeyValue{Key: k, Value: av})
	}
	return &commonpb.AnyValue{
		Value: &commonpb.AnyValue_KvlistValue{
			KvlistValue: &commonpb.KeyValueList{Values: kvs},
		},
	}
}

// toAnyValue converts common Go types to OTLP AnyValue; returns nil for unsupported
func (o *OTLPGrpc) toAnyValue(v any) *commonpb.AnyValue {
	switch x := v.(type) {
	case nil:
		return nil
	case string:
		return &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: x}}
	case bool:
		return &commonpb.AnyValue{Value: &commonpb.AnyValue_BoolValue{BoolValue: x}}
	case int:
		return &commonpb.AnyValue{Value: &commonpb.AnyValue_IntValue{IntValue: int64(x)}}
	case int32:
		return &commonpb.AnyValue{Value: &commonpb.AnyValue_IntValue{IntValue: int64(x)}}
	case int64:
		return &commonpb.AnyValue{Value: &commonpb.AnyValue_IntValue{IntValue: x}}
	case uint:
		ux := uint64(x)
		if ux > uint64(math.MaxInt64) {
			return &commonpb.AnyValue{Value: &commonpb.AnyValue_DoubleValue{DoubleValue: float64(ux)}}
		}
		return &commonpb.AnyValue{Value: &commonpb.AnyValue_IntValue{IntValue: int64(ux)}}
	case uint32:
		return &commonpb.AnyValue{Value: &commonpb.AnyValue_IntValue{IntValue: int64(x)}}
	case uint64:
		if x > uint64(math.MaxInt64) {
			return &commonpb.AnyValue{Value: &commonpb.AnyValue_DoubleValue{DoubleValue: float64(x)}}
		}
		return &commonpb.AnyValue{Value: &commonpb.AnyValue_IntValue{IntValue: int64(x)}}
	case float32:
		return &commonpb.AnyValue{Value: &commonpb.AnyValue_DoubleValue{DoubleValue: float64(x)}}
	case float64:
		return &commonpb.AnyValue{Value: &commonpb.AnyValue_DoubleValue{DoubleValue: x}}
	case []any:
		values := make([]*commonpb.AnyValue, 0, len(x))
		for _, e := range x {
			av := o.toAnyValue(e)
			if av == nil {
				o.logger.Warn("Unsupported value in array; dropping element", zap.String("type", fmt.Sprintf("%T", e)))
				continue
			}
			values = append(values, av)
		}
		return &commonpb.AnyValue{Value: &commonpb.AnyValue_ArrayValue{ArrayValue: &commonpb.ArrayValue{Values: values}}}
	case []string:
		values := make([]*commonpb.AnyValue, 0, len(x))
		for _, s := range x {
			values = append(values, &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: s}})
		}
		return &commonpb.AnyValue{Value: &commonpb.AnyValue_ArrayValue{ArrayValue: &commonpb.ArrayValue{Values: values}}}
	case map[string]any:
		return o.convertMapToAnyValue(x)
	default:
		return nil
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
