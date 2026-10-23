package main

import (
	"context"
	"testing"
	"time"

	"github.com/observiq/blitz/generator/count"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
	"go.uber.org/zap/zaptest/observer"
)

// captureSignal returns a waiting hook that reports each channel capture,
// so a test can land a reset strictly after the wait took the old channels.
func captureSignal() (<-chan struct{}, func()) {
	captured := make(chan struct{}, 8)
	return captured, func() { captured <- struct{}{} }
}

func awaitCapture(t *testing.T, captured <-chan struct{}) {
	t.Helper()
	select {
	case <-captured:
	case <-time.After(5 * time.Second):
		t.Fatal("waitForFinish never took the tracker's channels")
	}
}

// deliver takes and commits n permits.
func deliver(t *testing.T, tracker *count.Tracker, n int) {
	t.Helper()
	for range n {
		p, _, ok := tracker.Acquire()
		require.True(t, ok)
		p.Commit()
	}
}

// TestWaitForFinishExitSurvivesMidRunReset pins that a reset (SIGUSR1) before
// the count is reached does not strand the exit-mode wait on the channel the
// reset replaced: the run still ends when the refilled count is reached.
func TestWaitForFinishExitSurvivesMidRunReset(t *testing.T) {
	t.Parallel()
	captured, waiting := captureSignal()
	tracker := count.NewTracker(2)
	done := make(chan struct{})
	go func() {
		waitForFinish(context.Background(), zaptest.NewLogger(t), tracker, "exit", waiting)
		close(done)
	}()

	awaitCapture(t, captured)
	deliver(t, tracker, 1)
	tracker.Reset()
	deliver(t, tracker, 2)

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("exit-mode wait never returned after the refilled count was reached")
	}
}

// TestWaitForFinishIdleSurvivesMidRunReset pins the idle-mode counterpart:
// after a mid-run reset, reaching the refilled count still moves the run to
// idle, and cancelling the context ends the wait.
func TestWaitForFinishIdleSurvivesMidRunReset(t *testing.T) {
	t.Parallel()
	captured, waiting := captureSignal()
	core, logs := observer.New(zap.InfoLevel)
	tracker := count.NewTracker(2)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		waitForFinish(ctx, zap.New(core), tracker, "idle", waiting)
		close(done)
	}()

	awaitCapture(t, captured)
	deliver(t, tracker, 1)
	tracker.Reset()
	deliver(t, tracker, 2)
	require.Eventually(t, func() bool {
		return logs.FilterMessageSnippet("idling").Len() == 1
	}, 5*time.Second, 5*time.Millisecond, "delivering the refilled count moves the run to idle")

	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("idle-mode wait never returned after cancel")
	}
}

// TestWaitForFinishReturnsOnCancel pins that cancelling the context ends the
// wait before the count is reached and while idling after it.
func TestWaitForFinishReturnsOnCancel(t *testing.T) {
	t.Parallel()
	for name, reach := range map[string]bool{"before the count": false, "while idling": true} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			core, logs := observer.New(zap.InfoLevel)
			tracker := count.NewTracker(1)
			ctx, cancel := context.WithCancel(context.Background())
			done := make(chan struct{})
			go func() {
				waitForFinish(ctx, zap.New(core), tracker, "idle", nil)
				close(done)
			}()
			if reach {
				deliver(t, tracker, 1)
				require.Eventually(t, func() bool { return logs.FilterMessageSnippet("idling").Len() == 1 },
					5*time.Second, 5*time.Millisecond)
			}
			cancel()
			select {
			case <-done:
			case <-time.After(5 * time.Second):
				t.Fatal("wait never returned after cancel")
			}
		})
	}
}

// TestWaitForFinishIdleRestartsAndIdlesAgain pins the idle cycle: the count
// is reached and the run idles, a reset (SIGUSR1) restarts it, and reaching
// the refilled count idles it again.
func TestWaitForFinishIdleRestartsAndIdlesAgain(t *testing.T) {
	t.Parallel()
	core, logs := observer.New(zap.InfoLevel)
	tracker := count.NewTracker(1)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		waitForFinish(ctx, zap.New(core), tracker, "idle", nil)
		close(done)
	}()
	logged := func(snippet string, n int) func() bool {
		return func() bool { return logs.FilterMessageSnippet(snippet).Len() == n }
	}

	deliver(t, tracker, 1)
	require.Eventually(t, logged("idling", 1), 5*time.Second, 5*time.Millisecond)
	tracker.Reset()
	require.Eventually(t, logged("Generation restarted", 1), 5*time.Second, 5*time.Millisecond)
	deliver(t, tracker, 1)
	require.Eventually(t, logged("idling", 2), 5*time.Second, 5*time.Millisecond,
		"the refilled count idles the run again")

	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("idle-mode wait never returned after cancel")
	}
}
