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
