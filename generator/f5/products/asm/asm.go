// Package asm registers the BIG-IP ASM (Application Security Manager)
// product: WAF security events / attack signatures.
//
// The ASM remote-logging Storage Format is operator-configurable (Field-List /
// User-Defined), so there is no single positional wire spec. This models F5's
// documented DEFAULT syslog field set, in the documented order, emitted as
// comma-separated key="value" pairs. The full field catalog is GUI-only and not
// published as an ordered reference.
//
// Reference (validated 2026-09-25):
//
//	https://techdocs.f5.com/en-us/bigip-17-5-0/big-ip-asm-implementations/logging-application-security-events.html
//	(Logging Application Security Events — default syslog format field set.)
package asm

import (
	"fmt"
	"math/rand"
	"strings"

	"github.com/observiq/blitz/generator/f5/catalog"
	"github.com/observiq/blitz/internal/datagen"
)

func init() { catalog.Register(catalog.Product{Name: "asm", Build: build}) }

// defaultFields is F5's documented default ASM syslog field set, in order.
var defaultFields = []string{
	"rejection_description",
	"request_violation",
	"support_id",
	"source_ip",
	"xff_ip",
	"source_port",
	"destination_ip",
	"destination_port",
	"route_domain",
	"http_classifier",
	"scheme",
	"geographic_location",
	"request",
	"username",
	"session_id",
	"violation_rating",
}

// DefaultFields returns the documented default field order (for tests).
func DefaultFields() []string { return append([]string(nil), defaultFields...) }

var (
	violations = []string{"Attack signature detected", "Illegal meta character in value", "Illegal URL length", "Illegal HTTP method"}
	statuses   = []string{"blocked", "alerted", "passed"}
	classes    = []string{"/Common/prod_waf_policy", "/Common/api_protection", "/Common/owasp_top10"}
	geos       = []string{"US", "GB", "DE", "CN"}
	schemes    = []string{"https", "http"}
)

func build(r *rand.Rand, c *catalog.Ctx) string {
	status := statuses[r.Intn(len(statuses))]
	rejection := "N/A"
	if status == "blocked" {
		rejection = "Request was blocked"
	}
	vals := map[string]string{
		"rejection_description": rejection,
		"request_violation":     violations[r.Intn(len(violations))],
		"support_id":            fmt.Sprintf("%d", 1000000000000000+r.Int63n(8999999999999999)),
		"source_ip":             datagen.RandomPublicIPv4(r),
		"xff_ip":                datagen.RandomPublicIPv4(r),
		"source_port":           fmt.Sprintf("%d", 1024+r.Intn(64000)),
		"destination_ip":        datagen.RandomPrivateIPv4(r),
		"destination_port":      "443",
		"route_domain":          "0",
		"http_classifier":       classes[r.Intn(len(classes))],
		"scheme":                schemes[r.Intn(len(schemes))],
		"geographic_location":   geos[r.Intn(len(geos))],
		"request":               "GET /api/v1/orders?id=1%27%20or%20%271%27=%271 HTTP/1.1",
		"username":              "N/A",
		"session_id":            fmt.Sprintf("%x", r.Uint64()),
		"violation_rating":      fmt.Sprintf("%d", 1+r.Intn(5)),
	}

	header := catalog.SyslogHeader(134, c.Now, c.Hostname, "ASM", 0)
	parts := make([]string, len(defaultFields))
	for i, name := range defaultFields {
		parts[i] = name + `="` + vals[name] + `"`
	}
	return header + " " + strings.Join(parts, ",")
}
