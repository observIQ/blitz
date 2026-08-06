package runtime_test

import (
	"testing"
	"time"

	"github.com/observiq/blitz/internal/runtime"
	"github.com/stretchr/testify/require"
)

func TestDurationMillis(t *testing.T) {
	cases := []struct {
		name string
		in   time.Duration
		want float64
	}{
		{"whole milliseconds", 250 * time.Millisecond, 250},
		{"seconds scale", 2 * time.Second, 2000},
		{"sub-millisecond preserved", 500 * time.Microsecond, 0.5},
		{"zero", 0, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.InDelta(t, tc.want, runtime.DurationMillis(tc.in), 1e-9)
		})
	}
}
