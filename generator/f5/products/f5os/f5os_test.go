package f5os

import (
	"math/rand"
	"strings"
	"testing"
	"time"

	"github.com/observiq/blitz/generator/f5/catalog"
	"github.com/stretchr/testify/require"
)

func TestBuildDeterministicAndShaped(t *testing.T) {
	c := &catalog.Ctx{Now: time.Date(2026, 9, 24, 15, 4, 5, 0, time.UTC), Hostname: "f5os-a"}
	got := build(rand.New(rand.NewSource(23)), c)
	require.Equal(t, got, build(rand.New(rand.NewSource(23)), c))
	require.True(t, strings.HasPrefix(got, "<"), got)
	require.Contains(t, got, "f5os-a")
}

func TestRegistered(t *testing.T) {
	_, ok := catalog.Get("f5os")
	require.True(t, ok)
}
