package promscrape

import (
	"strings"
	"sync"
	"time"

	"github.com/jonboulle/clockwork"

	"github.com/observiq/blitz/internal/prommap"
)

// registry holds the latest MetricFamily per series identity. WriteMetric
// upserts; the scrape handler snapshots. It is bounded by distinct series
// (updates overwrite), not by write count, and a series not updated within
// expiry is dropped, so churned or stopped hosts stop being exposed.
type registry struct {
	clock  clockwork.Clock
	expiry time.Duration // 0 keeps series forever

	mu      sync.Mutex
	entries map[string]regEntry
}

type regEntry struct {
	fam      prommap.MetricFamily
	lastSeen time.Time
}

func newRegistry(clk clockwork.Clock, expiry time.Duration) *registry {
	return &registry{clock: clk, expiry: expiry, entries: make(map[string]regEntry)}
}

// upsert stores fam under its identity key, replacing any prior value and
// refreshing its expiry.
func (r *registry) upsert(fam prommap.MetricFamily) {
	k := familyKey(fam)
	now := r.clock.Now()
	r.mu.Lock()
	r.entries[k] = regEntry{fam: fam, lastSeen: now}
	r.mu.Unlock()
}

// snapshot drops expired series and returns a copy of the rest for encoding.
func (r *registry) snapshot() []prommap.MetricFamily {
	now := r.clock.Now()
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]prommap.MetricFamily, 0, len(r.entries))
	for k, e := range r.entries {
		if r.expiry > 0 && now.Sub(e.lastSeen) > r.expiry {
			delete(r.entries, k)
			continue
		}
		out = append(out, e.fam)
	}
	return out
}

// len reports the current series count.
func (r *registry) len() int {
	r.mu.Lock()
	n := len(r.entries)
	r.mu.Unlock()
	return n
}

// familyKey is name + type + base labels. Base labels are the family's sample
// labels excluding the synthetic "le" (identical across a family's samples), so
// same-name families with different attributes stay distinct.
func familyKey(fam prommap.MetricFamily) string {
	var b strings.Builder
	b.WriteString(fam.Name)
	b.WriteByte('\x00')
	b.WriteString(string(fam.Type))
	if len(fam.Samples) > 0 {
		for _, l := range fam.Samples[0].Labels {
			if l.Name == "le" {
				continue
			}
			b.WriteByte('\x00')
			b.WriteString(l.Name)
			b.WriteByte('=')
			b.WriteString(l.Value)
		}
	}
	return b.String()
}
