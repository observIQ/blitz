// Package prommap maps blitz embed MetricPoints to a neutral in-memory
// Prometheus model that both the scrape (text) and remote-write (protobuf)
// output encoders serialize from. It owns type mapping, name/label
// sanitization, suffixing, cumulative histogram expansion, and HELP/TYPE/unit.
package prommap

import (
	"encoding/json"
	"fmt"
	"maps"
	"math"
	"slices"
	"sort"
	"strconv"
	"time"

	"github.com/prometheus/otlptranslator"

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
	base, err := promName(mp)
	if err != nil {
		return MetricFamily{}, err
	}
	labels, err := sortedLabels(mp.Metadata.Attributes, targetLabels(mp.Metadata.Resource))
	if err != nil {
		return MetricFamily{}, err
	}
	tsMS := SampleTime(mp.Metadata.Timestamp).UnixMilli()

	fam := MetricFamily{Help: mp.Description, Unit: mp.Unit}

	switch mp.Type {
	case embed.MetricTypeGauge, embed.MetricTypeSum:
		fam.Type = TypeGauge
		fam.Name = base
		fam.Samples = []Sample{{Name: base, Labels: labels, Value: scalarValue(mp), TimestampMS: tsMS}}
	case embed.MetricTypeCounter:
		fam.Type = TypeCounter
		fam.Name = base
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

// namer names metrics as the collector's prometheusremotewrite exporter does
// by default: escaped to underscores, with unit, _total, and _ratio suffixes.
var namer = otlptranslator.NewMetricNamer("", otlptranslator.UnderscoreEscapingWithSuffixes)

// promTypes are the OTel kinds the namer suffixes by: a Sum is non-monotonic.
var promTypes = map[embed.MetricType]otlptranslator.MetricType{
	embed.MetricTypeGauge:     otlptranslator.MetricTypeGauge,
	embed.MetricTypeSum:       otlptranslator.MetricTypeNonMonotonicCounter,
	embed.MetricTypeCounter:   otlptranslator.MetricTypeMonotonicCounter,
	embed.MetricTypeHistogram: otlptranslator.MetricTypeHistogram,
}

// promName is mp's Prometheus metric name (a histogram's base name).
func promName(mp embed.MetricPoint) (string, error) {
	name, err := namer.Build(otlptranslator.Metric{Name: mp.Name, Unit: mp.Unit, Type: promTypes[mp.Type]})
	if err != nil {
		return "", fmt.Errorf("prommap: %w", err)
	}
	return name, nil
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
	// le="+Inf" and _count both carry HistogramCount, as the collector does.
	samples = append(samples,
		Sample{Name: base + "_bucket", Labels: withLE(labels, "+Inf"), Value: float64(mp.HistogramCount), TimestampMS: tsMS},
		Sample{Name: base + "_sum", Labels: labels, Value: mp.HistogramSum, TimestampMS: tsMS},
		Sample{Name: base + "_count", Labels: labels, Value: float64(mp.HistogramCount), TimestampMS: tsMS},
	)
	return samples, nil
}

// sortedLabels converts metric attributes to name-sorted labels. Target
// labels (job, instance) win over a same-named attribute.
func sortedLabels(attrs map[string]string, target []Label) ([]Label, error) {
	if len(attrs) == 0 && len(target) == 0 {
		return nil, nil
	}
	byName := make(map[string]string, len(attrs)+len(target))
	if err := mergeLabels(byName, attrs); err != nil {
		return nil, err
	}
	for _, l := range target {
		byName[l.Name] = l.Value
	}
	return sortLabels(byName), nil
}

// labelNamer names labels as the collector's prometheusremotewrite exporter
// does by default.
var labelNamer = otlptranslator.LabelNamer{UnderscoreLabelSanitization: true} //nolint:staticcheck // the exporter's default

// mergeLabels adds attrs to byName as the collector's createAttributes does:
// keys in sorted order, empty values dropped, and keys that name one label
// joined with ";".
func mergeLabels(byName, attrs map[string]string) error {
	for _, k := range slices.Sorted(maps.Keys(attrs)) {
		v := attrs[k]
		if v == "" {
			continue
		}
		name, err := labelNamer.Build(k)
		if err != nil {
			return fmt.Errorf("prommap: %w", err)
		}
		if old, ok := byName[name]; ok {
			if old != v {
				byName[name] = old + ";" + v
			}
			continue
		}
		byName[name] = v
	}
	return nil
}

func sortLabels(byName map[string]string) []Label {
	labels := make([]Label, 0, len(byName))
	for k, v := range byName {
		labels = append(labels, Label{Name: k, Value: v})
	}
	sort.Slice(labels, func(i, j int) bool { return labels[i].Name < labels[j].Name })
	return labels
}

// Resource keys the OTel-to-Prometheus compatibility spec promotes to job and
// instance. They feed the target labels and are left off target_info.
const (
	keyServiceName       = "service.name"
	keyServiceNamespace  = "service.namespace"
	keyServiceInstanceID = "service.instance.id"
	keyHostName          = "host.name"
	keyTelemetrySource   = "telemetry.source"
)

// targetLabels derives job and instance from a resource as the collector's
// remote-write createAttributes does (job = [service.namespace/]service.name,
// instance = service.instance.id, each only when non-empty; a present
// service.namespace is prefixed even when empty). When service.name or
// service.instance.id is absent, it falls back to telemetry.source or
// host.name, which blitz generators set in place of service.*.
func targetLabels(res map[string]any) []Label {
	if len(res) == 0 {
		return nil
	}
	job := resourceString(res, keyTelemetrySource)
	if _, ok := res[keyServiceName]; ok {
		job = resourceString(res, keyServiceName)
		if _, ok := res[keyServiceNamespace]; ok && job != "" {
			job = resourceString(res, keyServiceNamespace) + "/" + job
		}
	}
	instance := resourceString(res, keyHostName)
	if _, ok := res[keyServiceInstanceID]; ok {
		instance = resourceString(res, keyServiceInstanceID)
	}
	var labels []Label
	if instance != "" {
		labels = append(labels, Label{Name: "instance", Value: instance})
	}
	if job != "" {
		labels = append(labels, Label{Name: "job", Value: job})
	}
	return labels
}

// TargetInfo builds the target_info series for a point's resource: a gauge
// of 1 labeled with job, instance, and every other resource attribute. Like
// the collector, it reports false when the point has no timestamp, the
// resource yields neither job nor instance, has nothing beyond the identifying
// service.* attributes, or has a key no label can name.
func TargetInfo(mp embed.MetricPoint) (MetricFamily, bool) {
	if mp.Metadata.Timestamp.IsZero() {
		return MetricFamily{}, false
	}
	target := targetLabels(mp.Metadata.Resource)
	if len(target) == 0 {
		return MetricFamily{}, false
	}
	attrs := make(map[string]string, len(mp.Metadata.Resource))
	for k, v := range mp.Metadata.Resource {
		switch k {
		case keyServiceName, keyServiceNamespace, keyServiceInstanceID:
			continue
		}
		attrs[k] = attrString(v)
	}
	if len(attrs) == 0 {
		return MetricFamily{}, false
	}
	byName := make(map[string]string, len(attrs)+len(target))
	if err := mergeLabels(byName, attrs); err != nil {
		return MetricFamily{}, false
	}
	for _, l := range target {
		byName[l.Name] = l.Value
	}
	return MetricFamily{
		Name: "target_info",
		Type: TypeGauge,
		Help: "Target metadata",
		Samples: []Sample{{
			Name:        "target_info",
			Labels:      sortLabels(byName),
			Value:       1,
			TimestampMS: mp.Metadata.Timestamp.UnixMilli(),
		}},
	}, true
}

func resourceString(res map[string]any, key string) string {
	v, ok := res[key]
	if !ok {
		return ""
	}
	return attrString(v)
}

// attrString renders a resource value as the collector does (pcommon
// Value.AsString): strings as-is, scalars formatted, slices and maps as JSON.
func attrString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case nil:
		return ""
	case float32:
		return float64AsString(float64(t)) // pcommon has no float32: it widens
	case float64:
		return float64AsString(t)
	case bool, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return fmt.Sprint(t)
	default:
		b, err := json.Marshal(t)
		if err != nil {
			return fmt.Sprint(t)
		}
		return string(b)
	}
}

