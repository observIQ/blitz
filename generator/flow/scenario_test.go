package flow

import (
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func meanBytes(s Scenario, seed int64, n int) float64 {
	r := rand.New(rand.NewSource(seed))
	var sum uint64
	for i := 0; i < n; i++ {
		sum += generateFlow(r, s, time.Unix(1700000000, 0)).Bytes
	}
	return float64(sum) / float64(n)
}

func TestWANEdgeFlowsLargerThanDatacenter(t *testing.T) {
	require.Greater(t, meanBytes(ScenarioWANEdge, 1, 500), meanBytes(ScenarioDatacenter, 1, 500))
}

func TestDDOSTargetSharesDestination(t *testing.T) {
	r := rand.New(rand.NewSource(1))
	first := generateFlow(r, ScenarioDDOSTarget, time.Unix(1700000000, 0)).DstIP
	for i := 0; i < 100; i++ {
		require.True(t, generateFlow(r, ScenarioDDOSTarget, time.Unix(1700000000, 0)).DstIP.Equal(first))
	}
}

func TestDeterministicFromSeed(t *testing.T) {
	a := generateFlow(rand.New(rand.NewSource(42)), ScenarioDefault, time.Unix(1700000000, 0))
	b := generateFlow(rand.New(rand.NewSource(42)), ScenarioDefault, time.Unix(1700000000, 0))
	require.Equal(t, a, b)
}
