// Package workermanager provides a robust worker management system with graceful reconnection
// and exponential backoff for handling network operations that may fail.
//
// The WorkerManager is designed to manage goroutines that perform potentially failing operations,
// such as network I/O, with automatic retry logic and graceful shutdown capabilities.
//
// Key Features:
//   - Automatic worker restart with exponential backoff
//   - Configurable retry policies with sane defaults
//   - Context-aware shutdown
//   - Comprehensive logging of failures and retries
//   - Thread-safe worker count tracking
//
// Example Usage for TCP Output:
//
//	type TCPOutput struct {
//		logger        *zap.Logger
//		host          string
//		port          string
//		dataChan      chan []byte
//		workerManager *workermanager.WorkerManager
//	}
//
//	func NewTCPOutput(logger *zap.Logger, host, port string, workers int) (*TCPOutput, error) {
//		tcp := &TCPOutput{
//			logger:   logger.Named("tcp-output"),
//			host:     host,
//			port:     port,
//			dataChan: make(chan []byte, 100),
//		}
//
//		// Create worker manager with TCP worker function
//		tcp.workerManager = workermanager.NewWorkerManager(
//			tcp.logger,
//			workers,
//			tcp.tcpWorker,
//		)
//
//		// Start the workers
//		tcp.workerManager.Start()
//		return tcp, nil
//	}
//
//	func (t *TCPOutput) tcpWorker(id int) {
//		// Establish TCP connection
//		conn, err := net.Dial("tcp", net.JoinHostPort(t.host, t.port))
//		if err != nil {
//			t.logger.Error("Failed to connect", zap.Error(err))
//			return // Worker manager will retry with backoff
//		}
//		defer conn.Close()
//
//		// Process data from channel
//		for {
//			select {
//			case data, ok := <-t.dataChan:
//				if !ok {
//					return // Channel closed, exit gracefully
//				}
//				if _, err := conn.Write(data); err != nil {
//					t.logger.Error("Failed to write data", zap.Error(err))
//					return // Worker manager will retry with backoff
//				}
//			case <-t.ctx.Done():
//				return // Context cancelled, exit gracefully
//			}
//		}
//	}
//
//	func (t *TCPOutput) Stop() {
//		close(t.dataChan)
//		t.workerManager.Stop()
//	}
//
// Example Usage for UDP Output:
//
//	type UDPOutput struct {
//		logger        *zap.Logger
//		host          string
//		port          string
//		dataChan      chan []byte
//		workerManager *workermanager.WorkerManager
//	}
//
//	func NewUDPOutput(logger *zap.Logger, host, port string, workers int) (*UDPOutput, error) {
//		udp := &UDPOutput{
//			logger:   logger.Named("udp-output"),
//			host:     host,
//			port:     port,
//			dataChan: make(chan []byte, 100),
//		}
//
//		// Create worker manager with UDP worker function
//		udp.workerManager = workermanager.NewWorkerManager(
//			udp.logger,
//			workers,
//			udp.udpWorker,
//		)
//
//		// Start the workers
//		udp.workerManager.Start()
//		return udp, nil
//	}
//
//	func (u *UDPOutput) udpWorker(id int) {
//		// Establish UDP connection
//		conn, err := net.Dial("udp", net.JoinHostPort(u.host, u.port))
//		if err != nil {
//			u.logger.Error("Failed to connect", zap.Error(err))
//			return // Worker manager will retry with backoff
//		}
//		defer conn.Close()
//
//		// Process data from channel
//		for {
//			select {
//			case data, ok := <-u.dataChan:
//				if !ok {
//					return // Channel closed, exit gracefully
//				}
//				if _, err := conn.Write(data); err != nil {
//					u.logger.Error("Failed to write data", zap.Error(err))
//					return // Worker manager will retry with backoff
//				}
//			case <-u.ctx.Done():
//				return // Context cancelled, exit gracefully
//			}
//		}
//	}
//
//	func (u *UDPOutput) Stop() {
//		close(u.dataChan)
//		u.workerManager.Stop()
//	}
//
// The WorkerManager handles all the complexity of:
//   - Restarting failed workers with exponential backoff
//   - Logging retry attempts and failures
//   - Graceful shutdown when Stop() is called
//   - Tracking active worker count
//   - Preventing resource leaks
package workermanager

