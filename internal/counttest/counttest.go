// Package counttest helps output tests check how a record's count permit
// settles.
package counttest

import (
	"context"
	"testing"
	"time"

	"github.com/observiq/blitz/generator/count"
	"github.com/stretchr/testify/require"
)

// Held returns a context carrying a fresh permit, and the permit. Call
// p.Commit after the write returns, as a generator does; the permit then
// settles on the output's result.
func Held(t *testing.T) (ctx context.Context, p count.Permit) {
	t.Helper()
	p, _, ok := count.NewTracker(1).Acquire()
	if !ok {
		t.Fatal("a fresh tracker refused a permit")
	}
	return count.WithPermit(context.Background(), p), p
}

// Settled waits up to 5s for p to settle and reports whether it did.
func Settled(p count.Permit) bool {
	select {
	case <-p.Settled():
		return true
	case <-time.After(5 * time.Second):
		return false
	}
}

// StaysPending fails the test if p settles within a short window, as it
// would if an output settled on dequeue rather than on the send's result.
func StaysPending(t *testing.T, p count.Permit, msgAndArgs ...any) {
	t.Helper()
	require.Never(t, func() bool {
		select {
		case <-p.Settled():
			return true
		default:
			return false
		}
	}, 100*time.Millisecond, 5*time.Millisecond, msgAndArgs...)
}

// ReleasesUnqueued pins that a record that never reaches an output's queue,
// on a cancelled write or a stopping output, is released. write builds the
// output around outputCtx and writes one record with writeCtx; its queue must
// have no reader, so only the two contexts can end the write.
func ReleasesUnqueued(t *testing.T, write func(writeCtx, outputCtx context.Context) error) {
	t.Helper()
	stopping, stop := context.WithCancel(context.Background())
	stop()
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	for name, tc := range map[string]struct{ write, output context.Context }{
		"write cancelled": {cancelled, context.Background()},
		"output stopping": {context.Background(), stopping},
	} {
		t.Run(name, func(t *testing.T) {
			ctx, p := Held(t)
			require.Error(t, write(count.Inherit(tc.write, ctx), tc.output))
			p.Commit()
			require.True(t, Settled(p))
			require.False(t, p.Delivered())
		})
	}
}

// WakesOnReset pins that a generator worker refused a permit waits for the
// tracker, then generates again once Reset is called. Start the generator
// first, with one worker at a short rate and tracker counting 1; written
// reports the records written so far.
func WakesOnReset(t *testing.T, tracker *count.Tracker, written func() int) {
	t.Helper()
	for cycle := 1; cycle <= 2; cycle++ {
		select {
		case <-tracker.Done():
		case <-time.After(10 * time.Second):
			t.Fatalf("cycle %d never completed", cycle)
		}
		// At a short rate the worker is refused many times over this window,
		// so Reset finds it waiting.
		require.Never(t, func() bool { return written() > cycle },
			50*time.Millisecond, time.Millisecond, "the refused worker wrote nothing more")
		tracker.Reset()
	}
}

// StopsWhileRefused pins that a generator worker refused a permit stops when
// the generator stops. Start the generator first, with one worker at a short
// rate and tracker counting 1; written reports the records written so far.
func StopsWhileRefused(t *testing.T, tracker *count.Tracker, written func() int, stop func(context.Context) error) {
	t.Helper()
	select {
	case <-tracker.Done():
	case <-time.After(10 * time.Second):
		t.Fatal("the count never completed")
	}
	// At a short rate the worker is refused within this window, so Stop
	// finds it waiting on the tracker.
	require.Never(t, func() bool { return written() > 1 },
		50*time.Millisecond, time.Millisecond, "the refused worker wrote nothing more")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, stop(ctx))
	require.NoError(t, ctx.Err(), "the refused worker did not stop")
}
