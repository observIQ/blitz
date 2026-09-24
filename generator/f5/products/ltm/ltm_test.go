package ltm

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

	got := build(rand.New(rand.NewSource(42)), c)
	again := build(rand.New(rand.NewSource(42)), c)
	require.Equal(t, got, again, "same seed must yield identical output")

	require.True(t, strings.HasPrefix(got, "<134>Sep 24 15:04:05 bigip1 tmm["), got)
	require.Contains(t, got, "vs=/Common/")
	require.Contains(t, got, "HTTP/1.1")
}

func TestRegistered(t *testing.T) {
	_, ok := catalog.Get("ltm")
	require.True(t, ok)
}
