// Package prommap maps blitz embed MetricPoints to a neutral in-memory
// Prometheus model that both the scrape (text) and remote-write (protobuf)
// output encoders serialize from. It owns type mapping, name/label
// sanitization, suffixing, cumulative histogram expansion, and HELP/TYPE/unit.
package prommap

import (
	"fmt"
	"sort"
	"strconv"

	"github.com/observiq/blitz/embed"
)

// Type is a Prometheus metric family type. Counter and Histogram map directly;
// Sum maps to gauge (no monotonicity flag, so treated as non-monotonic).
type Type string

const (
	// TypeGauge is a Prometheus gauge.
	TypeGauge Type = "gauge"
	// TypeCounter is a Prometheus counter.
	TypeCounter Type = "counter"
	// TypeHistogram is a Prometheus histogram.
	TypeHistogram Type = "histogram"
)

// Label is a single Prometheus label name/value pair.
type Label struct {
	Name  string
	Value string
}

// Sample is one time-series point. Name already carries any
// _total/_bucket/_sum/_count suffix.
type Sample struct {
	Name        string
	Labels      []Label
	Value       float64
	TimestampMS int64
}

// MetricFamily groups samples sharing a base name, type, and metadata. Name is
// the HELP/TYPE-line name (for a counter it already includes _total).
type MetricFamily struct {
	Name    string
	Type    Type
	Help    string
	Unit    string
	Samples []Sample
}

// Map projects a single embed MetricPoint into a MetricFamily. A gauge, sum, or
// counter yields one sample; a histogram yields cumulative _bucket series (with
// le labels, including +Inf), plus _sum and _count.
func Map(mp embed.MetricPoint) (MetricFamily, error) {
	base := sanitizeName(mp.Name)
	labels := sortedLabels(mp.Metadata.Attributes)
	tsMS := mp.Metadata.Timestamp.UnixMilli()

	fam := MetricFamily{Help: mp.Description, Unit: mp.Unit}

	switch mp.Type {
	case embed.MetricTypeGauge, embed.MetricTypeSum:
		fam.Type = TypeGauge
		fam.Name = base
		fam.Samples = []Sample{{Name: base, Labels: labels, Value: scalarValue(mp), TimestampMS: tsMS}}
	case embed.MetricTypeCounter:
		fam.Type = TypeCounter
		fam.Name = withTotal(base)
		fam.Samples = []Sample{{Name: fam.Name, Labels: labels, Value: scalarValue(mp), TimestampMS: tsMS}}
	case embed.MetricTypeHistogram:
		fam.Type = TypeHistogram
		fam.Name = base
		samples, err := histogramSamples(base, labels, mp, tsMS)
		if err != nil {
			return MetricFamily{}, err
		}
		fam.Samples = samples
	default:
		return MetricFamily{}, fmt.Errorf("prommap: unsupported metric type %q for %q", mp.Type, mp.Name)
	}
	return fam, nil
}

// scalarValue returns the gauge/sum/counter value as a float64, preferring
// IntValue when set, then DoubleValue, else 0.
func scalarValue(mp embed.MetricPoint) float64 {
	switch {
	case mp.IntValue != nil:
		return float64(*mp.IntValue)
	case mp.DoubleValue != nil:
		return *mp.DoubleValue
	default:
		return 0
	}
}

// histogramSamples expands an OTel explicit-bucket histogram into cumulative
// _bucket series (le, plus +Inf), _sum, and _count. OTel counts are per-bucket
// with one overflow bucket, so len(counts) must be len(bounds)+1.
func histogramSamples(base string, labels []Label, mp embed.MetricPoint, tsMS int64) ([]Sample, error) {
	bounds := mp.HistogramBucketBounds
	counts := mp.HistogramBucketCounts
	if len(counts) != len(bounds)+1 {
		return nil, fmt.Errorf("prommap: histogram %q has %d bucket counts, want len(bounds)+1 = %d", mp.Name, len(counts), len(bounds)+1)
	}

	samples := make([]Sample, 0, len(bounds)+3)
	var cumulative uint64
	for i, bound := range bounds {
		cumulative += counts[i]
		samples = append(samples, Sample{
			Name:        base + "_bucket",
			Labels:      withLE(labels, formatFloat(bound)),
			Value:       float64(cumulative),
			TimestampMS: tsMS,
		})
	}
	// The overflow bucket becomes le="+Inf" and holds the full count.
	cumulative += counts[len(counts)-1]
	samples = append(samples,
		Sample{Name: base + "_bucket", Labels: withLE(labels, "+Inf"), Value: float64(cumulative), TimestampMS: tsMS},
		Sample{Name: base + "_sum", Labels: labels, Value: mp.HistogramSum, TimestampMS: tsMS},
		Sample{Name: base + "_count", Labels: labels, Value: float64(mp.HistogramCount), TimestampMS: tsMS},
	)
	return samples, nil
}

// sortedLabels converts metric attributes to sanitized, name-sorted labels.
func sortedLabels(attrs map[string]string) []Label {
	if len(attrs) == 0 {
		return nil
	}
	labels := make([]Label, 0, len(attrs))
	for k, v := range attrs {
		labels = append(labels, Label{Name: sanitizeLabelName(k), Value: v})
	}
	sort.Slice(labels, func(i, j int) bool { return labels[i].Name < labels[j].Name })
	return labels
}

// withLE copies base and appends le last (Prometheus puts le last on a bucket).
func withLE(base []Label, le string) []Label {
	out := make([]Label, len(base), len(base)+1)
	copy(out, base)
	return append(out, Label{Name: "le", Value: le})
}

// withTotal appends _total to a counter name unless it is already present.
func withTotal(name string) string {
	if len(name) >= len("_total") && name[len(name)-len("_total"):] == "_total" {
		return name
	}
	return name + "_total"
}

// formatFloat renders a bucket boundary as the shortest round-trippable decimal.
func formatFloat(f float64) string {
	return strconv.FormatFloat(f, 'g', -1, 64)
}

// sanitizeName maps a name to a valid Prometheus metric name (colons allowed).
func sanitizeName(s string) string { return sanitize(s, true) }

// sanitizeLabelName maps a key to a valid Prometheus label name (no colons).
func sanitizeLabelName(s string) string { return sanitize(s, false) }

func sanitize(s string, allowColon bool) string {
	if s == "" {
		return s
	}
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c == '_':
			out = append(out, c)
		case c == ':' && allowColon:
			out = append(out, c)
		case c >= '0' && c <= '9':
			if i == 0 {
				out = append(out, '_')
			}
			out = append(out, c)
		default:
			out = append(out, '_')
		}
	}
	return string(out)
}
