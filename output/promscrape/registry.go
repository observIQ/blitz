package promscrape

import (
	"strings"
	"sync"

	"github.com/observiq/blitz/internal/prommap"
)

// registry holds the latest MetricFamily per series identity. WriteMetric
// upserts; the scrape handler snapshots. It is bounded by distinct series
// (updates overwrite), not by write count.
type registry struct {
	mu   sync.Mutex
	fams map[string]prommap.MetricFamily
}

func newRegistry() *registry {
	return &registry{fams: make(map[string]prommap.MetricFamily)}
}

// upsert stores fam under its identity key, replacing any prior value.
func (r *registry) upsert(fam prommap.MetricFamily) {
	k := familyKey(fam)
	r.mu.Lock()
	r.fams[k] = fam
	r.mu.Unlock()
}

// snapshot returns a copy of the current families for encoding.
func (r *registry) snapshot() []prommap.MetricFamily {
	r.mu.Lock()
	out := make([]prommap.MetricFamily, 0, len(r.fams))
	for _, f := range r.fams {
		out = append(out, f)
	}
	r.mu.Unlock()
	return out
}

// len reports the current series count.
func (r *registry) len() int {
	r.mu.Lock()
	n := len(r.fams)
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
