// Package f5os registers the F5OS / TMOS platform product: platform-
// level audit and system events (chassis, tenant, service lifecycle).
//
// F5OS platform logs come from several daemons (confd, systemd, platform)
// with no single positional wire spec. This emits realistic instances of
// default platform audit/system log lines. Config-dependent.
//
// Reference (validated 2026-09-25):
//
//	https://techdocs.f5.com/en-us/f5os-a-1-8-0/f5-rseries-systems-administration-configuration.html
package f5os

import (
	"fmt"
	"math/rand"

	"github.com/observiq/blitz/generator/f5/catalog"
	"github.com/observiq/blitz/internal/datagen"
)

func init() { catalog.Register(catalog.Product{Name: "f5os", Build: build}) }

var users = []string{"admin", "root", "svc_orchestration"}

type ev struct {
	tag    string
	pri    int
	render func(r *rand.Rand, user, ip string) string
}

var events = []ev{
	{"confd", 133, func(_ *rand.Rand, user, ip string) string {
		return fmt.Sprintf("audit - user: %s from %s - command: set tenant tenant1 config state deployed", user, ip)
	}},
	{"systemd", 150, func(_ *rand.Rand, _, _ string) string {
		return "Started F5 platform tenant tenant1.service"
	}},
	{"platform", 147, func(r *rand.Rand, _, _ string) string {
		return fmt.Sprintf("chassis partition 1 blade 1: temperature nominal (%dC)", 30+r.Intn(15))
	}},
	{"velos-controller", 148, func(_ *rand.Rand, _, _ string) string {
		return "tenant tenant1 transitioned Running -> Deployed"
	}},
	{"sshd", 134, func(r *rand.Rand, user, ip string) string {
		return fmt.Sprintf("Accepted publickey for %s from %s port %d ssh2", user, ip, 1024+r.Intn(64000))
	}},
	{"confd", 132, func(_ *rand.Rand, user, ip string) string {
		return fmt.Sprintf("audit - user: %s from %s - command: delete interfaces interface 2.0", user, ip)
	}},
}

func build(r *rand.Rand, c *catalog.Ctx) string {
	e := events[r.Intn(len(events))]
	header := catalog.SyslogHeader(e.pri, c.Now, c.Hostname, e.tag, 1000+r.Intn(9000))
	msg := e.render(r, users[r.Intn(len(users))], datagen.RandomPrivateIPv4(r))
	return header + " " + msg
}
