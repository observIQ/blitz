// Package audit registers the BIG-IP audit product: configuration and login
// audit records.
//
// Three real record shapes, each under its own real AUDIT code:
//
//	mcpd 01070417:5  AUDIT - client <c>, user <u> - transaction #<t>-<s> - object 0 - <verb> { <class> { ... } } [Status=Command OK]
//	tmsh 01420002:5  AUDIT - pid=<p> user=<u> folder=<f> module=(tmos)# status=[Command OK] cmd_data=<cmd>
//	pam  01070417:6  AUDIT - user <u> - RAW: <daemon>(pam_audit): user=<u>(<u>) partition=[All] ... start="<t>" end="<t>".
//
// A tmsh client shows its PID in the mcpd record ("tmsh, tmsh-pid-N") and
// tags its own 01420002 record with the same PID.
//
// Sources (validated 2026-09-29):
//
//	F5Networks/f5-ansible issues #438 and #1677 (real device syslog; testdata/golden.log)
//	splunk/splunk-connect-for-syslog tests/test_f5_bigip.py (mcpd, tmsh, and pam_audit fixtures)
//	F5Networks/f5-telemetry-streaming test/unit/tests/data/messageStreamTestsData.js (pam_audit at <134>)
package audit

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/observiq/blitz/generator/f5/catalog"
	"github.com/observiq/blitz/internal/datagen"
)

func init() { catalog.Register(catalog.Product{Name: "audit", Build: build}) }

var (
	users = []string{"admin", "root", "operator"}
	vss   = []string{"/Common/vs_https_443", "/Common/main_HTTP", "/Common/vs_api_8443"}
	// bodies are mcpd object bodies in the real class-and-attribute shape;
	// %s is a virtual server name.
	bodies = []string{
		`virtual_server_policy { virtual_server_policy_vs_name "%s" virtual_server_policy_policy_name "/Common/main_policy" }`,
		`virtual_server_profile { virtual_server_profile_vs_name "%s" virtual_server_profile_profile_name "/Common/websecurity" virtual_server_profile_profile_context 0 }`,
	}
	templates = []string{"f5.http", "base-network-config", "web_frontends_option_default_route_domain_true"}
	// pamDaemons are the services pam_audit reports logins for.
	pamDaemons = []string{"httpd", "sshd", "rest"}
)

func build(r *rand.Rand, c *catalog.Ctx) string {
	user := users[r.Intn(len(users))]
	switch r.Intn(3) {
	case 0: // a GUI or unattributed configuration change
		client := []string{"tmui", "Unknown"}[r.Intn(2)]
		body := fmt.Sprintf(bodies[r.Intn(len(bodies))], vss[r.Intn(len(vss))])
		return renderMCPD(c.Now, c.Hostname, c.Device.PID("mcpd"), client, user,
			10000+r.Intn(900000), 2+r.Intn(4), []string{"create", "modify"}[r.Intn(2)], body)
	case 1: // tmsh loading a template: the tmsh command record, or mcpd's view of it
		pid := 10000 + r.Intn(30000) // tmsh is a per-session CLI process, so its PID varies
		tmpl := templates[r.Intn(len(templates))]
		if r.Intn(2) == 0 {
			return renderTmsh(c.Now, c.Hostname, pid, user, "/Common",
				"load sys application template /var/config/rest/downloads/"+tmpl)
		}
		return renderMCPD(c.Now, c.Hostname, c.Device.PID("mcpd"), fmt.Sprintf("tmsh, tmsh-pid-%d", pid), user,
			10000+r.Intn(900000), 3, "create",
			fmt.Sprintf(`app_template { app_template_name "/Common/%s" app_template_config_source 0 }`, tmpl))
	default: // a login
		daemon := pamDaemons[r.Intn(len(pamDaemons))]
		from := datagen.RandomPrivateIPv4(r)
		if daemon == "rest" {
			from = "(unknown)" // iControl REST does not know the client
		}
		pid := 10000 + r.Intn(30000)
		if daemon == "httpd" {
			pid = c.Device.PID("httpd")
		}
		return renderPAM(c.Now, c.Hostname, daemon, pid, user, from, c.Now.Add(-time.Duration(r.Intn(3600))*time.Second))
	}
}

// renderMCPD renders an mcpd configuration-transaction record (local0.notice).
func renderMCPD(now time.Time, host string, pid int, client, user string, txn, seq int, verb, body string) string {
	return fmt.Sprintf("%s 01070417:5: AUDIT - client %s, user %s - transaction #%d-%d - object 0 - %s { %s } [Status=Command OK]",
		catalog.BigIPHeader(catalog.PRI(catalog.Local0, 5), now, host, "mcpd", pid), client, user, txn, seq, verb, body)
}

// renderTmsh renders a tmsh command record (local0.notice).
func renderTmsh(now time.Time, host string, pid int, user, folder, cmd string) string {
	return fmt.Sprintf("%s 01420002:5: AUDIT - pid=%d user=%s folder=%s module=(tmos)# status=[Command OK] cmd_data=%s",
		catalog.BigIPHeader(catalog.PRI(catalog.Local0, 5), now, host, "tmsh", pid), pid, user, folder, cmd)
}

// renderPAM renders a pam_audit login record (local0.info). from is the
// client host, "(unknown)" when the service does not know it.
func renderPAM(now time.Time, host, daemon string, pid int, user, from string, start time.Time) string {
	tty := map[string]string{"httpd": "/sbin/nologin", "sshd": "ssh", "rest": "(unknown)"}[daemon]
	return fmt.Sprintf(`%s 01070417:6: AUDIT - user %s - RAW: %s(pam_audit): user=%s(%s) partition=[All] level=Administrator tty=%s host=%s attempts=1 start="%s" end="%s".`,
		catalog.BigIPHeader(catalog.PRI(catalog.Local0, 6), now, host, daemon+"(pam_audit)", pid),
		user, daemon, user, user, tty, from, start.Format(time.ANSIC), now.Format(time.ANSIC))
}
