package promscrape

import (
	"maps"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/jonboulle/clockwork"

	"github.com/observiq/blitz/embed"
	"github.com/observiq/blitz/internal/prommap"
)

// store keeps the latest point per series, as the collector
// prometheusexporter's accumulator does: a series is the metric name and
// type, its attributes, and its resource; an older point never replaces a
// newer one; a series not updated within expiry is dropped (0 keeps it).
type store struct {
	clock  clockwork.Clock
	expiry time.Duration

	mu        sync.Mutex
	entries   map[string]stored
	lastSweep time.Time
}

type stored struct {
	mp      embed.MetricPoint
	updated time.Time
}

func newStore(clk clockwork.Clock, expiry time.Duration) *store {
	return &store{clock: clk, expiry: expiry, entries: make(map[string]stored)}
}

// put stores a copy of mp unless the series already holds a later point.
func (s *store) put(mp embed.MetricPoint) {
	mp.Metadata.Attributes = maps.Clone(mp.Metadata.Attributes)
	mp.Metadata.Resource = maps.Clone(mp.Metadata.Resource)
	mp.HistogramBucketBounds = slices.Clone(mp.HistogramBucketBounds)
	mp.HistogramBucketCounts = slices.Clone(mp.HistogramBucketCounts)
	k := signature(mp)
	now := s.clock.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	if old, ok := s.entries[k]; ok && mp.Metadata.Timestamp.Before(old.mp.Metadata.Timestamp) {
		return
	}
	s.entries[k] = stored{mp: mp, updated: now}
	// Sweep here too, at most once per expiry window, so the map stays
	// bounded when nothing scrapes.
	if s.expiry > 0 && now.Sub(s.lastSweep) >= s.expiry {
		s.sweep(now)
	}
}

// snapshot drops expired series and returns the rest.
func (s *store) snapshot() []embed.MetricPoint {
	now := s.clock.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.expiry > 0 {
		s.sweep(now)
	}
	out := make([]embed.MetricPoint, 0, len(s.entries))
	for _, e := range s.entries {
		out = append(out, e.mp)
	}
	return out
}

// sweep drops series not updated within expiry. The caller holds mu.
func (s *store) sweep(now time.Time) {
	s.lastSweep = now
	for k, e := range s.entries {
		if now.Sub(e.updated) > s.expiry {
			delete(s.entries, k)
		}
	}
}

// signature identifies a series: name, type, attributes, and resource.
func signature(mp embed.MetricPoint) string {
	var b strings.Builder
	b.WriteString(mp.Name)
	b.WriteByte(0)
	b.WriteString(string(mp.Type))
	for _, k := range slices.Sorted(maps.Keys(mp.Metadata.Attributes)) {
		b.WriteByte(0)
		b.WriteString(k)
		b.WriteByte('=')
		b.WriteString(mp.Metadata.Attributes[k])
	}
	b.WriteByte(1)
	for _, k := range slices.Sorted(maps.Keys(mp.Metadata.Resource)) {
		b.WriteByte(0)
		b.WriteString(k)
		b.WriteByte('=')
		b.WriteString(prommap.AttrString(mp.Metadata.Resource[k]))
	}
	return b.String()
}