// withLE copies base and appends le last (Prometheus puts le last on a bucket).
func withLE(base []Label, le string) []Label {
	out := make([]Label, len(base), len(base)+1)
	copy(out, base)
	return append(out, Label{Name: "le", Value: le})
}

// formatFloat renders a bucket boundary as the collector does: the shortest
// round-trippable decimal, never in exponent notation.
func formatFloat(f float64) string {
	return strconv.FormatFloat(f, 'f', -1, 64)
}

// float64AsString is pcommon's float64AsString (pdata v1.56): ES6 number
// formatting, plain from 1e-6 up to 1e21 and exponent outside that range.
func float64AsString(f float64) string {
	if math.IsInf(f, 0) || math.IsNaN(f) {
		return "json: unsupported value: " + strconv.FormatFloat(f, 'g', -1, 64)
	}
	format := byte('f')
	if abs := math.Abs(f); abs != 0 && (abs < 1e-6 || abs >= 1e21) {
		format = 'e'
	}
	b := strconv.AppendFloat(nil, f, format, -1, 64)
	// Trim the exponent's leading zero: e-09 becomes e-9.
	if n := len(b); format == 'e' && n >= 4 && b[n-4] == 'e' && b[n-3] == '-' && b[n-2] == '0' {
		b[n-2] = b[n-1]
		b = b[:n-1]
	}
	return string(b)
}

// SampleTime is the time a point's samples carry. A point with no timestamp
// is stamped at the Unix epoch, as the collector stamps an unset OTLP
// timestamp, instead of at year 1.
func SampleTime(t time.Time) time.Time {
	if t.IsZero() {
		return time.Unix(0, 0)
	}
	return t
}
