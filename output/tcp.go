package output

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/observiq/bindplane-loader/internal/workermanager"
	"go.uber.org/zap"
)

const (
	// DefaultTCPChannelSize is the default size of the data channel
	DefaultTCPChannelSize = 100

	// DefaultTCPWorkers is the default number of worker goroutines
	DefaultTCPWorkers = 1

	// DefaultTCPConnectTimeout is the default timeout for establishing TCP connections
	DefaultTCPConnectTimeout = 10 * time.Second

	// DefaultTCPWriteTimeout is the default timeout for writing data to TCP connections
	DefaultTCPWriteTimeout = 5 * time.Second

	// DefaultTCPStopTimeout is the default timeout for graceful shutdown
	DefaultTCPStopTimeout = 30 * time.Second
)

// TCP implements the Output interface for TCP connections
type TCP struct {
	logger        *zap.Logger
	host          string
	port          string
	workers       int
	dataChan      chan []byte
	ctx           context.Context
	cancel        context.CancelFunc
	workerManager *workermanager.WorkerManager
}

// NewTCP creates a new TCP output instance
func NewTCP(logger *zap.Logger, host, port string, workers int) (*TCP, error) {
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
		workers = DefaultTCPWorkers
	}

	ctx, cancel := context.WithCancel(context.Background())

	tcp := &TCP{
		logger:   logger.Named("output-tcp"),
		host:     host,
		port:     port,
		workers:  workers,
		dataChan: make(chan []byte, DefaultTCPChannelSize),
		ctx:      ctx,
		cancel:   cancel,
	}

	tcp.logger.Info("Starting TCP output",
		zap.String("host", tcp.host),
		zap.String("port", tcp.port),
		zap.Int("workers", tcp.workers),
		zap.Int("channel_size", DefaultTCPChannelSize),
	)

	// Create worker manager
	tcp.workerManager = workermanager.NewWorkerManager(tcp.logger, workers, tcp.tcpWorker)

	// Start the workers
	tcp.workerManager.Start()

	return tcp, nil
}

// Write sends data to the TCP output channel for processing by workers.
// Write shall not be called after Stop is called.
// If the provided context is done, Write will return immediately
// even if the data is not written to the channel.
func (t *TCP) Write(ctx context.Context, data []byte) error {
	select {
	case t.dataChan <- data:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("context cancelled while waiting to write data: %w", ctx.Err())
	case <-t.ctx.Done():
		return fmt.Errorf("TCP output is shutting down")
	}
}

// Stop gracefully shuts down all workers and closes TCP connections
// Stop shall not be called more than once.
// If the provided context is done, Stop will return immediately
// even if workers are still shutting down.
func (t *TCP) Stop(ctx context.Context) error {
	t.logger.Info("Stopping TCP output")

	// Close the channel to ensure workers do not
	// process new data.
	close(t.dataChan)

	// Signal the workers to stop.
	t.cancel()

	// Stop the worker manager
	t.workerManager.Stop()

	t.logger.Info("TCP output stopped successfully")
	return nil
}

// tcpWorker processes TCP data from the channel and sends it to the configured host and port.
// This function is designed to work with the worker manager, which handles automatic restart
// with exponential backoff when the worker exits due to connection failures or errors.
// The worker should return immediately on any failure - the worker manager will handle
// reconnection attempts with appropriate backoff delays.
func (t *TCP) tcpWorker(id int) {
	t.logger.Info("Starting TCP worker", zap.Int("worker_id", id))

	conn, err := t.connect()
	if err != nil {
		t.logger.Error("Failed to establish initial TCP connection",
			zap.Int("worker_id", id),
			zap.Error(err))
		return
	}
	defer conn.Close()

	for {
		select {
		case data, ok := <-t.dataChan:
			if !ok {
				t.logger.Info("TCP worker exiting - channel closed", zap.Int("worker_id", id))
				return
			}

			if err := t.sendData(conn, data); err != nil {
				t.logger.Error("Failed to send TCP data",
					zap.Int("worker_id", id),
					zap.Error(err))
				return
			}

		case <-t.ctx.Done():
			t.logger.Info("TCP worker exiting - context cancelled", zap.Int("worker_id", id))
			return
		}
	}
}

// connect establishes a TCP connection to the configured host and port
func (t *TCP) connect() (net.Conn, error) {
	address := net.JoinHostPort(t.host, t.port)

	conn, err := net.DialTimeout("tcp", address, DefaultTCPConnectTimeout)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %w", address, err)
	}

	return conn, nil
}

// sendData sends data to the TCP connection with a timeout
func (t *TCP) sendData(conn net.Conn, data []byte) error {
	// Set write timeout
	if err := conn.SetWriteDeadline(time.Now().Add(DefaultTCPWriteTimeout)); err != nil {
		return fmt.Errorf("failed to set write deadline: %w", err)
	}

	// Send the data
	_, err := conn.Write(data)
	if err != nil {
		return fmt.Errorf("failed to write data: %w", err)
	}

	return nil
}
