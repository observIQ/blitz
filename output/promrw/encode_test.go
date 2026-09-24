package promrw

import (
	"testing"

	"github.com/golang/snappy"
	"github.com/observiq/blitz/internal/prommap"
	"github.com/prometheus/prometheus/prompb"
	writev2 "github.com/prometheus/prometheus/prompb/io/prometheus/write/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// sampleFamilies is a small gauge + counter set exercising labels and the
// __name__ series name.
func sampleFamilies() []prommap.MetricFamily {
	return []prommap.MetricFamily{
		{
			Name: "system_cpu_utilization",
			Type: prommap.TypeGauge,
			Help: "CPU utilization",
			Unit: "1",
			Samples: []prommap.Sample{
				{Name: "system_cpu_utilization", Labels: []prommap.Label{{Name: "cpu", Value: "0"}}, Value: 0.5, TimestampMS: 1700000000000},
			},
		},
		{
			Name: "http_requests_total",
			Type: prommap.TypeCounter,
			Samples: []prommap.Sample{
				{Name: "http_requests_total", Labels: []prommap.Label{{Name: "method", Value: "get"}}, Value: 1027, TimestampMS: 1700000000000},
			},
		},
	}
}

// labelMap flattens prompb labels for assertion.
func labelMap(ls []prompb.Label) map[string]string {
	m := map[string]string{}
	for _, l := range ls {
		m[l.Name] = l.Value
	}
	return m
}

func TestEncodeV1_RoundTrip(t *testing.T) {
	body, err := encode(versionV1, sampleFamilies())
	require.NoError(t, err)

	raw, err := snappy.Decode(nil, body)
	require.NoError(t, err)

	var req prompb.WriteRequest
	require.NoError(t, req.Unmarshal(raw))
	require.Len(t, req.Timeseries, 2)

	// Find the gauge series by __name__ and check its label + sample.
	var found bool
	for _, ts := range req.Timeseries {
		lm := labelMap(ts.Labels)
		if lm["__name__"] != "system_cpu_utilization" {
			continue
		}
		found = true
		assert.Equal(t, "0", lm["cpu"])
		require.Len(t, ts.Samples, 1)
		assert.Equal(t, 0.5, ts.Samples[0].Value)
		assert.Equal(t, int64(1700000000000), ts.Samples[0].Timestamp)
	}
	assert.True(t, found, "gauge series present with __name__")
}

func TestEncodeV2_SymbolTableRoundTrip(t *testing.T) {
	body, err := encode(versionV2, sampleFamilies())
	require.NoError(t, err)

	raw, err := snappy.Decode(nil, body)
	require.NoError(t, err)

	var req writev2.Request
	require.NoError(t, req.Unmarshal(raw))
	require.NotEmpty(t, req.Symbols)
	assert.Equal(t, "", req.Symbols[0], "v2 symbol table must start with the empty string")
	require.Len(t, req.Timeseries, 2)

	// Resolve the first series' labels through the symbol table.
	sym := req.Symbols
	var found bool
	for _, ts := range req.Timeseries {
		lm := map[string]string{}
		refs := ts.LabelsRefs
		for i := 0; i+1 < len(refs); i += 2 {
			lm[sym[refs[i]]] = sym[refs[i+1]]
		}
		if lm["__name__"] != "http_requests_total" {
			continue
		}
		found = true
		assert.Equal(t, "get", lm["method"])
		require.Len(t, ts.Samples, 1)
		assert.Equal(t, float64(1027), ts.Samples[0].Value)
	}
	assert.True(t, found, "counter series resolvable through the symbol table")
}
