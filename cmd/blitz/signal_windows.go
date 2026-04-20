//go:build windows

package main

import (
	"context"

	"github.com/observiq/blitz/generator/count"
	"go.uber.org/zap"
)

// setupRestartSignal is a no-op on Windows. SIGUSR1 is not available.
func setupRestartSignal(_ context.Context, logger *zap.Logger, tracker *count.Tracker) {
	if tracker != nil {
		logger.Warn("SIGUSR1 restart signal is not supported on Windows")
	}
}
