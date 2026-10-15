package apm

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

// golden holds real APM lines posted to DevCentral (threads 169738 and
// 69459), without their <PRI>. The posters masked IPs as x.x.x.x.
func golden(t *testing.T) []string {
	t.Helper()
	b, err := os.ReadFile("testdata/golden.log")
	require.NoError(t, err)
	return strings.Split(strings.TrimRight(string(b), "\n"), "\n")
}

func TestRenderMatchesGolden(t *testing.T) {
	g := golden(t)
	feb := func(h, m, s int) time.Time { return time.Date(2019, 2, 12, h, m, s, 0, time.UTC) }
	nov := func(h, m, s int) time.Time { return time.Date(2014, 11, 21, h, m, s, 0, time.UTC) }
	p1, p2 := "/Common/exchange2016:Common:8014e002", "/Common/exchange2016:Common:36e6238b"
	cases := []string{
		render(feb(16, 11, 31), "F5-TEST", "tmm", 22913, "01490500", p1,
			"New session from client IP x.x.x.x (ST=Ile-de-France/CC=FR/C=EU) at VIP x.x.x.x Listener /Common/VS-EX2016-IQR_https (Reputation=Unknown)"),
		render(feb(16, 11, 33), "F5-TEST", "apmd", 15789, "01490102", p1, "Access policy result: LTM+APM_Mode"),
		render(feb(16, 11, 36), "F5-TEST", "tmm2", 22913, "01490506", p2, "Received User-Agent header: MSRPC."),
		render(nov(18, 3, 9), "vddmz-px13-an", "apd", 10743, "01490005", "6370a13a",
			"Following rule 'fallback' from item 'NTLM Auth Result' to ending 'Deny'"),
		render(nov(18, 3, 9), "vddmz-px13-an", "tmm", 15203, "01490520", "6370a13a", "Session deleted due to admin initiated termination."),
		render(nov(18, 3, 43), "vddmz-px13-an", "tmm", 15203, "01490521", "6370a13a", "Session statistics - bytes in: 0, bytes out: 0"),
	}
	require.Len(t, g, len(cases))
	for i, got := range cases {
		require.Equal(t, "<141>"+g[i], got)
	}
}

var lineRe = regexp.MustCompile(`^<141>\w{3} [ \d]\d \d\d:\d\d:\d\d bigip1 notice (tmm\d?|apmd)\[(\d+)\]: (0149\w{4}):5: /Common/\w+:Common:([0-9a-f]{8}): `)

// allowed and denied are the message sequences one session walks through.
var (
	allowed = []string{"01490506", "01490500", "01490010", "01490005", "01490102", "01490248", "01490520", "01490521"}
	denied  = []string{"01490506", "01490500", "01490010", "01490005", "01490102", "01490520", "01490521"}
)

// TestSessionsPairCreateAndDelete follows every session ID and requires its
// lines in lifecycle order: created (01490500) before deleted (01490520)
// before its closing statistics (01490521), with nothing after.
func TestSessionsPairCreateAndDelete(t *testing.T) {
	dev := catalog.NewDevice(rand.New(rand.NewSource(5)))
	c := &catalog.Ctx{Now: time.Date(2026, 9, 24, 15, 4, 5, 0, time.UTC), Hostname: "bigip1", Device: dev}
	r := rand.New(rand.NewSource(8))
	seen := map[string][]string{}
	for i := 0; i < 600; i++ {
		line := build(r, c)
		m := lineRe.FindStringSubmatch(line)
		require.NotNil(t, m, line)
		daemon := "apmd"
		if strings.HasPrefix(m[1], "tmm") {
			daemon = "tmm"
		}
		require.Equal(t, strconv.Itoa(dev.PID(daemon)), m[2], "stable %s PID", daemon)
		seen[m[4]] = append(seen[m[4]], m[3])
	}
	closed := 0
	for sid, ids := range seen {
		want := allowed
		if len(ids) > 5 && ids[5] != "01490248" {
			want = denied
		}
		require.Equal(t, want[:len(ids)], ids, "session %s out of order", sid)
		if len(ids) == len(want) {
			closed++
		}
	}
	require.Greater(t, closed, 20)
}

func TestRegistered(t *testing.T) {
	_, ok := catalog.Get("apm")
	require.True(t, ok)
}
