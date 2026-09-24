// Package irules registers the iRules logging product: operator-emitted
// log lines from custom TCL rules.
//
// iRules log output is fully operator-defined (arbitrary TCL `log` statements),
// so no wire spec exists. This emits realistic instances of common operator
// log lines in the tmm "Rule /Common/<rule> <EVENT>:" framing. Config-dependent.
//
// Reference (validated 2026-09-25):
//
//	https://clouddocs.f5.com/api/irules/log.html
package irules

import (
	"fmt"
	"math/rand"

	"github.com/observiq/blitz/generator/f5/catalog"
	"github.com/observiq/blitz/internal/datagen"
)

func init() { catalog.Register(catalog.Product{Name: "irules", Build: build}) }

var rules = []string{"log_http_requests", "maintenance_page", "block_bad_bots", "header_insert", "rate_limit"}

type ev struct {
	name   string
	render func(clientIP string, port int) string
}

var events = []ev{
	{"HTTP_REQUEST", func(ip string, port int) string {
		return fmt.Sprintf("client %s:%d requested /login, inserting X-Forwarded-For", ip, port)
	}},
	{"HTTP_REQUEST", func(ip string, _ int) string {
		return fmt.Sprintf("blocked bad bot from %s (User-Agent matched)", ip)
	}},
	{"CLIENTSSL_HANDSHAKE", func(ip string, port int) string {
		return fmt.Sprintf("TLS handshake complete for %s:%d, cipher ECDHE-RSA-AES256-GCM-SHA384", ip, port)
	}},
	{"LB_SELECTED", func(ip string, _ int) string {
		return fmt.Sprintf("selected pool member for %s", ip)
	}},
	{"HTTP_RESPONSE", func(ip string, _ int) string {
		return fmt.Sprintf("serving maintenance page to %s", ip)
	}},
	{"RULE_INIT", func(_ string, _ int) string {
		return "rule initialized"
	}},
}

func build(r *rand.Rand, c *catalog.Ctx) string {
	header := catalog.SyslogHeader(134, c.Now, c.Hostname, "tmm", 4000+r.Intn(2000))
	rule := rules[r.Intn(len(rules))]
	e := events[r.Intn(len(events))]
	msg := e.render(datagen.RandomPublicIPv4(r), 1024+r.Intn(64000))
	return fmt.Sprintf("%s Rule /Common/%s <%s>: %s", header, rule, e.name, msg)
}
