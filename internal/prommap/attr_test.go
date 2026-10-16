package prommap

import (
	"fmt"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/observiq/blitz/embed"
)

// TestAttrStringFormatsFloatsAsTheCollector pins float attribute values to
// pcommon's Value.AsString (ES6 number formatting: plain below 1e21 and from
// 1e-6, exponent otherwise; float32 widened first).
func TestAttrStringFormatsFloatsAsTheCollector(t *testing.T) {
	for in, want := range map[any]string{
		1e6:               "1000000",
		1.5:               "1.5",
		1e20:              "100000000000000000000",
		1e21:              "1e+21",
		1e-7:              "1e-7",
		0.000001:          "0.000001",
		float32(0.5):      "0.5",
		float32(0.1):      "0.10000000149011612",
		math.Inf(1):       "json: unsupported value: +Inf",
		float64(-2500000): "-2500000",
	} {
		assert.Equal(t, want, AttrString(in), "%v (%T)", in, in)
	}
}

// TestAttrStringOtherKinds pins the non-float kinds: nil is empty, scalars
// print plainly, and composites are JSON (or printed when JSON fails).
func TestAttrStringOtherKinds(t *testing.T) {
	assert.Equal(t, "", AttrString(nil))
	assert.Equal(t, "true", AttrString(true))
	assert.Equal(t, "42", AttrString(42))
	assert.Equal(t, `["a","b"]`, AttrString([]string{"a", "b"}))
	ch := make(chan int)
	assert.Equal(t, fmt.Sprint(ch), AttrString(ch))
}

// TestMapRejects pins Map's errors: a name the namer refuses, an attribute
// key that is no label name, and an unsupported type.
func TestMapRejects(t *testing.T) {
	_, err := Map(embed.MetricPoint{Type: embed.MetricTypeGauge})
	require.Error(t, err)
	_, err = Map(embed.MetricPoint{Name: "m", Type: embed.MetricTypeGauge, Metadata: embed.MetricPointMetadata{Attributes: map[string]string{"": "x"}}})
	require.Error(t, err)
	_, err = Map(embed.MetricPoint{Name: "m", Type: "summary"})
	require.ErrorContains(t, err, "unsupported metric type")
}

// A point with neither value set maps to 0.
func TestMapUnsetValueIsZero(t *testing.T) {
	fam, err := Map(embed.MetricPoint{Name: "m", Type: embed.MetricTypeGauge})
	require.NoError(t, err)
	assert.Equal(t, 0.0, fam.Samples[0].Value)
}

// A resource key that is no label name yields no target_info.
func TestTargetInfoBadKey(t *testing.T) {
	_, ok := TargetInfo(gaugeWithResource(map[string]any{"host.name": "h", "": "x"}))
	assert.False(t, ok)
}

// LabelName sanitizes as the exporters do and rejects an empty key.
func TestLabelName(t *testing.T) {
	name, err := LabelName("http.method")
	require.NoError(t, err)
	assert.Equal(t, "http_method", name)
	_, err = LabelName("")
	require.Error(t, err)
}
