package catalog

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSyslogHeader(t *testing.T) {
	now := time.Date(2026, 9, 24, 15, 4, 5, 0, time.UTC)
	got := SyslogHeader(134, now, "bigip1", "tmm", 4211)
	require.Equal(t, "<134>Sep 24 15:04:05 bigip1 tmm[4211]:", got)
}

func TestSyslogHeaderNoPID(t *testing.T) {
	now := time.Date(2026, 9, 24, 15, 4, 5, 0, time.UTC)
	got := SyslogHeader(134, now, "bigip1", "mcpd", 0)
	require.Equal(t, "<134>Sep 24 15:04:05 bigip1 mcpd:", got)
}
