// Package pacing paces generator workers between writes.
package pacing

import (
	"time"

	"github.com/cenkalti/backoff/v4"
)

// NewWriteBackoff paces a generator worker: rate between writes, growing
// exponentially to 5s (or rate, when longer) while writes fail, never giving
// up. It starts from rate; setting InitialInterval alone would leave the
// first interval at the library's 500ms default.
func NewWriteBackoff(rate time.Duration) *backoff.ExponentialBackOff {
	bo := backoff.NewExponentialBackOff()
	bo.InitialInterval = rate
	bo.MaxInterval = max(5*time.Second, rate)
	bo.MaxElapsedTime = 0
	bo.Reset()
	return bo
}

// AwaitWake parks a worker until wait closes (a released permit went back to
// the pool, or the tracker was reset) or stop closes, and reports whether to
// keep going. A wake is not a write failure, so the backoff restarts at the
// rate.
func AwaitWake(stop, wait <-chan struct{}, bo *backoff.ExponentialBackOff, timer interface{ Reset(time.Duration) bool }) bool {
	select {
	case <-stop:
		return false
	case <-wait:
		bo.Reset()
		timer.Reset(bo.NextBackOff())
		return true
	}
}
