package prommap

import (
	"testing"
	"time"

	"github.com/observiq/blitz/embed"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func i64(v int64) *int64     { return &v }
func f64(v float64) *float64 { return &v }
func ts() time.Time          { return time.Unix(1700000000, 0) }

const tsMS int64 = 1700000000000

// find returns the sample with the given series name, or fails.
func find(t *testing.T, fam MetricFamily, name string) Sample {
	t.Helper()
	for _, s := range fam.Samples {
		if s.Name == name {
			return s
		}
	}
	t.Fatalf("no sample named %q in %+v", name, fam.Samples)
	return Sample{}
}

func TestMap_Gauge(t *testing.T) {
	fam, err := Map(embed.MetricPoint{
		Name:        "system.cpu.utilization",
		Description: "CPU utilization",
		Unit:        "1",
		Type:        embed.MetricTypeGauge,
		DoubleValue: f64(0.5),
		Metadata: embed.MetricPointMetadata{
			Timestamp:  ts(),
			Attributes: map[string]string{"state": "idle", "cpu": "0"},
		},
	})
	require.NoError(t, err)
	assert.Equal(t, "system_cpu_utilization", fam.Name)
	assert.Equal(t, TypeGauge, fam.Type)
	assert.Equal(t, "CPU utilization", fam.Help)
	assert.Equal(t, "1", fam.Unit)
	require.Len(t, fam.Samples, 1)
	s := fam.Samples[0]
	assert.Equal(t, "system_cpu_utilization", s.Name)
	assert.Equal(t, 0.5, s.Value)
	assert.Equal(t, tsMS, s.TimestampMS)
	// Labels sorted by name.
	assert.Equal(t, []Label{{"cpu", "0"}, {"state", "idle"}}, s.Labels)
}

func TestMap_Counter_TotalSuffix(t *testing.T) {
	fam, err := Map(embed.MetricPoint{
		Name:     "http.requests",
		Type:     embed.MetricTypeCounter,
		IntValue: i64(1027),
		Metadata: embed.MetricPointMetadata{Timestamp: ts()},
	})
	require.NoError(t, err)
	assert.Equal(t, TypeCounter, fam.Type)
	assert.Equal(t, "http_requests_total", fam.Name)
	require.Len(t, fam.Samples, 1)
	assert.Equal(t, "http_requests_total", fam.Samples[0].Name)
	assert.Equal(t, 1027.0, fam.Samples[0].Value)
}

func TestMap_Counter_AlreadyTotal(t *testing.T) {
	fam, err := Map(embed.MetricPoint{
		Name:     "http_requests_total",
		Type:     embed.MetricTypeCounter,
		IntValue: i64(1),
		Metadata: embed.MetricPointMetadata{Timestamp: ts()},
	})
	require.NoError(t, err)
	assert.Equal(t, "http_requests_total", fam.Name, "_total must not be doubled")
}

func TestMap_Sum_IsGauge(t *testing.T) {
	fam, err := Map(embed.MetricPoint{
		Name:     "queue.size",
		Type:     embed.MetricTypeSum,
		IntValue: i64(5),
		Metadata: embed.MetricPointMetadata{Timestamp: ts()},
	})
	require.NoError(t, err)
	assert.Equal(t, TypeGauge, fam.Type, "non-monotonic Sum maps to gauge")
	assert.Equal(t, "queue_size", fam.Name, "gauge takes no _total suffix")
	assert.Equal(t, 5.0, fam.Samples[0].Value)
}

func TestMap_Histogram(t *testing.T) {
	fam, err := Map(embed.MetricPoint{
		Name:                  "request.duration",
		Type:                  embed.MetricTypeHistogram,
		HistogramBucketBounds: []float64{0.1, 0.5, 1},
		HistogramBucketCounts: []uint64{1, 2, 3, 4}, // len == bounds+1
		HistogramSum:          3.3,
		HistogramCount:        10,
		Metadata:              embed.MetricPointMetadata{Timestamp: ts(), Attributes: map[string]string{"route": "/x"}},
	})
	require.NoError(t, err)
	assert.Equal(t, TypeHistogram, fam.Type)
	assert.Equal(t, "request_duration", fam.Name)

	// Cumulative bucket counts, each with an le label plus the base route label.
	b1 := find(t, fam, "request_duration_bucket")
	_ = b1 // multiple _bucket samples share the name; assert via le below

	le := func(v string) Sample {
		for _, s := range fam.Samples {
			if s.Name != "request_duration_bucket" {
				continue
			}
			for _, l := range s.Labels {
				if l.Name == "le" && l.Value == v {
					return s
				}
			}
		}
		t.Fatalf("no _bucket sample with le=%q", v)
		return Sample{}
	}
	assert.Equal(t, 1.0, le("0.1").Value)
	assert.Equal(t, 3.0, le("0.5").Value)   // 1+2
	assert.Equal(t, 6.0, le("1").Value)     // 1+2+3
	assert.Equal(t, 10.0, le("+Inf").Value) // 1+2+3+4

	// le buckets carry the base labels too, sorted (le sorts after route).
	assert.Equal(t, []Label{{"route", "/x"}, {"le", "0.1"}}, le("0.1").Labels)

	assert.Equal(t, 3.3, find(t, fam, "request_duration_sum").Value)
	assert.Equal(t, 10.0, find(t, fam, "request_duration_count").Value)
}

func TestMap_Histogram_BucketLengthMismatch(t *testing.T) {
	_, err := Map(embed.MetricPoint{
		Name:                  "bad",
		Type:                  embed.MetricTypeHistogram,
		HistogramBucketBounds: []float64{0.1, 0.5},
		HistogramBucketCounts: []uint64{1, 2, 3, 4}, // should be len(bounds)+1 == 3
		Metadata:              embed.MetricPointMetadata{Timestamp: ts()},
	})
	require.Error(t, err)
}

func TestMap_NameAndLabelSanitization(t *testing.T) {
	fam, err := Map(embed.MetricPoint{
		Name:        "weird-name.with/chars",
		Type:        embed.MetricTypeGauge,
		DoubleValue: f64(1),
		Metadata: embed.MetricPointMetadata{
			Timestamp:  ts(),
			Attributes: map[string]string{"http.method": "GET", "1bad": "x"},
		},
	})
	require.NoError(t, err)
	assert.Equal(t, "weird_name_with_chars", fam.Name)
	got := map[string]string{}
	for _, l := range fam.Samples[0].Labels {
		got[l.Name] = l.Value
	}
	assert.Equal(t, "GET", got["http_method"], "dotted label name sanitized")
	assert.Equal(t, "x", got["_1bad"], "label name starting with a digit gets a leading underscore")
}

func TestMap_Deterministic(t *testing.T) {
	mp := embed.MetricPoint{
		Name:        "m",
		Type:        embed.MetricTypeGauge,
		DoubleValue: f64(2),
		Metadata: embed.MetricPointMetadata{
			Timestamp:  ts(),
			Attributes: map[string]string{"b": "2", "a": "1", "c": "3"},
		},
	}
	a, err := Map(mp)
	require.NoError(t, err)
	b, err := Map(mp)
	require.NoError(t, err)
	assert.Equal(t, a, b)
	assert.Equal(t, []Label{{"a", "1"}, {"b", "2"}, {"c", "3"}}, a.Samples[0].Labels)
}
