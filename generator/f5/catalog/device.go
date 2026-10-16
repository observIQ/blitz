package catalog

import (
	"math/rand"

	"github.com/observiq/blitz/internal/datagen"
)

// daemonPIDRange is each simulated daemon's PID floor and span. tmm and the
// BIG-IP control-plane daemons start well after boot, while F5OS runs
// audit-service as a low-numbered container process (rSeries samples show
// audit-service[12]).
var daemonPIDRange = map[string][2]int{
	"tmm":           {10000, 30000},
	"mcpd":          {4000, 3000},
	"apmd":          {10000, 10000},
	"httpd":         {10000, 20000},
	"audit-service": {8, 10},
	"nginx":         {1000, 60000},
}

// Device is the simulated F5 deployment's stable identity: the BIG-IP unit's
// management IP and one PID per long-running daemon, so consecutive lines
// from the same host agree with each other. Each daemon runs on one host
// (see Host): nginx on the NGINX server, audit-service on the F5OS
// platform, and the rest on the BIG-IP unit. It is built once per generator
// and never mutated, which is what lets every worker read it without a lock.
type Device struct {
	// MgmtIP is the unit's management-port address.
	MgmtIP string
	pids   map[string]int
}

// NewDevice draws a Device from r, so a fixed seed yields the same unit.
func NewDevice(r *rand.Rand) *Device {
	d := &Device{MgmtIP: datagen.RandomPrivateIPv4(r), pids: make(map[string]int, len(daemonPIDRange))}
	// Iterate in a fixed order so the draw sequence, and so the PIDs, are
	// deterministic for a seed (map iteration order is not).
	for _, name := range []string{"tmm", "mcpd", "apmd", "httpd", "audit-service", "nginx"} {
		rng := daemonPIDRange[name]
		d.pids[name] = rng[0] + r.Intn(rng[1])
	}
	return d
}

// PID returns the stable process ID of a known daemon. An unknown daemon name
// is a programmer error in a product, so it panics rather than inventing one.
func (d *Device) PID(daemon string) int {
	pid, ok := d.pids[daemon]
	if !ok {
		panic("f5 catalog: no PID for daemon " + daemon)
	}
	return pid
}
