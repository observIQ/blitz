package asm

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

// golden is a real ASM remote-logging line from F5's own
// f5-telemetry-streaming test fixtures
// (test/unit/tests/data/messageStreamTestsData.js).
func golden(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile("testdata/golden.log")
	require.NoError(t, err)
	return strings.TrimRight(string(b), "\n")
}

func TestRenderMatchesGolden(t *testing.T) {
	e := event{
		Now: time.Date(2021, 5, 4, 11, 1, 56, 0, time.UTC), HeaderHost: "localhost.localdomain",
		UnitHost: "bigip1", MgmtIP: "192.168.2.1", Policy: "/Common/ASMTestPolicy",
		PolicyApplyDate: "2021-04-29 14:20:42", SupportID: 2508780119460416236,
		Status: "passed", ResponseCode: 0, ClientIP: "192.168.2.2", Method: "OPTIONS", Protocol: "HTTP",
		Path: "/hello", Query: "param=tomatoes", DateTime: time.Date(2021, 5, 4, 11, 1, 55, 0, time.UTC),
		Severity: "Informational", Geo: "N/A", SessionID: "0", SrcPort: 56022, DestPort: 7878,
		DestIP: "192.168.2.3", Rating: 0, VSName: "/Common/testvs", HostHeader: "192.168.2.3:7878",
		Headers: []string{"User-Agent: curl/7.64.1", "Accept: */*", "Content-Type: application/json", "token: 12341234"},
	}
	require.Equal(t, golden(t), render(e))
}

var kvRe = regexp.MustCompile(`([a-z_0-9]+)="([^"]*)"(?:,|$)`)

func keysAndFields(line string) ([]string, map[string]string) {
	body := line[strings.Index(line, "ASM:")+len("ASM:"):]
	var keys []string
	f := map[string]string{}
	for _, m := range kvRe.FindAllStringSubmatch(body, -1) {
		keys = append(keys, m[1])
		f[m[1]] = m[2]
	}
	return keys, f
}

func TestBuildIsCoherent(t *testing.T) {
	goldenKeys, _ := keysAndFields(golden(t))
	dev := catalog.NewDevice(rand.New(rand.NewSource(9)))
	c := &catalog.Ctx{Now: time.Date(2026, 9, 24, 15, 4, 5, 0, time.UTC), Hostname: "bigip1", Device: dev}
	statuses := map[string]bool{}
	for seed := int64(0); seed < 80; seed++ {
		line := build(rand.New(rand.NewSource(seed)), c)
		require.True(t, strings.HasPrefix(line, "<134>Sep 24 15:04:05 bigip1 ASM:unit_hostname="), line)
		keys, f := keysAndFields(line)
		require.Equal(t, goldenKeys, keys)
		require.Equal(t, dev.MgmtIP, f["management_ip_address"])
		_, err := strconv.ParseUint(f["support_id"], 10, 64)
		require.NoError(t, err, f["support_id"])
		target := f["uri"]
		if f["query_string"] != "" {
			target += "?" + f["query_string"]
		}
		require.True(t, strings.HasPrefix(f["request"], f["method"]+" "+target+" HTTP/1.1\\r\\n"), f["request"])
		statuses[f["request_status"]] = true
		switch f["request_status"] {
		case "passed":
			require.Empty(t, f["violations"])
			require.Equal(t, "0", f["violation_rating"])
			require.Equal(t, "Informational", f["severity"])
		case "blocked":
			require.NotEmpty(t, f["violations"])
			require.Equal(t, "0", f["response_code"])
		case "alerted":
			require.NotEmpty(t, f["violations"])
		}
	}
	require.Equal(t, map[string]bool{"passed": true, "blocked": true, "alerted": true}, statuses)
}

func TestRegistered(t *testing.T) {
	_, ok := catalog.Get("asm")
	require.True(t, ok)
}

// TestRenderEscapesQuotes pins the Splunk key-value format's escaping: a
// double quote inside a value is written as \" (F5 bug 1054297 states the
// format requirement), so the value cannot end early.
func TestRenderEscapesQuotes(t *testing.T) {
	e := event{Now: time.Date(2026, 9, 29, 10, 10, 47, 0, time.UTC), Path: `/a.htm"l`, Method: "GET", Protocol: "HTTP/1.1"}
	line := render(e)
	require.Contains(t, line, `uri="/a.htm\"l"`)
	require.Contains(t, line, `GET /a.htm\"l HTTP/1.1`)
}
