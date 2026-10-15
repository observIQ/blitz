package audit

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

// golden holds real BIG-IP audit lines from F5's f5-ansible issues #438 and
// #1677 (device syslog pasted by users), without their <PRI>.
func golden(t *testing.T) []string {
	t.Helper()
	b, err := os.ReadFile("testdata/golden.log")
	require.NoError(t, err)
	return strings.Split(strings.TrimRight(string(b), "\n"), "\n")
}

func TestRenderMatchesGolden(t *testing.T) {
	g := golden(t)
	at := func(mo time.Month, d, h, m, s int, y int) time.Time { return time.Date(y, mo, d, h, m, s, 0, time.UTC) }
	host := "bigip2-v13"
	require.Equal(t, "<134>"+g[0], renderPAM(at(5, 28, 12, 36, 36, 2018), host, "rest", 29606, "admin", "(unknown)", at(5, 28, 12, 36, 36, 2018)))
	require.Equal(t, "<133>"+g[1], renderMCPD(at(5, 28, 12, 36, 38, 2018), host, 5428, "tmsh, tmsh-pid-29617", "admin", 40045, 3,
		"create", `app_template { app_template_name "/Common/web_frontends_option_default_route_domain_true" app_template_config_source 0 }`))
	require.Equal(t, "<133>"+g[2], renderTmsh(at(5, 28, 12, 36, 38, 2018), host, 29617, "admin", "/Common",
		"load sys application template /var/config/rest/downloads/web_frontends_option_default_route_domain_true"))
	require.Equal(t, "<133>"+g[3], renderMCPD(at(3, 4, 17, 59, 26, 2018), "ip-10-100-1-98.eu-west-1.compute.internal", 4324, "tmui", "my-admin", 728301, 3,
		"create", `virtual_server_policy { virtual_server_policy_vs_name "/Common/main_HTTP" virtual_server_policy_policy_name "/Common/main_policy" }`))
}

var shapes = []*regexp.Regexp{
	regexp.MustCompile(`^<133>\w{3} [ \d]\d \d\d:\d\d:\d\d bigip1 notice mcpd\[\d+\]: 01070417:5: AUDIT - client (tmui|Unknown|tmsh, tmsh-pid-\d+), user \S+ - transaction #\d+-\d+ - object 0 - (create|modify) \{ .+ \} \[Status=Command OK\]$`),
	regexp.MustCompile(`^<133>\w{3} [ \d]\d \d\d:\d\d:\d\d bigip1 notice tmsh\[(\d+)\]: 01420002:5: AUDIT - pid=(\d+) user=\S+ folder=\S+ module=\(tmos\)# status=\[Command OK\] cmd_data=.+$`),
	regexp.MustCompile(`^<134>\w{3} [ \d]\d \d\d:\d\d:\d\d bigip1 info (httpd|sshd|rest)\(pam_audit\)\[\d+\]: 01070417:6: AUDIT - user (\S+) - RAW: (httpd|sshd|rest)\(pam_audit\): user=\S+\(\S+\) partition=\[All\] level=Administrator tty=\S+ host=\S+ attempts=1 start="[^"]+" end="[^"]+"\.$`),
}

func TestBuildShapesAndOnlyRealCodes(t *testing.T) {
	dev := catalog.NewDevice(rand.New(rand.NewSource(2)))
	c := &catalog.Ctx{Now: time.Date(2026, 9, 24, 15, 4, 5, 0, time.UTC), Hostname: "bigip1", Device: dev}
	hit := make([]bool, len(shapes))
	for seed := int64(0); seed < 200; seed++ {
		line := build(rand.New(rand.NewSource(seed)), c)
		matched := false
		for i, re := range shapes {
			if m := re.FindStringSubmatch(line); m != nil {
				matched, hit[i] = true, true
				if i == 1 {
					require.Equal(t, m[1], m[2], "tmsh pid= must match its tag PID")
				}
			}
		}
		require.True(t, matched, line)
		if strings.Contains(line, " mcpd[") {
			require.Contains(t, line, " mcpd["+strconv.Itoa(dev.PID("mcpd"))+"]:", "mcpd PID stable per device")
		}
	}
	require.Equal(t, []bool{true, true, true}, hit)
}

// TestRestLoginsHaveNoClientHost pins the golden rest login: iControl REST
// does not know the client, so host=(unknown); sshd and httpd name an address.
func TestRestLoginsHaveNoClientHost(t *testing.T) {
	c := &catalog.Ctx{Now: time.Date(2026, 9, 24, 15, 4, 5, 0, time.UTC), Hostname: "bigip1", Device: catalog.NewDevice(rand.New(rand.NewSource(2)))}
	host := regexp.MustCompile(`RAW: (\w+)\(pam_audit\):.* host=(\S+) `)
	seen := map[string]bool{}
	for seed := int64(0); seed < 300; seed++ {
		m := host.FindStringSubmatch(build(rand.New(rand.NewSource(seed)), c))
		if m == nil {
			continue
		}
		seen[m[1]] = true
		if m[1] == "rest" {
			require.Equal(t, "(unknown)", m[2])
		} else {
			require.Regexp(t, `^\d+\.\d+\.\d+\.\d+$`, m[2], m[1])
		}
	}
	require.True(t, seen["rest"] && seen["sshd"] && seen["httpd"], "%v", seen)
}

func TestRegistered(t *testing.T) {
	_, ok := catalog.Get("audit")
	require.True(t, ok)
}
