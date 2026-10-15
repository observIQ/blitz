package nginxplus

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

// upstreamTimeFormat is the upstream_time log_format exactly as NGINX
// documents it (docs.nginx.com/nginx/admin-guide/monitoring/logging/),
// including the missing space before rt=, which the docs' string
// concatenation produces.
const upstreamTimeFormat = `$remote_addr - $remote_user [$time_local] ` +
	`"$request" $status $body_bytes_sent ` +
	`"$http_referer" "$http_user_agent"` +
	`rt="$request_time" uct="$upstream_connect_time" uht="$upstream_header_time" urt="$upstream_response_time"`

var varRe = regexp.MustCompile(`\$[a-z_]+`)

// expand substitutes NGINX variables into a log_format string.
func expand(format string, vars map[string]string) string {
	return varRe.ReplaceAllStringFunc(format, func(v string) string { return vars[v[1:]] })
}

func TestAccessMatchesDocumentedFormat(t *testing.T) {
	now := time.Date(2026, 9, 24, 15, 4, 5, 0, time.UTC)
	a := access{RemoteAddr: "203.0.113.9", Request: "GET /health HTTP/1.1", Status: 200, BodyBytes: 512,
		UserAgent: "curl/8.4.0", RT: 0.052, UCT: 0.001, UHT: 0.040, URT: 0.050}
	want := "<190>Sep 24 15:04:05 nginxp1 nginx: " + expand(upstreamTimeFormat, map[string]string{
		"remote_addr": "203.0.113.9", "remote_user": "-", "time_local": "24/Sep/2026:15:04:05 +0000",
		"request": "GET /health HTTP/1.1", "status": "200", "body_bytes_sent": "512",
		"http_referer": "-", "http_user_agent": "curl/8.4.0", "request_time": "0.052",
		"upstream_connect_time": "0.001", "upstream_header_time": "0.040", "upstream_response_time": "0.050",
	})
	require.Equal(t, want, renderAccess(now, "nginxp1", a))
}

// TestErrorMatchesGolden renders a real NGINX error-log line (Elastic's nginx
// integration fixture, packages/nginx/data_stream/error/_dev/test/pipeline/
// test-error-raw.log) behind NGINX's syslog header. The fixture's build has
// no thread support, so it logs thread id 0.
func TestErrorMatchesGolden(t *testing.T) {
	b, err := os.ReadFile("testdata/error_golden.log")
	require.NoError(t, err)
	golden := strings.TrimRight(string(b), "\n")
	e := errEvent{Level: "error", PID: 54053, TID: 0, Conn: 1,
		Msg:    `open() "/usr/local/Cellar/nginx/1.10.2_1/html/favicon.ico" failed (2: No such file or directory)`,
		Client: "127.0.0.1", Server: "localhost", Request: "GET /favicon.ico HTTP/1.1",
		Host: "localhost:8080", Referrer: "http://localhost:8080/"}
	now := time.Date(2016, 10, 25, 14, 49, 34, 0, time.UTC)
	require.Equal(t, "<187>Oct 25 14:49:34 web1 nginx: "+golden, renderError(now, "web1", e))
}

var accessRe = regexp.MustCompile(`^<190>\w{3} [ \d]\d \d\d:\d\d:\d\d nginxp1 nginx: \S+ - - \[[^\]]+\] "[A-Z]+ \S+ HTTP/1\.1" \d{3} \d+ "-" "[^"]*"rt="([\d.]+)" uct="([\d.]+)" uht="([\d.]+)" urt="([\d.]+)"$`)
var errorRe = regexp.MustCompile(`^<(\d+)>\w{3} [ \d]\d \d\d:\d\d:\d\d nginxp1 nginx: \d{4}/\d\d/\d\d \d\d:\d\d:\d\d \[(error|warn)\] (\d+)#(\d+): \*\d+ `)

func TestBuildCoherent(t *testing.T) {
	c := &catalog.Ctx{Now: time.Date(2026, 9, 24, 15, 4, 5, 0, time.UTC), Hostname: "nginxp1",
		Device: catalog.NewDevice(rand.New(rand.NewSource(1)))}
	pids := map[string]bool{}
	errs := 0
	for seed := int64(0); seed < 400; seed++ {
		line := build(rand.New(rand.NewSource(seed)), c)
		if m := accessRe.FindStringSubmatch(line); m != nil {
			f := func(i int) float64 { v, err := strconv.ParseFloat(m[i], 64); require.NoError(t, err); return v }
			rt, uct, uht, urt := f(1), f(2), f(3), f(4)
			require.LessOrEqual(t, uct, uht, line)
			require.LessOrEqual(t, uht, urt, line)
			require.LessOrEqual(t, urt, rt, line)
			continue
		}
		m := errorRe.FindStringSubmatch(line)
		require.NotNil(t, m, line)
		pri, _ := strconv.Atoi(m[1])
		require.Equal(t, map[string]int{"error": 187, "warn": 188}[m[2]], pri, "PRI follows the level")
		require.Equal(t, map[string]string{"error": "ERROR", "warn": "WARN"}[m[2]], catalog.SeverityFromPRI(line))
		// NGINX builds --with-threads (as the Linux packages do) log the
		// thread id, which for a worker's main thread is its PID.
		require.Equal(t, m[3], m[4], "pid#tid names the worker's main thread")
		pids[m[3]] = true
		errs++
	}
	require.Greater(t, errs, 10)
	require.Len(t, pids, 1, "one stable NGINX worker PID per device")
}

// TestNoLiveUpstreamsNamesTheGroup pins NGINX's upstream field: with no peer
// left, round robin sets the connection's name to the upstream group's
// (ngx_http_upstream_round_robin.c), so the line names the group, while the
// other upstream errors name the peer that failed.
func TestNoLiveUpstreamsNamesTheGroup(t *testing.T) {
	c := &catalog.Ctx{Now: time.Date(2026, 9, 24, 15, 4, 5, 0, time.UTC), Hostname: "nginxp1",
		Device: catalog.NewDevice(rand.New(rand.NewSource(1)))}
	upstream := regexp.MustCompile(`upstream: "http://([^/"]+)/`)
	seen := 0
	for seed := int64(0); seed < 2000; seed++ {
		line := build(rand.New(rand.NewSource(seed)), c)
		m := upstream.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		if strings.Contains(line, "no live upstreams") {
			require.Equal(t, "backend", m[1], line)
			seen++
		} else {
			require.Regexp(t, `^\d+\.\d+\.\d+\.\d+:\d+$`, m[1], line)
		}
	}
	require.Positive(t, seen)
}

func TestRegistered(t *testing.T) {
	_, ok := catalog.Get("nginx-plus")
	require.True(t, ok)
}
