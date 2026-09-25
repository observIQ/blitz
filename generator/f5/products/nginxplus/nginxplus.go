// Package nginxplus registers the NGINX-on-F5 (NGINX Plus) product:
// access and error logs.
//
// The access log is byte-exact to NGINX's documented DEFAULT "combined"
// log_format, extended with the documented NGINX Plus upstream timing
// variables (request_time + upstream_connect_time / upstream_header_time /
// upstream_response_time / upstream_addr) that distinguish Plus from the
// community nginx generator. The error log follows NGINX's default error_log
// line format.
//
// References (validated 2026-09-25):
//
//	https://docs.nginx.com/nginx/admin-guide/monitoring/logging/
//	(combined default format; $upstream_*/$request_time timing variables.)
package nginxplus

import (
	"fmt"
	"math/rand"

	"github.com/observiq/blitz/generator/f5/catalog"
	"github.com/observiq/blitz/internal/datagen"
)

func init() { catalog.Register(catalog.Product{Name: "nginx-plus", Build: build}) }

// combinedVars is the ordered variable list of NGINX's default "combined"
// log_format:
//
//	$remote_addr - $remote_user [$time_local] "$request" $status
//	$body_bytes_sent "$http_referer" "$http_user_agent"
//
// plusExtraVars are the documented NGINX Plus upstream timing fields appended.
var (
	combinedVars = []string{
		"remote_addr", "remote_user", "time_local", "request",
		"status", "body_bytes_sent", "http_referer", "http_user_agent",
	}
	plusExtraVars = []string{"request_time", "upstream_connect_time", "upstream_header_time", "upstream_response_time", "upstream_addr"}
)

// AccessFormatVars returns the full ordered variable list of the access line
// (combined + Plus extras), for tests.
func AccessFormatVars() []string {
	return append(append([]string(nil), combinedVars...), plusExtraVars...)
}

var (
	methods    = []string{"GET", "POST", "PUT", "DELETE"}
	uris       = []string{"/", "/api/v2/users", "/assets/main.css", "/health", "/checkout"}
	statuses   = []int{200, 200, 201, 301, 404, 502, 504}
	agents     = []string{"Mozilla/5.0", "curl/8.4.0", "kube-probe/1.29"}
	errLevels  = []string{"error", "warn", "crit"}
	errReasons = []string{
		"upstream timed out (110: Connection timed out) while reading response header from upstream",
		"connect() failed (111: Connection refused) while connecting to upstream",
		"no live upstreams while connecting to upstream",
	}
)

func build(r *rand.Rand, c *catalog.Ctx) string {
	if r.Intn(10) == 0 { // ~10% error log
		return errorLine(r, c)
	}
	return accessLine(r, c)
}

// accessLine emits the combined format followed by the Plus upstream fields.
func accessLine(r *rand.Rand, c *catalog.Ctx) string {
	header := catalog.SyslogHeader(158, c.Now, c.Hostname, "nginx", 0)
	remoteAddr := datagen.RandomPublicIPv4(r)
	request := fmt.Sprintf("%s %s HTTP/1.1", methods[r.Intn(len(methods))], uris[r.Intn(len(uris))])
	status := statuses[r.Intn(len(statuses))]
	bodyBytes := 100 + r.Intn(40000)
	ua := agents[r.Intn(len(agents))]
	upstreamAddr := fmt.Sprintf("%s:%d", datagen.RandomPrivateIPv4(r), []int{8080, 8443, 9000}[r.Intn(3)])

	// combined: $remote_addr - $remote_user [$time_local] "$request" $status $body_bytes_sent "$http_referer" "$http_user_agent"
	combined := fmt.Sprintf(`%s - - [%s] "%s" %d %d "-" "%s"`,
		remoteAddr, c.Now.Format("02/Jan/2006:15:04:05 -0700"), request, status, bodyBytes, ua)
	// Plus upstream timing extension.
	plus := fmt.Sprintf(`rt=%.3f uct="%.3f" uht="%.3f" urt="%.3f" upstream_addr=%s`,
		r.Float64(), r.Float64()/10, r.Float64()/5, r.Float64(), upstreamAddr)
	return header + " " + combined + " " + plus
}

func errorLine(r *rand.Rand, c *catalog.Ctx) string {
	header := catalog.SyslogHeader(155, c.Now, c.Hostname, "nginx", 0)
	// Default error_log line: <date> [<level>] <pid>#<tid>: *<cid> <msg>, client: ..., server: ..., request: ..., upstream: ...
	return fmt.Sprintf(`%s %s [%s] %d#%d: *%d %s, client: %s, server: app.example.com, request: "GET / HTTP/1.1", upstream: "http://%s:8080/"`,
		header, c.Now.Format("2006/01/02 15:04:05"), errLevels[r.Intn(len(errLevels))], 1000+r.Intn(9000), r.Intn(64),
		r.Intn(1000000), errReasons[r.Intn(len(errReasons))], datagen.RandomPublicIPv4(r), datagen.RandomPrivateIPv4(r))
}
