// Package irules registers the iRules logging product: operator-emitted
// log lines from custom TCL rules.
//
// iRules log output is fully operator-defined (arbitrary TCL `log` statements),
// so no wire spec exists. This emits realistic instances of common operator
// log lines in the tmm "Rule /Common/<rule> <EVENT>:" framing that the log
// command adds (config-dependent), plus tmm's real 01220001 TCL runtime error.
// Both ride BIG-IP's syslog-ng header.
//
// References (validated 2026-09-29):
//
//	https://clouddocs.f5.com/api/irules/log.html (rule name and event prefix)
//	splunk/splunk-connect-for-syslog tests/test_f5_bigip.py (01220001 TCL error line)
package irules

import (
	"fmt"
	"math/rand"
	"time"

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

// tclErrors are commands an iRule can call in an event that does not
// support them, with that event.
var tclErrors = []struct{ event, cmd string }{
	{"HTTP_REQUEST", "HTTP::method"},
	{"CLIENT_ACCEPTED", "HTTP::uri"},
	{"HTTP_RESPONSE", "HTTP::host"},
}

func build(r *rand.Rand, c *catalog.Ctx) string {
	tag, pid := catalog.TmmTag(r), c.Device.PID("tmm")
	rule := rules[r.Intn(len(rules))]
	if r.Intn(20) == 0 { // an occasional runtime error
		e := tclErrors[r.Intn(len(tclErrors))]
		return renderTCLError(c.Now, c.Hostname, tag, pid, rule, e.event, 1+r.Intn(40), e.cmd)
	}
	header := catalog.BigIPHeader(catalog.PRI(catalog.Local0, 6), c.Now, c.Hostname, tag, pid)
	e := events[r.Intn(len(events))]
	msg := e.render(datagen.RandomPublicIPv4(r), 1024+r.Intn(64000))
	return fmt.Sprintf("%s Rule /Common/%s <%s>: %s", header, rule, e.name, msg)
}

// renderTCLError renders tmm's 01220001 TCL runtime error (local0.err).
func renderTCLError(now time.Time, host, tag string, pid int, rule, event string, line int, cmd string) string {
	return fmt.Sprintf(`%s 01220001:3: TCL error: /Common/%s <%s> - ERR_NOT_SUPPORTED (line %d)     invoked from within "%s"`,
		catalog.BigIPHeader(catalog.PRI(catalog.Local0, 3), now, host, tag, pid), rule, event, line, cmd)
}
