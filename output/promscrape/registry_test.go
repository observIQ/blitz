package promscrape

import (
	"sync"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/observiq/blitz/internal/prommap"
	"github.com/stretchr/testify/require"
)

func gaugeFam(name, labelVal string, v float64) prommap.MetricFamily {
	return prommap.MetricFamily{
		Name: name,
		Type: prommap.TypeGauge,
		Samples: []prommap.Sample{
			{Name: name, Labels: []prommap.Label{{Name: "host", Value: labelVal}}, Value: v},
		},
	}
}

func TestRegistryUpsertOverwrites(t *testing.T) {
	r := newRegistry(clockwork.NewFakeClock(), 0)
	r.upsert(gaugeFam("cpu", "h1", 1))
	r.upsert(gaugeFam("cpu", "h1", 2))

	snap := r.snapshot()
	require.Len(t, snap, 1)
	require.Equal(t, float64(2), snap[0].Samples[0].Value)
}

func TestRegistryDistinctLabelsAreDistinctSeries(t *testing.T) {
	r := newRegistry(clockwork.NewFakeClock(), 0)
	r.upsert(gaugeFam("cpu", "h1", 1))
	r.upsert(gaugeFam("cpu", "h2", 1))

	require.Len(t, r.snapshot(), 2)
}

func TestRegistryHistogramKeyedByBaseLabels(t *testing.T) {
	hist := func(host string) prommap.MetricFamily {
		return prommap.MetricFamily{
			Name: "lat",
			Type: prommap.TypeHistogram,
			Samples: []prommap.Sample{
				{Name: "lat_bucket", Labels: []prommap.Label{{Name: "host", Value: host}, {Name: "le", Value: "+Inf"}}, Value: 5},
				{Name: "lat_sum", Labels: []prommap.Label{{Name: "host", Value: host}}, Value: 1},
				{Name: "lat_count", Labels: []prommap.Label{{Name: "host", Value: host}}, Value: 5},
			},
		}
	}
	r := newRegistry(clockwork.NewFakeClock(), 0)
	r.upsert(hist("h1"))
	r.upsert(hist("h2"))
	r.upsert(hist("h1"))

	require.Len(t, r.snapshot(), 2)
}

func TestRegistryConcurrent(t *testing.T) {
	r := newRegistry(clockwork.NewFakeClock(), 0)
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func() { defer wg.Done(); r.upsert(gaugeFam("cpu", "h1", 1)) }()
		go func() { defer wg.Done(); _ = r.snapshot() }()
	}
	wg.Wait()
	require.Len(t, r.snapshot(), 1)
}

// A series not updated within the expiry is dropped at snapshot, so churned or
// stopped hosts stop being exposed.
func TestRegistryExpiresStaleSeries(t *testing.T) {
	clk := clockwork.NewFakeClock()
	r := newRegistry(clk, 5*time.Minute)
	r.upsert(gaugeFam("cpu", "old", 1))
	clk.Advance(4 * time.Minute)
	r.upsert(gaugeFam("cpu", "fresh", 1))
	clk.Advance(2 * time.Minute)

	snap := r.snapshot()
	require.Len(t, snap, 1)
	require.Equal(t, "fresh", snap[0].Samples[0].Labels[0].Value)
	require.Equal(t, 1, r.len())
}

// An update refreshes a series' expiry.
func TestRegistryUpdateRefreshesExpiry(t *testing.T) {
	clk := clockwork.NewFakeClock()
	r := newRegistry(clk, 5*time.Minute)
	r.upsert(gaugeFam("cpu", "h1", 1))
	clk.Advance(4 * time.Minute)
	r.upsert(gaugeFam("cpu", "h1", 2))
	clk.Advance(4 * time.Minute)

	require.Len(t, r.snapshot(), 1)
}

// Zero expiry keeps series forever.
func TestRegistryZeroExpiryKeeps(t *testing.T) {
	clk := clockwork.NewFakeClock()
	r := newRegistry(clk, 0)
	r.upsert(gaugeFam("cpu", "h1", 1))
	clk.Advance(24 * time.Hour)

	require.Len(t, r.snapshot(), 1)
}
