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

// Ctx carries the per-line context a product's Build func needs: the
// emission time and the emitting device hostname. All randomness comes
// from the supplied *rand.Rand so output is deterministic from seed.
type Ctx struct {
	// Now is the timestamp for the emitted record.
	Now time.Time
	// Hostname is the simulated F5 device hostname.
	Hostname string
}

// Product is one F5 product's log emitter. Name is the config token
// (e.g. "ltm", "asm"). Build returns one syslog-shaped log line for the
// product, drawing all randomness from r.
type Product struct {
	Name  string
	Build func(r *rand.Rand, c *Ctx) string
}
