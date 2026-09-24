package apm

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
	got := build(rand.New(rand.NewSource(9)), c)
	require.Equal(t, got, build(rand.New(rand.NewSource(9)), c))

	require.True(t, strings.HasPrefix(got, "<134>Sep 24 15:04:05 bigip1 apmd["), got)
	require.Contains(t, got, ":5: /Common/access_policy:Common:")
}

func TestRegistered(t *testing.T) {
	_, ok := catalog.Get("apm")
	require.True(t, ok)
}
