// Package nginxplus registers the NGINX-on-F5 (NGINX Plus) product: access
// and error logs sent over syslog.
//
// The access line is NGINX's documented upstream_time log_format, byte for
// byte: combined, then rt/uct/uht/urt with every value quoted and no space
// before rt= (the documented string concatenation omits it). Timings are
// coherent: uct <= uht <= urt <= rt. The error line is NGINX's standard
// error-log line, message text taken from the NGINX source, with its level
// setting the syslog severity. Both go out under NGINX's default facility,
// local7; access lines at the default access severity, info.
//
// Sources (validated 2026-09-29):
//
//	https://docs.nginx.com/nginx/admin-guide/monitoring/logging/ (upstream_time format)
//	https://nginx.org/en/docs/syslog.html (facility local7, access severity info, tag nginx)
//	nginx/nginx src/core/ngx_log.c, src/core/ngx_syslog.c (error line, severity = level, header)
//	elastic/integrations packages/nginx/data_stream/error/_dev/test/pipeline/test-error-raw.log
//	(real error line; testdata/error_golden.log)
package nginxplus

import (
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/observiq/blitz/generator/f5/catalog"
	"github.com/observiq/blitz/internal/datagen"
)

func init() {
	catalog.Register(catalog.Product{Name: "nginx-plus", Host: catalog.HostNGINX, Build: build})
}

// access holds an access line's variable values.
type access struct {
	RemoteAddr, Request, UserAgent string
	Status, BodyBytes              int
	// RT is $request_time; UCT, UHT, and URT are the upstream connect,
	// header, and response times.
	RT, UCT, UHT, URT float64
}

// errEvent holds an error line's values. Upstream and Referrer are omitted
// from the line when empty, as NGINX does.
type errEvent struct {
	Level             string
	PID, TID, Conn    int
	Msg               string
	Client, Server    string
	Request, Upstream string
	Host, Referrer    string
}

var (
	methods  = []string{"GET", "POST", "PUT", "DELETE"}
	uris     = []string{"/", "/api/v2/users", "/assets/main.css", "/health", "/checkout"}
	statuses = []int{200, 200, 201, 301, 404, 502, 504}
	agents   = []string{"Mozilla/5.0", "curl/8.4.0", "kube-probe/1.29"}
	// errMsgs are NGINX upstream error messages (ngx_http_upstream.c,
	// ngx_event_pipe.c) with the level NGINX logs each at.
	errMsgs = []struct{ level, msg string }{
		{"error", "upstream timed out (110: Connection timed out) while reading response header from upstream"},
		{"error", "connect() failed (111: Connection refused) while connecting to upstream"},
		{"error", "no live upstreams while connecting to upstream"},
		{"warn", "an upstream response is buffered to a temporary file /var/cache/nginx/proxy_temp/1/00/0000000001 while reading upstream"},
	}
	// upstreamGroup is the upstream block the proxied requests go to.
	upstreamGroup = "backend"
	// severities maps an NGINX level to its syslog severity.
	severities = map[string]int{"error": 3, "warn": 4}
)

func build(r *rand.Rand, c *catalog.Ctx) string {
	if r.Intn(10) == 0 { // about one line in ten is an error
		m := errMsgs[r.Intn(len(errMsgs))]
		uri := uris[r.Intn(len(uris))]
		up := fmt.Sprintf("%s:%d", datagen.RandomPrivateIPv4(r), []int{8080, 8443, 9000}[r.Intn(3)])
		if strings.HasPrefix(m.msg, "no live upstreams") {
			up = upstreamGroup // no peer is left, so NGINX names the group
		}
		// Linux NGINX builds --with-threads log the thread id after the PID;
		// for a worker's main thread it is the PID.
		pid := c.Device.PID("nginx")
		return renderError(c.Now, c.Hostname, errEvent{
			Level: m.level, PID: pid, TID: pid, Conn: 1 + r.Intn(1000000), Msg: m.msg,
			Client: datagen.RandomPublicIPv4(r), Server: "app.example.com",
			Request: "GET " + uri + " HTTP/1.1", Upstream: "http://" + up + uri, Host: "app.example.com",
		})
	}
	// Each phase ends after the one before it: connect, first header byte,
	// last response byte, then the request's own end.
	uct := float64(r.Intn(20)) / 1000
	uht := uct + float64(r.Intn(300))/1000
	urt := uht + float64(r.Intn(200))/1000
	rt := urt + float64(r.Intn(5))/1000
	return renderAccess(c.Now, c.Hostname, access{
		RemoteAddr: datagen.RandomPublicIPv4(r),
		Request:    fmt.Sprintf("%s %s HTTP/1.1", methods[r.Intn(len(methods))], uris[r.Intn(len(uris))]),
		Status:     statuses[r.Intn(len(statuses))], BodyBytes: 100 + r.Intn(40000),
		UserAgent: agents[r.Intn(len(agents))], RT: rt, UCT: uct, UHT: uht, URT: urt,
	})
}

// renderAccess renders the upstream_time access line at local7.info.
func renderAccess(now time.Time, host string, a access) string {
	return fmt.Sprintf(`%s %s - - [%s] "%s" %d %d "-" "%s"rt="%.3f" uct="%.3f" uht="%.3f" urt="%.3f"`,
		catalog.SyslogHeader(catalog.PRI(catalog.Local7, 6), now, host, "nginx", 0),
		a.RemoteAddr, now.Format("02/Jan/2006:15:04:05 -0700"), a.Request, a.Status, a.BodyBytes,
		a.UserAgent, a.RT, a.UCT, a.UHT, a.URT)
}

// renderError renders an error line at local7, severity from its level.
func renderError(now time.Time, host string, e errEvent) string {
	line := fmt.Sprintf(`%s %s [%s] %d#%d: *%d %s, client: %s, server: %s, request: "%s"`,
		catalog.SyslogHeader(catalog.PRI(catalog.Local7, severities[e.Level]), now, host, "nginx", 0),
		now.Format("2006/01/02 15:04:05"), e.Level, e.PID, e.TID, e.Conn, e.Msg, e.Client, e.Server, e.Request)
	if e.Upstream != "" {
		line += fmt.Sprintf(`, upstream: "%s"`, e.Upstream)
	}
	line += fmt.Sprintf(`, host: "%s"`, e.Host)
	if e.Referrer != "" {
		line += fmt.Sprintf(`, referrer: "%s"`, e.Referrer)
	}
	return line
}
