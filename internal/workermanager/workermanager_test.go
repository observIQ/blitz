package workermanager

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestStopDoesNotLogWorkerFailure(t *testing.T) {
	core, logs := observer.New(zap.WarnLevel)

	started := make(chan struct{})
	var wm *WorkerManager
	wm = NewWorkerManager(zap.New(core), 1, func(int) {
		close(started)
		<-wm.ctx.Done()
	})
	wm.Start()
	<-started
	wm.Stop()

	require.Empty(t, logs.All())
}
