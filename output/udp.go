package output

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/observiq/blitz/internal/workermanager"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"
)

const (
	// DefaultUDPChannelSize is the default size of the data channel
	DefaultUDPChannelSize = 100

	// DefaultUDPWorkers is the default number of worker goroutines
	DefaultUDPWorkers = 1

	// DefaultUDPWriteTimeout is the default timeout for writing data to UDP connections
	DefaultUDPWriteTimeout = 5 * time.Second

	// DefaultUDPStopTimeout is the default timeout for graceful shutdown
	DefaultUDPStopTimeout = 30 * time.Second
)

// UDP implements the Output interface for UDP connections
type UDP struct {
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
	udpLogsReceived     metric.Int64Counter
	udpActiveWorkers    metric.Int64Gauge
	udpLogRate          metric.Float64Counter
	udpRequestSizeBytes metric.Int64Histogram
	udpSendErrors       metric.Int64Counter
}

// NewUDP creates a new UDP output instance
func NewUDP(logger *zap.Logger, host, port string, workers int) (*UDP, error) {
	var err error

	if logger == nil {
		return nil, fmt.Errorf("logger cannot be nil")
	}
	if host == "" {
		return nil, fmt.Errorf("host cannot be empty")
	}
	if port == "" {
		return nil, fmt.Errorf("port cannot be empty")
	}
	if workers <= 0 {
		workers = DefaultUDPWorkers
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		if err != nil {
			cancel()
		}
	}()

	meter := otel.Meter("bindplane-loader-udp-output")

	// Initialize metrics
	udpLogsReceived, err := meter.Int64Counter(
		"bindplane-loader.udp.logs.received",
		metric.WithDescription("Number of logs received from the write channel"),
	)
	if err != nil {
		return nil, fmt.Errorf("create logs received counter: %w", err)
	}

	udpActiveWorkers, err := meter.Int64Gauge(
		"bindplane-loader.udp.workers.active",
		metric.WithDescription("Number of active worker goroutines"),
	)
	if err != nil {
		return nil, fmt.Errorf("create active workers gauge: %w", err)
	}

	udpLogRate, err := meter.Float64Counter(
		"bindplane-loader.udp.log.rate",
		metric.WithDescription("Rate at which logs are successfully sent to the configured host"),
	)
	if err != nil {
		return nil, fmt.Errorf("create log rate counter: %w", err)
	}

	udpRequestSizeBytes, err := meter.Int64Histogram(
		"bindplane-loader.udp.request.size.bytes",
		metric.WithDescription("Size of requests in bytes"),
	)
	if err != nil {
		return nil, fmt.Errorf("create request size histogram: %w", err)
	}

	udpSendErrors, err := meter.Int64Counter(
		"bindplane-loader.udp.send.errors",
		metric.WithDescription("Total number of send errors"),
	)
	if err != nil {
		return nil, fmt.Errorf("create send errors counter: %w", err)
	}

	udp := &UDP{
		logger:              logger.Named("output-udp"),
		host:                host,
		port:                port,
		workers:             workers,
		dataChan:            make(chan []byte, DefaultUDPChannelSize),
		ctx:                 ctx,
		cancel:              cancel,
		meter:               meter,
		udpLogsReceived:     udpLogsReceived,
		udpActiveWorkers:    udpActiveWorkers,
		udpLogRate:          udpLogRate,
		udpRequestSizeBytes: udpRequestSizeBytes,
		udpSendErrors:       udpSendErrors,
	}

	udp.logger.Info("Starting UDP output",
		zap.String("host", udp.host),
		zap.String("port", udp.port),
		zap.Int("workers", udp.workers),
		zap.Int("channel_size", DefaultUDPChannelSize),
	)

	// Create worker manager
	udp.workerManager = workermanager.NewWorkerManager(udp.logger, workers, udp.udpWorker)

	// Record initial active workers count
	udp.udpActiveWorkers.Record(context.Background(), int64(workers),
		metric.WithAttributeSet(
			attribute.NewSet(
				attribute.String("component", "output_udp"),
			),
		),
	)

	// Start the workers
	udp.workerManager.Start()

	return udp, nil
}

