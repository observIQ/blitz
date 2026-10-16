// Package catalog holds the F5 product registry. Each per-product
// subpackage under generator/f5/products registers one Product at init
// time via Register; the top-level f5 generator reads them back to emit
// a weighted mix of F5 log lines. Adding a product is a matter of a new
// subpackage that self-registers — no change to the generator core.
package catalog

import (
	"math/rand"
	"time"
)

// Ctx carries the context a product's Build func needs: the emission time,
// the emitting device, and per-worker product state. A worker reuses one Ctx
// for all its lines, updating Now each time. All randomness comes from the
// supplied *rand.Rand so output is deterministic from seed.
type Ctx struct {
	// Now is the timestamp for the emitted record.
	Now time.Time
	// Hostname is the hostname of the machine the product logs from.
	Hostname string
	// FQDN is that machine's fully qualified name; empty when it has none.
	FQDN string
	// Device is the unit's stable identity, shared read-only by all workers.
	Device *Device

	state map[string]any
}

// FullName is the name a BIG-IP writes where it uses its fully qualified name
// (the DNS query log body, AFM's header and bigip_hostname): FQDN, or
// Hostname when there is none.
func (c *Ctx) FullName() string {
	if c.FQDN != "" {
		return c.FQDN
	}
	return c.Hostname
}

// State returns the state a product keeps across its lines (for example APM's
// open sessions), creating it with init on first use. Each worker owns its
// Ctx, so the map needs no lock.
func (c *Ctx) State(key string, init func() any) any {
	if c.state == nil {
		c.state = map[string]any{}
	}
	v, ok := c.state[key]
	if !ok {
		v = init()
		c.state[key] = v
	}
	return v
}

// Host is the machine a product's logs come from. BIG-IP modules run on the
// BIG-IP unit; NGINX Plus and App Protect run on separate NGINX hosts; F5OS
// audit logs come from the rSeries platform the BIG-IP tenant runs on.
type Host int

const (
	// HostBIGIP is the BIG-IP unit (the default).
	HostBIGIP Host = iota
	// HostNGINX is an NGINX Plus server.
	HostNGINX
	// HostPlatform is the F5OS platform.
	HostPlatform
)

// Product is one F5 product's log emitter. Name is the config token
// (e.g. "asm", "afm"). Host is the machine it logs from. Build returns one
// syslog-shaped log line for the product, drawing all randomness from r; the
// generator sets c.Hostname to the product's host before calling it.
type Product struct {
	Name  string
	Host  Host
	Build func(r *rand.Rand, c *Ctx) string
}
