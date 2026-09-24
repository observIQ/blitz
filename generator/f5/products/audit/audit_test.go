package audit

import (
	"math/rand"
	"regexp"
	"testing"
	"time"

	"github.com/observiq/blitz/generator/f5/catalog"
	"github.com/stretchr/testify/require"
)

func TestBuildDeterministic(t *testing.T) {
	c := &catalog.Ctx{Now: time.Date(2026, 9, 24, 15, 4, 5, 0, time.UTC), Hostname: "bigip1"}
	got := build(rand.New(rand.NewSource(5)), c)
	require.Equal(t, got, build(rand.New(rand.NewSource(5)), c))
}

// TestAuditTemplateStructure asserts the emitted line matches the documented
// mcpd AUDIT template segment order.
func TestAuditTemplateStructure(t *testing.T) {
	c := &catalog.Ctx{Now: time.Date(2026, 9, 24, 15, 4, 5, 0, time.UTC), Hostname: "bigip1"}
	got := build(rand.New(rand.NewSource(5)), c)
	re := regexp.MustCompile(`^<133>Sep 24 15:04:05 bigip1 mcpd\[\d+\]: ` +
		`\d{8}:5: AUDIT - client [^,]+, user \S+ - transaction #\d+-1 - (create|modify|delete|list) .+$`)
	require.Regexp(t, re, got)
}

func TestRegistered(t *testing.T) {
	_, ok := catalog.Get("audit")
	require.True(t, ok)
}
