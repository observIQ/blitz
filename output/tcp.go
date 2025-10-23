package output

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"
)

const (
	// DefaultChannelSize is the default size of the data channel
	DefaultChannelSize = 100

	// DefaultWorkers is the default number of worker goroutines
	DefaultWorkers = 1

	// DefaultConnectTimeout is the default timeout for establishing TCP connections
	DefaultConnectTimeout = 10 * time.Second

	// DefaultWriteTimeout is the default timeout for writing data to TCP connections
	DefaultWriteTimeout = 5 * time.Second

	// DefaultStopTimeout is the default timeout for graceful shutdown
	DefaultStopTimeout = 30 * time.Second
)

// TCP implements the Output interface for TCP connections
type TCP struct {
	host     string
	port     string
	workers  int
	dataChan chan []byte
	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup
}

// NewTCP creates a new TCP output instance
func NewTCP(host, port string, workers int) (*TCP, error) {
	if host == "" {
		return nil, fmt.Errorf("host cannot be empty")
	}
	if port == "" {
		return nil, fmt.Errorf("port cannot be empty")
	}
	if workers <= 0 {
		workers = DefaultWorkers
	}

	ctx, cancel := context.WithCancel(context.Background())

	tcp := &TCP{
		host:     host,
		port:     port,
		workers:  workers,
		dataChan: make(chan []byte, DefaultChannelSize),
		ctx:      ctx,
		cancel:   cancel,
	}

	// Start the workers
	tcp.startWorkers()

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
	// Close the channel to ensure workers do not
	// process new data.
	close(t.dataChan)

	// Signal the workers to stop.
	t.cancel()

	// Wait for all workers to finish or timeout
	done := make(chan struct{})
	go func() {
		t.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("context cancelled while waiting for workers to finish: %w", ctx.Err())
	}
}

// startWorkers starts the worker goroutines
func (t *TCP) startWorkers() {
	for i := 0; i < t.workers; i++ {
		t.wg.Add(1)
		go t.worker(i)
	}
}

// worker processes data from the channel and sends it via TCP
func (t *TCP) worker(id int) {
	defer t.wg.Done()

	var conn net.Conn
	var err error

	for {
		select {
		case data, ok := <-t.dataChan:
			if !ok {
				// Channel closed, worker should exit
				if conn != nil {
					conn.Close()
				}
				return
			}

			// Establish connection if not already connected
			if conn == nil {
				conn, err = t.connect()
				if err != nil {
					// Log error and continue to next data
					continue
				}
			}

			// Send data with timeout
			if err := t.sendData(conn, data); err != nil {
				// Connection error, close and retry next time
				conn.Close()
				conn = nil
			}

		case <-t.ctx.Done():
			// Context cancelled, worker should exit
			if conn != nil {
				conn.Close()
			}
			return
		}
	}
}

// connect establishes a TCP connection to the configured host and port
func (t *TCP) connect() (net.Conn, error) {
	address := net.JoinHostPort(t.host, t.port)

	conn, err := net.DialTimeout("tcp", address, DefaultConnectTimeout)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %w", address, err)
	}

	return conn, nil
}

// sendData sends data to the TCP connection with a timeout
func (t *TCP) sendData(conn net.Conn, data []byte) error {
	// Set write timeout
	if err := conn.SetWriteDeadline(time.Now().Add(DefaultWriteTimeout)); err != nil {
		return fmt.Errorf("failed to set write deadline: %w", err)
	}

	// Send the data
	_, err := conn.Write(data)
	if err != nil {
		return fmt.Errorf("failed to write data: %w", err)
	}

	return nil
}
