package irules

import (
	"math/rand"
	"strings"
	"testing"
	"time"

	"github.com/observiq/blitz/generator/f5/catalog"
	"github.com/stretchr/testify/require"
)

func TestBuildDeterministicAndShaped(t *testing.T) {
	c := &catalog.Ctx{Now: time.Date(2026, 9, 24, 15, 4, 5, 0, time.UTC), Hostname: "bigip1"}
	got := build(rand.New(rand.NewSource(17)), c)
	require.Equal(t, got, build(rand.New(rand.NewSource(17)), c))

	require.True(t, strings.HasPrefix(got, "<134>Sep 24 15:04:05 bigip1 tmm["), got)
	require.Contains(t, got, "Rule /Common/")
	require.Contains(t, got, ">:")
}

func TestRegistered(t *testing.T) {
	_, ok := catalog.Get("irules")
	require.True(t, ok)
}
