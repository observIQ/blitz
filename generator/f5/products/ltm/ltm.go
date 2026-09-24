// Package ltm registers the BIG-IP LTM (Local Traffic Manager) product:
// request logs from the TMM data plane.
//
// LTM request logging uses an operator-defined Request Logging profile
// format string, so there is no fixed positional wire spec. This emits a
// realistic instance of a common default request-logging profile
// (combined-style with virtual-server/pool context). Config-dependent.
//
// Reference (validated 2026-09-25):
//
//	https://techdocs.f5.com/en-us/bigip-17-5-0/big-ip-ltm-implementations/configuring-request-logging.html
package ltm

import (
	"fmt"
	"math/rand"

	"github.com/observiq/blitz/generator/f5/catalog"
	"github.com/observiq/blitz/internal/datagen"
)

func init() { catalog.Register(catalog.Product{Name: "ltm", Build: build}) }

var methods = []string{"GET", "POST", "PUT", "DELETE", "HEAD"}
var uris = []string{"/", "/index.html", "/api/v1/orders", "/login", "/static/app.js", "/health", "/cart/checkout"}
var statuses = []int{200, 200, 200, 301, 302, 404, 500, 503}
var vips = []string{"/Common/vs_https_443", "/Common/vs_http_80", "/Common/vs_api_8443"}
var agents = []string{
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
	"curl/8.4.0",
	"python-requests/2.31.0",
}

func build(r *rand.Rand, c *catalog.Ctx) string {
	client := datagen.RandomPublicIPv4(r)
	vip := vips[r.Intn(len(vips))]
	method := methods[r.Intn(len(methods))]
	uri := uris[r.Intn(len(uris))]
	status := statuses[r.Intn(len(statuses))]
	bytes := 200 + r.Intn(50000)
	ua := agents[r.Intn(len(agents))]
	pool := "/Common/pool_web"
	member := fmt.Sprintf("%s:%d", datagen.RandomPrivateIPv4(r), []int{80, 443, 8080}[r.Intn(3)])

	header := catalog.SyslogHeader(134, c.Now, c.Hostname, "tmm", 4000+r.Intn(2000))
	// F5 LTM request-logging profile: combined-log body plus virtual/pool context.
	return fmt.Sprintf(`%s %s - - [%s] "%s %s HTTP/1.1" %d %d "-" "%s" vs=%s pool=%s member=%s`,
		header, client, c.Now.Format("02/Jan/2006:15:04:05 -0700"), method, uri, status, bytes, ua, vip, pool, member)
}
