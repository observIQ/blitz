package f5os

import (
	"math/rand"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/observiq/blitz/generator/f5/catalog"
)

// golden holds real rSeries F5OS audit.log lines from F5's rSeries training
// guide (clouddocs.f5.com/training/community/rseries-training/html/
// rseries_security.html), read from the file, so without their <PRI>.
func golden(t *testing.T) []string {
	t.Helper()
	b, err := os.ReadFile("testdata/golden.log")
	require.NoError(t, err)
	return strings.Split(strings.TrimRight(string(b), "\n"), "\n")
}

var goldenRe = regexp.MustCompile(`^(\S+) (\S+) audit-service\[(\d+)\]: priority="(\w+)" version=1\.0 msgid=0x1f0300000000000(\d) msg="audit" user="(\S+)" cmd="(.*)"\.$`)

// TestRenderMatchesGolden re-renders each golden line from its parsed parts.
// The parse only lifts the variable values out; the fixed text, the level-to-
// msgid pairing, and the PRI all come from render.
func TestRenderMatchesGolden(t *testing.T) {
	for _, g := range golden(t) {
		m := goldenRe.FindStringSubmatch(g)
		require.NotNil(t, m, g)
		ts, err := time.Parse("2006-01-02T15:04:05.000000-07:00", m[1])
		require.NoError(t, err)
		info := m[4] == "Info"
		require.Equal(t, map[bool]string{true: "1", false: "2"}[info], m[5], "Info pairs with msgid ...01, Notice with ...02")
		want := map[bool]string{true: "<86>", false: "<85>"}[info] + g
		require.Equal(t, want, render(ts, m[2], 12, info, m[6], m[7]))
	}
}

var lineRe = regexp.MustCompile(`^<(85|86)>\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d\.\d{6}[+-]\d\d:\d\d bigip1 audit-service\[(\d+)\]: priority="(Notice|Info)" version=1\.0 msgid=0x1f0300000000000([12]) msg="audit" user="(\w+/\d+)" cmd="(.*)"\.$`)

// TestSessionsAreCoherent follows each user/session ID: it opens with the
// group assignment and "created new session", and closes with
// "terminated session" (a REST session logs its response just after).
func TestSessionsAreCoherent(t *testing.T) {
	dev := catalog.NewDevice(rand.New(rand.NewSource(6)))
	c := &catalog.Ctx{Now: time.Date(2026, 9, 24, 15, 4, 5, 0, time.UTC), Hostname: "bigip1", Device: dev}
	r := rand.New(rand.NewSource(3))
	cmds := map[string][]string{}
	for i := 0; i < 500; i++ {
		line := build(r, c)
		m := lineRe.FindStringSubmatch(line)
		require.NotNil(t, m, line)
		require.Equal(t, map[string]string{"85": "Notice", "86": "Info"}[m[1]], m[3])
		require.Equal(t, map[string]string{"Notice": "2", "Info": "1"}[m[3]], m[4])
		require.Equal(t, strconv.Itoa(dev.PID("audit-service")), m[2])
		cmds[m[5]] = append(cmds[m[5]], m[6])
	}
	finished := 0
	for id, got := range cmds {
		require.Equal(t, "assigned to groups: admin", got[0], id)
		if len(got) > 1 {
			require.True(t, strings.HasPrefix(got[1], "created new session via "), id)
		}
		for i, cmd := range got {
			if cmd == "terminated session (reason: normal)" {
				finished++
				rest := got[i+1:]
				require.LessOrEqual(t, len(rest), 1, "only a REST response may follow termination: %v", got)
			}
		}
	}
	require.Greater(t, finished, 10)
}

func TestRegistered(t *testing.T) {
	_, ok := catalog.Get("f5os")
	require.True(t, ok)
}
