package appprotect

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
	"github.com/observiq/blitz/generator/f5/waf"
)

// goldenLines are two real App Protect security-log lines (default format),
// copied from F5's NGINX Gateway Fabric lab
// (f5devcentral/NGINX-Gateway-Fabric-Lab, labs/11.waf/README.md at 00c6cae).
// They come from a syslog file, so they carry no <PRI>.
func goldenLines(t *testing.T) []string {
	t.Helper()
	b, err := os.ReadFile("testdata/golden.log")
	require.NoError(t, err)
	return strings.Split(strings.TrimRight(string(b), "\n"), "\n")
}

// TestRenderMatchesGolden renders each golden line's own values and expects
// the golden line back byte for byte.
func TestRenderMatchesGolden(t *testing.T) {
	golden := goldenLines(t)
	cases := []event{
		{Now: time.Date(2026, 9, 3, 7, 39, 40, 0, time.UTC), Host: "gateway-nginx-68d68854d7-4rfrf",
			ClientIP: "192.168.2.26", SrcPort: 55426, DestPort: 80, SupportID: 7582683759784056240, S: waf.XSSParameter},
		{Now: time.Date(2026, 9, 3, 7, 41, 14, 0, time.UTC), Host: "gateway-nginx-68d68854d7-4rfrf",
			ClientIP: "192.168.2.26", SrcPort: 42994, DestPort: 80, SupportID: 7624501502690286169, S: waf.DataGuardLeak},
	}
	require.Len(t, golden, len(cases))
	for i, e := range cases {
		require.Equal(t, "<134>"+golden[i], render(e))
	}
}

var kvRe = regexp.MustCompile(`([a-z_]+)="((?:[^"]|"")*)"(?:,|$)`)

func fields(line string) map[string]string {
	body := line[strings.Index(line, "ASM:")+len("ASM:"):]
	out := map[string]string{}
	for _, m := range kvRe.FindAllStringSubmatch(body, -1) {
		out[m[1]] = m[2]
	}
	return out
}

// TestBuildIsCoherent checks drawn lines: the request, json_log, and verdict
// fields must agree with each other.
func TestBuildIsCoherent(t *testing.T) {
	c := &catalog.Ctx{Now: time.Date(2026, 9, 24, 15, 4, 5, 0, time.UTC), Hostname: "bigip1"}
	for seed := int64(0); seed < 50; seed++ {
		line := build(rand.New(rand.NewSource(seed)), c)
		require.True(t, strings.HasPrefix(line, "<134>Sep 24 15:04:05 bigip1 ASM:attack_type="), line)
		f := fields(line)
		require.Len(t, f, 39)
		require.Equal(t, "false", f["is_truncated"])
		require.Equal(t, "bigip1", f["unit_hostname"])
		_, err := strconv.ParseUint(f["support_id"], 10, 64)
		require.NoError(t, err)
		require.Contains(t, f["json_log"], `""id"":""`+f["support_id"]+`""`)
		require.Contains(t, f["json_log"], `""clientPort"":`+f["src_port"]+`,`)
		require.True(t, strings.HasPrefix(f["request"], f["method"]+" "), f["request"])
		switch f["request_status"] {
		case "blocked":
			require.Equal(t, "REJECTED", f["outcome"])
			require.Equal(t, "0", f["response_code"])
		case "alerted":
			require.Equal(t, "PASSED", f["outcome"])
		default:
			t.Fatalf("unexpected request_status %q", f["request_status"])
		}
		require.True(t, strings.HasPrefix(f["violation_details"], "<?xml version='1.0' encoding='UTF-8'?>"))
	}
}

func TestRegistered(t *testing.T) {
	_, ok := catalog.Get("nginx-app-protect")
	require.True(t, ok)
}
