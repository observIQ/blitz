// Package f5os registers the F5OS platform product: audit-service records
// from an F5OS-A (rSeries) or F5OS-C (VELOS) system.
//
// Lines follow F5OS's audit.log record, sent under authpriv (K000135458):
//
//	<ts> <host> audit-service[<pid>]: priority="<Notice|Info>" version=1.0 msgid=0x1f0300000000000<2|1> msg="audit" user="<user>/<usid>" cmd="<event>".
//
// Session events are Notice (msgid ...02); commands are Info (...01). Each
// user session runs as it does on a real system: group assignment, "created
// new session", its commands, then "terminated session" (a REST session's
// response record lands just after the termination, as in the samples).
//
// Sources (validated 2026-09-29):
//
//	https://clouddocs.f5.com/training/community/rseries-training/html/rseries_security.html
//	(rSeries audit.log lines; testdata/golden.log)
//	https://my.f5.com/manage/s/article/K000135458 (audit.log is sent under authpriv)
package f5os

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/observiq/blitz/generator/f5/catalog"
	"github.com/observiq/blitz/internal/datagen"
)

func init() {
	catalog.Register(catalog.Product{Name: "f5os", Host: catalog.HostPlatform, Build: build})
}

var (
	users = []string{"admin", "operator"}
	// cliCommands and restPaths are commands seen in rSeries audit logs.
	cliCommands = []string{"show system state hostname", "file show log/system/audit.log", "config", "commit"}
	restPaths   = []string{"/restconf/", "/restconf/data/openconfig-vlan:vlans", "/restconf/data/openconfig-interfaces:interfaces", "/restconf/data/openconfig-system:system/aaa"}
)

// session is one F5OS user session's remaining audit events.
type session struct {
	user   string
	events []event
}

type event struct {
	info bool
	cmd  string
}

const maxOpen = 3

func build(r *rand.Rand, c *catalog.Ctx) string {
	open := c.State("f5os", func() any { return &[]*session{} }).(*[]*session)
	if len(*open) == 0 || (len(*open) < maxOpen && r.Intn(4) == 0) {
		*open = append(*open, newSession(r))
	}
	i := r.Intn(len(*open))
	s := (*open)[i]
	e := s.events[0]
	s.events = s.events[1:]
	if len(s.events) == 0 {
		*open = append((*open)[:i], (*open)[i+1:]...)
	}
	return render(c.Now, c.Hostname, c.Device.PID("audit-service"), e.info, s.user, e.cmd)
}

// newSession scripts a CLI or REST session from login to termination.
func newSession(r *rand.Rand) *session {
	s := &session{user: fmt.Sprintf("%s/%d", users[r.Intn(len(users))], 10000000+r.Intn(9000000))}
	from := datagen.RandomPrivateIPv4(r)
	notice := func(cmd string) { s.events = append(s.events, event{false, cmd}) }
	info := func(cmd string) { s.events = append(s.events, event{true, cmd}) }

	notice("assigned to groups: admin")
	if r.Intn(2) == 0 {
		notice(fmt.Sprintf("created new session via cli from %s:%d with ssh", from, 1024+r.Intn(64512)))
		for n := 1 + r.Intn(3); n > 0; n-- {
			info(fmt.Sprintf("CLI '%s'", cliCommands[r.Intn(len(cliCommands))]))
			info("CLI done")
		}
		notice("terminated session (reason: normal)")
		return s
	}
	path := restPaths[r.Intn(len(restPaths))]
	notice(fmt.Sprintf("created new session via rest from %s:0 with http", from))
	info(fmt.Sprintf("RESTCONF: request with http: GET %s HTTP/1.1", path))
	notice("terminated session (reason: normal)")
	info(fmt.Sprintf("RESTCONF: response with http: HTTP/1.1 %s 200 duration %d ms", path, 1000+r.Intn(90000)))
	return s
}

// render renders one audit-service record: Info commands at authpriv.info
// with msgid ...01, Notice session events at authpriv.notice with ...02.
func render(ts time.Time, host string, pid int, info bool, user, cmd string) string {
	prio, sev, msgid := "Notice", 5, 2
	if info {
		prio, sev, msgid = "Info", 6, 1
	}
	return fmt.Sprintf(`<%d>%s %s audit-service[%d]: priority="%s" version=1.0 msgid=0x1f0300000000000%d msg="audit" user="%s" cmd="%s".`,
		catalog.PRI(catalog.AuthPriv, sev), ts.Format("2006-01-02T15:04:05.000000-07:00"), host, pid, prio, msgid, user, cmd)
}
