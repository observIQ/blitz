// Package apm registers the BIG-IP APM (Access Policy Manager) product:
// access / authentication / SSO logs.
//
// APM logs are per-message-code free-text templates, not a fixed positional
// field set. This emits a realistic instance of the default APM access-log
// messages (MCP-style code + access-policy + session context).
// Config-dependent.
//
// Reference (validated 2026-09-25):
//
//	https://techdocs.f5.com/en-us/bigip-17-5-0/big-ip-access-policy-manager-visual-policy-editor.html
package apm

import (
	"fmt"
	"math/rand"

	"github.com/observiq/blitz/generator/f5/catalog"
	"github.com/observiq/blitz/internal/datagen"
)

func init() { catalog.Register(catalog.Product{Name: "apm", Build: build}) }

var users = []string{"jsmith", "adoe", "svc_app", "contractor1", "admin"}

type event struct {
	code   string
	render func(user, session, clientIP string) string
}

var events = []event{
	{"01490005", func(_, _, _ string) string {
		return "Following rule 'fallback' from item 'Logon Page' to ending 'Allow'"
	}},
	{"01490000", func(_, session, clientIP string) string {
		return fmt.Sprintf("Session %s created from client %s", session, clientIP)
	}},
	{"01490102", func(user, _, _ string) string {
		return fmt.Sprintf("Access policy result: LTM+APM_Mode for user %s", user)
	}},
	{"01490547", func(user, _, _ string) string {
		return fmt.Sprintf("SSO: successful Kerberos SSO for user %s", user)
	}},
	{"01490567", func(user, _, _ string) string {
		return fmt.Sprintf("Session deleted due to user logout for %s", user)
	}},
	{"0149004b", func(user, _, _ string) string {
		return fmt.Sprintf("Authentication failed for user %s (Active Directory)", user)
	}},
}

func build(r *rand.Rand, c *catalog.Ctx) string {
	header := catalog.SyslogHeader(134, c.Now, c.Hostname, "apmd", 4000+r.Intn(2000))
	ev := events[r.Intn(len(events))]
	user := users[r.Intn(len(users))]
	session := fmt.Sprintf("%08x", r.Uint32())
	msg := ev.render(user, session, datagen.RandomPublicIPv4(r))
	// APM: "<code>:5: /Common/<policy>:Common:<session>: <message>"
	return fmt.Sprintf("%s %s:5: /Common/access_policy:Common:%s: %s", header, ev.code, session, msg)
}