import (
	"context"
	"sync"
	"time"

	"github.com/cenkalti/backoff/v4"
	"go.uber.org/zap"
)

// WorkerManager manages worker goroutines with graceful reconnection
type WorkerManager struct {
	logger        *zap.Logger
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup
	workerFunc    func(id int)
	workerCount   int
	activeWorkers int32
	mu            sync.RWMutex
}

// NewWorkerManager creates a new worker manager
func NewWorkerManager(logger *zap.Logger, workerCount int, workerFunc func(id int)) *WorkerManager {
	ctx, cancel := context.WithCancel(context.Background())

	return &WorkerManager{
		logger:      logger,
		ctx:         ctx,
		cancel:      cancel,
		workerFunc:  workerFunc,
		workerCount: workerCount,
	}
}

// Start starts the worker manager and spawns initial workers
func (wm *WorkerManager) Start() {
	wm.logger.Info("Starting worker manager", zap.Int("target_workers", wm.workerCount))

	for i := 0; i < wm.workerCount; i++ {
		wm.startWorker(i)
	}
}

// Stop stops the worker manager and waits for all workers to finish
func (wm *WorkerManager) Stop() {
	wm.logger.Info("Stopping worker manager")
	wm.cancel()
	wm.wg.Wait()
	wm.logger.Info("Worker manager stopped")
}

// startWorker starts a single worker with graceful reconnection
func (wm *WorkerManager) startWorker(id int) {
	wm.mu.Lock()
	wm.activeWorkers++
	wm.mu.Unlock()

	wm.wg.Add(1)
	go wm.runWorker(id)
}

// runWorker runs a worker with exponential backoff for reconnection
func (wm *WorkerManager) runWorker(id int) {
	defer wm.wg.Done()
	defer func() {
		wm.mu.Lock()
		wm.activeWorkers--
		wm.mu.Unlock()
	}()

	// Create exponential backoff with sane defaults
	backoffPolicy := backoff.NewExponentialBackOff(
		backoff.WithInitialInterval(100*time.Millisecond),
		backoff.WithMaxInterval(30*time.Second),
		backoff.WithMultiplier(2),
		backoff.WithRandomizationFactor(0.1),
		backoff.WithMaxElapsedTime(5*time.Minute),
	)

	for {
		select {
		case <-wm.ctx.Done():
			wm.logger.Info("Worker exiting - context cancelled", zap.Int("worker_id", id))
			return
		default:
			// Run the worker function
			wm.workerFunc(id)

			// If worker function returns, it means it failed
			// Check if we should retry
			select {
			case <-wm.ctx.Done():
				wm.logger.Info("Worker exiting - context cancelled during backoff", zap.Int("worker_id", id))
				return
			default:
				// Calculate next backoff delay
				delay := backoffPolicy.NextBackOff()
				if delay == backoff.Stop {
					wm.logger.Error("Worker failed permanently - max retry time exceeded", zap.Int("worker_id", id))
					return
				}

				wm.logger.Warn("Worker failed, retrying with backoff",
					zap.Int("worker_id", id),
					zap.Duration("delay", delay))

				// Wait for backoff delay or context cancellation
				select {
				case <-wm.ctx.Done():
					return
				case <-time.After(delay):
					// Continue to retry
				}
			}
		}
	}
}

// GetActiveWorkerCount returns the current number of active workers
func (wm *WorkerManager) GetActiveWorkerCount() int {
	wm.mu.RLock()
	defer wm.mu.RUnlock()
	return int(wm.activeWorkers)
}
