package catalog

import (
	"math/rand"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSyslogHeader(t *testing.T) {
	now := time.Date(2026, 9, 24, 15, 4, 5, 0, time.UTC)
	require.Equal(t, "<134>Sep 24 15:04:05 bigip1 tmm[4211]:", SyslogHeader(134, now, "bigip1", "tmm", 4211))
	require.Equal(t, "<134>Sep 24 15:04:05 bigip1 ASM:", SyslogHeader(134, now, "bigip1", "ASM", 0))
}

// TestBigIPHeaderGolden renders the BIG-IP syslog-ng header for lines copied
// from Splunk Connect for Syslog's F5 fixtures (tests/test_f5_bigip.py) and
// F5's f5-ansible issue #438, which carry the severity word after the host.
func TestBigIPHeaderGolden(t *testing.T) {
	cases := []struct {
		golden string
		pri    int
		tag    string
		pid    int
		now    time.Time
		host   string
	}{
		{"Jan 17 04:03:37 SV5-F5-5600-2 warning tmm1[23068]:", PRI(Local0, 4), "tmm1", 23068,
			time.Date(2020, 1, 17, 4, 3, 37, 0, time.UTC), "SV5-F5-5600-2"},
		{"Jan 17 03:38:00 SV5-F5-5600-2 err tmm[23068]:", PRI(Local0, 3), "tmm", 23068,
			time.Date(2020, 1, 17, 3, 38, 0, 0, time.UTC), "SV5-F5-5600-2"},
		{"May 28 12:36:38 bigip2-v13 notice mcpd[5428]:", PRI(Local0, 5), "mcpd", 5428,
			time.Date(2018, 5, 28, 12, 36, 38, 0, time.UTC), "bigip2-v13"},
		{"May 28 12:36:36 bigip2-v13 info rest(pam_audit)[29606]:", PRI(Local0, 6), "rest(pam_audit)", 29606,
			time.Date(2018, 5, 28, 12, 36, 36, 0, time.UTC), "bigip2-v13"},
	}
	for _, c := range cases {
		got := BigIPHeader(c.pri, c.now, c.host, c.tag, c.pid)
		require.Equal(t, "<"+strconv.Itoa(c.pri)+">"+c.golden, got)
	}
}

func TestPRI(t *testing.T) {
	require.Equal(t, 134, PRI(Local0, 6))
	require.Equal(t, 141, PRI(Local1, 5))
	require.Equal(t, 190, PRI(Local7, 6))
	require.Equal(t, 85, PRI(AuthPriv, 5))
}

// TestSeverityFromPRI pins one severity name per syslog level, which the
// syslog output and the OTLP output's syslog table both read.
func TestSeverityFromPRI(t *testing.T) {
	cases := map[string]string{
		"<128>x": "EMERGENCY", "<129>x": "ALERT", "<130>x": "CRITICAL", "<131>x": "ERROR",
		"<132>x": "WARN", "<133>x": "NOTICE", "<134>x": "INFO", "<135>x": "DEBUG",
		"<85>x": "NOTICE", "no pri": "INFO", "<>x": "INFO", "<abc>x": "INFO", "<-1>x": "INFO",
	}
	for line, want := range cases {
		require.Equal(t, want, SeverityFromPRI(line), line)
	}
}

func TestTmmTag(t *testing.T) {
	seen := map[string]bool{}
	r := rand.New(rand.NewSource(1))
	for i := 0; i < 200; i++ {
		seen[TmmTag(r)] = true
	}
	require.Equal(t, map[string]bool{"tmm": true, "tmm1": true, "tmm2": true, "tmm3": true}, seen)
}
