package afm

import (
	"math/rand"
	"strings"
	"testing"
	"time"

	"github.com/observiq/blitz/generator/f5/catalog"
	"github.com/stretchr/testify/require"
)

func TestBuildDeterministic(t *testing.T) {
	c := &catalog.Ctx{Now: time.Date(2026, 9, 24, 15, 4, 5, 0, time.UTC), Hostname: "bigip1"}
	got := build(rand.New(rand.NewSource(3)), c)
	require.Equal(t, got, build(rand.New(rand.NewSource(3)), c))
	require.True(t, strings.HasPrefix(got, "<134>Sep 24 15:04:05 bigip1 tmm["), got)
}

// TestDefaultFieldCountAndPositions asserts the emitted positional CSV matches
// the documented AFM default field order (14 fields).
func TestDefaultFieldCountAndPositions(t *testing.T) {
	c := &catalog.Ctx{Now: time.Date(2026, 9, 24, 15, 4, 5, 0, time.UTC), Hostname: "bigip1"}
	got := build(rand.New(rand.NewSource(3)), c)
	body := got[strings.Index(got, "]: ")+3:]
	fields := strings.Split(body, ",")

	require.Len(t, DefaultFields(), 14)
	require.Len(t, fields, 14, "emitted field count must match the documented default")

	// Every value is double-quoted.
	for i, f := range fields {
		require.True(t, strings.HasPrefix(f, `"`) && strings.HasSuffix(f, `"`), "field %d not quoted: %s", i, f)
	}
	// bigip_hostname is at index 1; context_name at 3; action at 12.
	require.Equal(t, `"bigip1"`, fields[1])
	require.Equal(t, `"/Common/vs_app"`, fields[3])
	require.Contains(t, []string{`"Drop"`, `"Reject"`, `"Accept"`, `"Accept-Decisively"`}, fields[12])
}

func TestRegistered(t *testing.T) {
	_, ok := catalog.Get("afm")
	require.True(t, ok)
}
