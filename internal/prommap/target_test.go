package prommap

import (
	"testing"

	"github.com/observiq/blitz/embed"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func gaugeWithResource(res map[string]any) embed.MetricPoint {
	return embed.MetricPoint{
		Name:     "system.memory.usage",
		Type:     embed.MetricTypeGauge,
		IntValue: i64(42),
		Metadata: embed.MetricPointMetadata{
			Timestamp:  ts(),
			Attributes: map[string]string{"state": "used"},
			Resource:   res,
		},
	}
}

// Distinct hosts must produce distinct series: host.name becomes instance and
// telemetry.source becomes job.
func TestMap_ResourceBecomesJobAndInstance(t *testing.T) {
	fam, err := Map(gaugeWithResource(map[string]any{
		"host.name":        "athena",
		"telemetry.source": "hostmetrics",
	}))
	require.NoError(t, err)
	assert.Equal(t, []Label{
		{"instance", "athena"},
		{"job", "hostmetrics"},
		{"state", "used"},
	}, fam.Samples[0].Labels)
}

func TestMap_DistinctHostsDistinctSeries(t *testing.T) {
	a, err := Map(gaugeWithResource(map[string]any{"host.name": "athena", "telemetry.source": "hostmetrics"}))
	require.NoError(t, err)
	b, err := Map(gaugeWithResource(map[string]any{"host.name": "hermes", "telemetry.source": "hostmetrics"}))
	require.NoError(t, err)
	assert.NotEqual(t, a.Samples[0].Labels, b.Samples[0].Labels)
}

// service.* takes precedence per the OTel-to-Prometheus compatibility spec.
func TestMap_ServiceAttributesTakePrecedence(t *testing.T) {
	fam, err := Map(gaugeWithResource(map[string]any{
		"service.namespace":   "shop",
		"service.name":        "cart",
		"service.instance.id": "cart-7",
		"host.name":           "athena",
		"telemetry.source":    "hostmetrics",
	}))
	require.NoError(t, err)
	assert.Equal(t, []Label{
		{"instance", "cart-7"},
		{"job", "shop/cart"},
		{"state", "used"},
	}, fam.Samples[0].Labels)
}

// No resource means no job/instance labels (existing behavior preserved).
func TestMap_NoResourceNoTargetLabels(t *testing.T) {
	fam, err := Map(gaugeWithResource(nil))
	require.NoError(t, err)
	assert.Equal(t, []Label{{"state", "used"}}, fam.Samples[0].Labels)
}

func TestTargetInfo(t *testing.T) {
	fam, ok := TargetInfo(gaugeWithResource(map[string]any{
		"host.name":        "athena",
		"telemetry.source": "hostmetrics",
		"os.type":          "linux",
		"host.ip":          []string{"10.0.0.1", "10.0.0.2"},
	}))
	require.True(t, ok)
	assert.Equal(t, "target_info", fam.Name)
	assert.Equal(t, TypeGauge, fam.Type)
	require.Len(t, fam.Samples, 1)
	s := fam.Samples[0]
	assert.Equal(t, "target_info", s.Name)
	assert.Equal(t, 1.0, s.Value)
	assert.Equal(t, tsMS, s.TimestampMS)
	assert.Equal(t, []Label{
		{"host_ip", `["10.0.0.1","10.0.0.2"]`},
		{"host_name", "athena"},
		{"instance", "athena"},
		{"job", "hostmetrics"},
		{"os_type", "linux"},
		{"telemetry_source", "hostmetrics"},
	}, s.Labels)
}

// service.* keys feed job/instance and are excluded from target_info's labels.
func TestTargetInfo_ExcludesServiceKeys(t *testing.T) {
	fam, ok := TargetInfo(gaugeWithResource(map[string]any{
		"service.name":        "cart",
		"service.instance.id": "cart-7",
		"os.type":             "linux",
	}))
	require.True(t, ok)
	assert.Equal(t, []Label{
		{"instance", "cart-7"},
		{"job", "cart"},
		{"os_type", "linux"},
	}, fam.Samples[0].Labels)
}

func TestTargetInfo_NoResource(t *testing.T) {
	_, ok := TargetInfo(gaugeWithResource(nil))
	assert.False(t, ok)
}

// _count must equal the le="+Inf" bucket even when HistogramCount disagrees
// with the bucket counts (Prometheus invariant).
func TestMap_Histogram_CountMatchesInf(t *testing.T) {
	fam, err := Map(embed.MetricPoint{
		Name:                  "latency",
		Type:                  embed.MetricTypeHistogram,
		HistogramBucketBounds: []float64{1, 5},
		HistogramBucketCounts: []uint64{2, 3, 4},
		HistogramCount:        99,
		HistogramSum:          20,
		Metadata:              embed.MetricPointMetadata{Timestamp: ts()},
	})
	require.NoError(t, err)
	var inf float64
	for _, s := range fam.Samples {
		if s.Name == "latency_bucket" && s.Labels[len(s.Labels)-1].Value == "+Inf" {
			inf = s.Value
		}
	}
	assert.Equal(t, 9.0, inf)
	assert.Equal(t, 9.0, find(t, fam, "latency_count").Value)
}
