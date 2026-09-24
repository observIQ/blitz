package appprotect

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
	c := &catalog.Ctx{Now: time.Date(2026, 9, 24, 15, 4, 5, 0, time.UTC), Hostname: "nap1"}
	got := build(rand.New(rand.NewSource(13)), c)
	require.Equal(t, got, build(rand.New(rand.NewSource(13)), c))
	require.True(t, strings.HasPrefix(got, "<134>Sep 24 15:04:05 nap1 app_protect:"), got)
}

// TestDefaultFieldSetAndOrder asserts the emitted key set and order exactly
// match the documented App Protect "default" security-log attribute list.
func TestDefaultFieldSetAndOrder(t *testing.T) {
	c := &catalog.Ctx{Now: time.Date(2026, 9, 24, 15, 4, 5, 0, time.UTC), Hostname: "nap1"}
	got := build(rand.New(rand.NewSource(1)), c)

	// Strip the syslog header, keep the key="value",... body.
	body := got[strings.Index(got, "app_protect:")+len("app_protect:")+1:]

	// Anchor to field boundaries (start-of-body or a comma) so a `key="`
	// substring inside a value (e.g. version="1.0" in violation_details) is
	// not mistaken for a field.
	keyRe := regexp.MustCompile(`(?:^|,)([a-z0-9_]+)="`)
	matches := keyRe.FindAllStringSubmatch(body, -1)
	keys := make([]string, 0, len(matches))
	for _, m := range matches {
		keys = append(keys, m[1])
	}

	require.Equal(t, DefaultFields(), keys, "emitted field set/order must match the documented default format")
	require.Len(t, keys, 41)
}

func TestRegistered(t *testing.T) {
	_, ok := catalog.Get("nginx-app-protect")
	require.True(t, ok)
}
