// Package workermanager provides a robust worker management system with graceful reconnection
// and exponential backoff for handling network operations that may fail.
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
	activeWorkers int
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

	backoffPolicy := backoff.NewExponentialBackOff(
		backoff.WithInitialInterval(100*time.Millisecond),
		backoff.WithMaxInterval(30*time.Second),
		backoff.WithMaxElapsedTime(0), // No max elapsed time - retry forever
		backoff.WithMultiplier(2),
		backoff.WithRandomizationFactor(0.1),
	)

	for {
		// Check if context is cancelled before running worker
		if wm.ctx.Err() != nil {
			wm.logger.Info("Worker exiting - context cancelled", zap.Int("worker_id", id))
			return
		}

		// Run the worker function
		wm.workerFunc(id)

		// If worker function returns, it means it failed - retry with backoff
		delay := backoffPolicy.NextBackOff()
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

// GetActiveWorkerCount returns the current number of active workers
func (wm *WorkerManager) GetActiveWorkerCount() int {
	wm.mu.RLock()
	defer wm.mu.RUnlock()
	return wm.activeWorkers
}
