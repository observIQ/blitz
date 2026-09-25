// Package audit registers the BIG-IP audit product: configuration audit
// records emitted by mcpd across modules.
//
// Byte-exact to the documented mcpd AUDIT message template:
//
//	<errcode>:<level>: AUDIT - client <client>, user <user> - transaction #<txn>-<seq> - <command> <object>
//
// Reference (validated 2026-09-25):
//
//	https://techdocs.f5.com/en-us/bigip-17-5-0/external-monitoring-of-big-ip-systems-implementations.html
//	(BIG-IP audit logging — mcpd AUDIT record format.)
package audit

import (
	"fmt"
	"math/rand"

	"github.com/observiq/blitz/generator/f5/catalog"
)

func init() { catalog.Register(catalog.Product{Name: "audit", Build: build}) }

var (
	clients = []string{"tmsh", "GUI", "iControl REST", "httpd(mod_auth_pam)"}
	users   = []string{"admin", "operator", "svc_automation", "auditor"}
	cmds    = []string{"create", "modify", "delete", "list"}
	objects = []string{
		"ltm virtual /Common/vs_https_443",
		"ltm pool /Common/pool_web",
		"security firewall policy /Common/fw_policy",
		"auth user operator",
		"sys ntp",
	}
	codes = []string{"01070417", "01071031", "01070734", "01420002"}
)

func build(r *rand.Rand, c *catalog.Ctx) string {
	header := catalog.SyslogHeader(133, c.Now, c.Hostname, "mcpd", 4000+r.Intn(2000))
	code := codes[r.Intn(len(codes))]
	client := clients[r.Intn(len(clients))]
	user := users[r.Intn(len(users))]
	cmd := cmds[r.Intn(len(cmds))]
	object := objects[r.Intn(len(objects))]
	txn := 1000 + r.Intn(90000)
	// Documented mcpd AUDIT template.
	return fmt.Sprintf("%s %s:5: AUDIT - client %s, user %s - transaction #%d-1 - %s %s",
		header, code, client, user, txn, cmd, object)
}
