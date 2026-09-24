package asm

import (
	"math/rand"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/observiq/blitz/generator/f5/catalog"
	"github.com/stretchr/testify/require"
)

func TestBuildDeterministic(t *testing.T) {
	c := &catalog.Ctx{Now: time.Date(2026, 9, 24, 15, 4, 5, 0, time.UTC), Hostname: "bigip1"}
	got := build(rand.New(rand.NewSource(7)), c)
	require.Equal(t, got, build(rand.New(rand.NewSource(7)), c))
	require.True(t, strings.HasPrefix(got, "<134>Sep 24 15:04:05 bigip1 ASM:"), got)
}

// TestDefaultFieldSetAndOrder asserts the emitted key set and order match F5's
// documented default ASM syslog field set (16 fields).
func TestDefaultFieldSetAndOrder(t *testing.T) {
	c := &catalog.Ctx{Now: time.Date(2026, 9, 24, 15, 4, 5, 0, time.UTC), Hostname: "bigip1"}
	got := build(rand.New(rand.NewSource(1)), c)
	body := got[strings.Index(got, "ASM:")+len("ASM:")+1:]

	keyRe := regexp.MustCompile(`(?:^|,)([a-z0-9_]+)="`)
	matches := keyRe.FindAllStringSubmatch(body, -1)
	keys := make([]string, 0, len(matches))
	for _, m := range matches {
		keys = append(keys, m[1])
	}
	require.Equal(t, DefaultFields(), keys)
	require.Len(t, keys, 16)
}

func TestRegistered(t *testing.T) {
	_, ok := catalog.Get("asm")
	require.True(t, ok)
}
