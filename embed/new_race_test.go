package embed_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/observiq/blitz/embed"
	"github.com/observiq/blitz/generator/apache"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// TestRunner_ConcurrentStartIsSerialized fires many concurrent Start
// calls. The mutex must serialize them so exactly one wins and the
// others return "already started" without racing on the (started, rt,
// resource) joint state. Run with -race to catch any field-level data
// race; without -race this still asserts the "first wins" semantics.
//
// Underlying modules (e.g. apache) close their stopCh in Stop and are
// not restartable, so we deliberately avoid restart scenarios here.
func TestRunner_ConcurrentStartIsSerialized(t *testing.T) {
	logger := zaptest.NewLogger(t)
	consumer := &memoryLogConsumer{}

	gen, err := apache.New(logger, 1, 10*time.Millisecond, consumer, embed.NopTelemetry())
	require.NoError(t, err)

	runner, err := embed.New(embed.Config{Modules: []embed.ProducerModule{gen}})
	require.NoError(t, err)

	host := embed.Host{Logs: consumer, Telemetry: embed.TelemetrySettings{Logger: logger}}

	const goroutines = 8
	var wg sync.WaitGroup
	var startMu sync.Mutex
	startResults := make([]error, 0, goroutines)

	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			err := runner.Start(context.Background(), host)
			startMu.Lock()
			startResults = append(startResults, err)
			startMu.Unlock()
		}()
	}
	wg.Wait()

	successes := 0
	for _, err := range startResults {
		if err == nil {
			successes++
		}
	}
	require.Equal(t, 1, successes, "exactly one Start should succeed; the rest should return already-started")

	stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, runner.Stop(stopCtx))
}

// TestRunner_ConcurrentStopIsIdempotent fires many concurrent Stop
// calls after a single successful Start. The mutex must serialize them
// so only the first acquires the running state, runs the underlying
// Stop, and clears the flag; the rest see started=false and return nil.
// Pre-mutex this would race on (started, rt) and could double-close the
// module's stopCh.
func TestRunner_ConcurrentStopIsIdempotent(t *testing.T) {
	logger := zaptest.NewLogger(t)
	consumer := &memoryLogConsumer{}

	gen, err := apache.New(logger, 1, 10*time.Millisecond, consumer, embed.NopTelemetry())
	require.NoError(t, err)

	runner, err := embed.New(embed.Config{Modules: []embed.ProducerModule{gen}})
	require.NoError(t, err)

	host := embed.Host{Logs: consumer, Telemetry: embed.TelemetrySettings{Logger: logger}}
	require.NoError(t, runner.Start(context.Background(), host))

	const goroutines = 8
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			require.NoError(t, runner.Stop(stopCtx))
		}()
	}
	wg.Wait()
}
