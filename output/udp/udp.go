package udp

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/observiq/blitz/internal/workermanager"
	"github.com/observiq/blitz/output"
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

// outputType is the output_type attribute value for UDP metrics.
const outputType = "udp"

// UDP implements the Output interface for UDP connections
type UDP struct {
	logger        *zap.Logger
	host          string
	port          string
	workers       int
	dataChan      chan string
	ctx           context.Context
	cancel        context.CancelFunc
	workerManager *workermanager.WorkerManager
}

// New creates a new UDP output instance
func New(logger *zap.Logger, host, port string, workers int) (*UDP, error) {
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

	udp := &UDP{
		logger:   logger.Named("output-udp"),
		host:     host,
		port:     port,
		workers:  workers,
		dataChan: make(chan string, DefaultUDPChannelSize),
		ctx:      ctx,
		cancel:   cancel,
	}

	udp.logger.Info("Starting UDP output",
		zap.String("host", udp.host),
		zap.String("port", udp.port),
		zap.Int("workers", udp.workers),
		zap.Int("channel_size", DefaultUDPChannelSize),
	)

	// Register observable metrics (queue_size)
	output.InitObservableMetrics(udp)

	// Create worker manager
	udp.workerManager = workermanager.NewWorkerManager(udp.logger, workers, udp.udpWorker)

	// Record initial active workers count
	output.BlitzOutputActiveWorkersGauge.Record(context.Background(), int64(workers), outputType)

	// Start the workers
	udp.workerManager.Start()

	return udp, nil
}

// ObserveBlitzOutputQueueSize implements the output.ObservableCallbacks interface
func (u *UDP) ObserveBlitzOutputQueueSize(_ context.Context, observer metric.Int64Observer) error {
	observer.Observe(int64(len(u.dataChan)))
	return nil
}

// Write sends data to the UDP output channel for processing by workers.
// Write shall not be called after Stop is called.
// If the provided context is done, Write will return immediately
// even if the data is not written to the channel.
func (u *UDP) Write(ctx context.Context, data output.LogRecord) error {
	select {
	case u.dataChan <- data.Message:
		output.BlitzOutputEntriesReceivedCounter.Add(ctx, 1, outputType)
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
	output.BlitzOutputActiveWorkersGauge.Record(ctx, 0, outputType)

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
func (u *UDP) sendData(conn net.Conn, data string) error {
	// Set write timeout
	if err := conn.SetWriteDeadline(time.Now().Add(DefaultUDPWriteTimeout)); err != nil {
		u.recordSendError("unknown", err)
		return fmt.Errorf("failed to set write deadline: %w", err)
	}

	// Send the data
	bytesWritten, err := conn.Write([]byte(data))
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
	output.BlitzOutputEntryRateCounter.Add(context.Background(), 1.0, outputType)
	output.BlitzOutputRequestSizeHistogram.Record(context.Background(), int64(bytesWritten), outputType)

	return nil
}

// recordSendError records metrics for send errors
func (u *UDP) recordSendError(_ string, _ error) {
	output.BlitzOutputSendErrorsCounter.Add(context.Background(), 1, outputType)
}