// Write sends data to the UDP output channel for processing by workers.
// Write shall not be called after Stop is called.
// If the provided context is done, Write will return immediately
// even if the data is not written to the channel.
func (u *UDP) Write(ctx context.Context, data []byte) error {
	select {
	case u.dataChan <- data:
		// Record logs received
		u.udpLogsReceived.Add(ctx, 1,
			metric.WithAttributeSet(
				attribute.NewSet(
					attribute.String("component", "output_udp"),
				),
			),
		)
		return nil
	case <-ctx.Done():
		return fmt.Errorf("context cancelled while waiting to write data: %w", ctx.Err())
	case <-u.ctx.Done():
		return fmt.Errorf("UDP output is shutting down")
	}
}

// Stop gracefully shuts down all workers and closes UDP connections
// Stop shall not be called more than once.
// If the provided context is done, Stop will return immediately
// even if workers are still shutting down.
func (u *UDP) Stop(ctx context.Context) error {
	u.logger.Info("Stopping UDP output")

	// Record zero active workers
	u.udpActiveWorkers.Record(ctx, 0,
		metric.WithAttributeSet(
			attribute.NewSet(
				attribute.String("component", "output_udp"),
			),
		),
	)

	// Close the channel to ensure workers do not
	// process new data.
	close(u.dataChan)

	// Signal the workers to stop.
	u.cancel()

	// Stop the worker manager
	u.workerManager.Stop()

	u.logger.Info("UDP output stopped successfully")
	return nil
}

// udpWorker processes UDP data from the channel and sends it to the configured host and port.
// This function is designed to work with the worker manager, which handles automatic restart
// with exponential backoff when the worker exits due to connection failures or errors.
// The worker should return immediately on any failure - the worker manager will handle
// reconnection attempts with appropriate backoff delays.
func (u *UDP) udpWorker(id int) {
	u.logger.Info("Starting UDP worker", zap.Int("worker_id", id))

	conn, err := u.connect()
	if err != nil {
		u.logger.Error("Failed to establish initial UDP connection",
			zap.Int("worker_id", id),
			zap.Error(err))
		return
	}
	defer conn.Close()

	for {
		select {
		case data, ok := <-u.dataChan:
			if !ok {
				u.logger.Info("UDP worker exiting - channel closed", zap.Int("worker_id", id))
				return
			}

			if err := u.sendData(conn, data); err != nil {
				u.logger.Error("Failed to send UDP data",
					zap.Int("worker_id", id),
					zap.Error(err))
				return
			}

		case <-u.ctx.Done():
			u.logger.Info("UDP worker exiting - context cancelled", zap.Int("worker_id", id))
			return
		}
	}
}

// connect establishes a UDP connection to the configured host and port
func (u *UDP) connect() (net.Conn, error) {
	address := net.JoinHostPort(u.host, u.port)

	conn, err := net.Dial("udp", address)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %w", address, err)
	}

	return conn, nil
}

// sendData sends data to the UDP connection with a timeout
func (u *UDP) sendData(conn net.Conn, data []byte) error {
	// Set write timeout
	if err := conn.SetWriteDeadline(time.Now().Add(DefaultUDPWriteTimeout)); err != nil {
		u.recordSendError("unknown", err)
		return fmt.Errorf("failed to set write deadline: %w", err)
	}

	// Send the data
	bytesWritten, err := conn.Write(data)
	if err != nil {
		// Classify error type
		errorType := "unknown"
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			errorType = "timeout"
		}
		u.recordSendError(errorType, err)
		return fmt.Errorf("failed to write data: %w", err)
	}

	// Record successful send metrics
	u.udpLogRate.Add(context.Background(), 1.0,
		metric.WithAttributeSet(
			attribute.NewSet(
				attribute.String("component", "output_udp"),
			),
		),
	)
	u.udpRequestSizeBytes.Record(context.Background(), int64(bytesWritten),
		metric.WithAttributeSet(
			attribute.NewSet(
				attribute.String("component", "output_udp"),
			),
		),
	)

	return nil
}

// recordSendError records metrics for send errors
func (u *UDP) recordSendError(errorType string, err error) {
	ctx := context.Background()

	u.udpSendErrors.Add(ctx, 1,
		metric.WithAttributeSet(
			attribute.NewSet(
				attribute.String("component", "output_udp"),
				attribute.String("error_type", errorType),
			),
		),
	)
}
