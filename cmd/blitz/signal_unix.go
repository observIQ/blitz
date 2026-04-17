//go:build !windows

package main

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/observiq/blitz/generator/count"
	"go.uber.org/zap"
)

// setupRestartSignal listens for SIGUSR1 and resets the count tracker,
// allowing generation to resume in idle mode.
func setupRestartSignal(logger *zap.Logger, tracker *count.Tracker) {
	if tracker == nil {
		return
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGUSR1)

	go func() {
		for range sigChan {
			logger.Info("Received SIGUSR1, resetting generation count")
			tracker.Reset()
		}
	}()
}
