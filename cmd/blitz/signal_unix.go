//go:build !windows

package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/observiq/blitz/generator/count"
	"go.uber.org/zap"
)

// setupRestartSignal listens for SIGUSR1 and resets the count tracker,
// allowing generation to resume in idle mode. The goroutine exits when
// ctx is cancelled.
func setupRestartSignal(ctx context.Context, logger *zap.Logger, tracker *count.Tracker) {
	if tracker == nil {
		return
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGUSR1)

	go func() {
		defer signal.Stop(sigChan)
		for {
			select {
			case <-ctx.Done():
				return
			case <-sigChan:
				logger.Info("Received SIGUSR1, resetting generation count")
				tracker.Reset()
			}
		}
	}()
}
