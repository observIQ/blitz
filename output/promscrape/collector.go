package promscrape

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"sort"
	"sync"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"

	"github.com/observiq/blitz/embed"
	"github.com/observiq/blitz/internal/prommap"
)

// collector turns the stored points into Prometheus metrics the way the
// collector prometheusexporter (v0.161.0, default config) does, so a scrape
// of blitz reads like a scrape of that exporter. The one difference is
// job/instance: they fall back to telemetry.source/host.name when
// service.name/service.instance.id is absent.
type collector struct {
	st             *store
	emitTimestamps bool
	logger         *zap.Logger
	clock          clockwork.Clock
	expiry         time.Duration

	mu       sync.Mutex
	families map[string]family // first-seen HELP and type per metric name

	// onCollect receives each Collect's series count, target_info aside,
	// while its scrape is still gathering, so overlapping scrapes each
	// report their own. Nil records nothing.
	onCollect func(exposed int64)
}

type family struct {
	help     string
	typ      embed.MetricType
	lastSeen time.Time
}

func newCollector(st *store, emitTimestamps bool, clk clockwork.Clock, expiry time.Duration, logger *zap.Logger) *collector {
	return &collector{st: st, emitTimestamps: emitTimestamps, logger: logger, clock: clk, expiry: expiry, families: make(map[string]family)}
}

// Describe sends nothing: the metric set is dynamic (an unchecked collector).
func (*collector) Describe(chan<- *prometheus.Desc) {}

// Collect emits one target_info per target, then every stored series.
func (c *collector) Collect(ch chan<- prometheus.Metric) {
	points := c.st.snapshot()
	for _, m := range c.targetInfo(points) {
		ch <- m
	}
	var exposed int64
	for _, mp := range points {
		m, err := c.convert(mp)
		if err != nil {
			c.logger.Error("failed to convert metric", zap.String("metric", mp.Name), zap.Error(err))
			continue
		}
		ch <- m
		exposed += seriesOf(mp)
	}
	if c.onCollect != nil {
		c.onCollect(exposed)
	}
	c.expireFamilies()
}

// seriesOf is the Prometheus series a point exposes: one, or for a histogram
// a bucket per distinct bound plus +Inf, _sum, and _count.
func seriesOf(mp embed.MetricPoint) int64 {
	if mp.Type == embed.MetricTypeHistogram {
		return int64(len(cumulativeBuckets(mp))) + 3
	}
	return 1
}

// scopeLabels are the exporter's instrumentation-scope labels; blitz sends an
// empty scope, so each is empty.
var scopeLabels = []string{"otel_scope_name", "otel_scope_version", "otel_scope_schema_url"}

func (c *collector) convert(mp embed.MetricPoint) (prometheus.Metric, error) {
	name, _ := prommap.MetricName(mp) // WriteMetric already accepted the name
	help, err := c.validate(name, mp)
	if err != nil {
		return nil, err
	}
	var keys, values []string
	var errs error
	for _, k := range slices.Sorted(maps.Keys(mp.Metadata.Attributes)) {
		label, err := prommap.LabelName(k)
		if err != nil {
			errs = errors.Join(errs, err)
			continue
		}
		keys = append(keys, label)
		values = append(values, mp.Metadata.Attributes[k])
	}
	if errs != nil {
		return nil, errs
	}
	for _, l := range scopeLabels {
		keys = append(keys, l)
		values = append(values, "")
	}
	job, instance, hasJob, hasInstance := target(mp.Metadata.Resource)
	if hasJob {
		keys = append(keys, "job")
		values = append(values, job)
	}
	if hasInstance {
		keys = append(keys, "instance")
		values = append(values, instance)
	}
	desc := prometheus.NewDesc(name, help, keys, nil)

	// WriteMetric admits only gauge, sum, counter, and histogram.
	var m prometheus.Metric
	switch mp.Type {
	case embed.MetricTypeHistogram:
		m, err = prometheus.NewConstHistogram(desc, mp.HistogramCount, mp.HistogramSum, cumulativeBuckets(mp), values...)
	case embed.MetricTypeCounter:
		m, err = prometheus.NewConstMetric(desc, prometheus.CounterValue, scalar(mp), values...)
	default:
		m, err = prometheus.NewConstMetric(desc, prometheus.GaugeValue, scalar(mp), values...)
	}
	if err != nil {
		return nil, err
	}
	if c.emitTimestamps {
		m = prometheus.NewMetricWithTimestamp(prommap.SampleTime(mp.Metadata.Timestamp), m)
	}
	return m, nil
}

// validate returns the HELP for name: the first description seen wins, and a
// point whose type differs from the name's first type is refused.
func (c *collector) validate(name string, mp embed.MetricPoint) (string, error) {
	typ := mp.Type
	if typ == embed.MetricTypeSum {
		typ = embed.MetricTypeGauge // a non-monotonic sum is exposed as a gauge
	}
	now := c.clock.Now()
	c.mu.Lock()
	defer c.mu.Unlock()
	f, ok := c.families[name]
	if !ok {
		c.families[name] = family{help: mp.Description, typ: typ, lastSeen: now}
		return mp.Description, nil
	}
	if f.typ != typ {
		return "", fmt.Errorf("instrument type conflict for %s: existing %s, dropped %s", name, f.typ, typ)
	}
	f.lastSeen = now
	c.families[name] = f
	return f.help, nil
}

func (c *collector) expireFamilies() {
	if c.expiry <= 0 {
		return
	}
	now := c.clock.Now()
	c.mu.Lock()
	defer c.mu.Unlock()
	for name, f := range c.families {
		if now.Sub(f.lastSeen) > c.expiry {
			delete(c.families, name)
		}
	}
}

// targetInfo builds one target_info per distinct job+instance; a resource
// missing either has no target, as in the exporter's resourceSignature. The
// labels are every resource attribute but service.*, joined with ";" where two
// keys name one label, plus job and instance.
func (c *collector) targetInfo(points []embed.MetricPoint) []prometheus.Metric {
	seen := map[string]bool{}
	var out []prometheus.Metric
	for _, mp := range points {
		job, instance, _, _ := target(mp.Metadata.Resource)
		sig := job + "\xff" + instance
		if job == "" || instance == "" || seen[sig] {
			continue
		}
		seen[sig] = true
		labels := map[string]string{}
		for _, k := range slices.Sorted(maps.Keys(mp.Metadata.Resource)) {
			switch k {
			case "service.name", "service.namespace", "service.instance.id":
				continue
			}
			label, err := prommap.LabelName(k)
			if err != nil {
				// The exporter drops every target_info on a bad key.
				c.logger.Error("failed to convert target_info", zap.Error(err))
				return nil
			}
			v := prommap.AttrString(mp.Metadata.Resource[k])
			if prev, dup := labels[label]; dup {
				v = prev + ";" + v
			}
			labels[label] = v
		}
		labels["job"] = job
		labels["instance"] = instance
		keys := slices.Sorted(maps.Keys(labels))
		values := make([]string, len(keys))
		for i, k := range keys {
			values[i] = labels[k]
		}
		m, err := prometheus.NewConstMetric(prometheus.NewDesc("target_info", "Target metadata", keys, nil), prometheus.GaugeValue, 1, values...)
		if err != nil {
			c.logger.Error("failed to convert target_info", zap.Error(err))
			continue
		}
		out = append(out, m)
	}
	return out
}

func scalar(mp embed.MetricPoint) float64 {
	switch {
	case mp.IntValue != nil:
		return float64(*mp.IntValue)
	case mp.DoubleValue != nil:
		return *mp.DoubleValue
	}
	return 0
}

// cumulativeBuckets sums per-bucket counts up each sorted, de-duplicated
// bound, as the exporter's convertDoubleHistogram does; +Inf comes from the
// point's count.
func cumulativeBuckets(mp embed.MetricPoint) map[float64]uint64 {
	index := map[float64]int{}
	var bounds []float64
	for i, b := range mp.HistogramBucketBounds {
		if _, ok := index[b]; !ok {
			index[b] = i
			bounds = append(bounds, b)
		}
	}
	sort.Float64s(bounds)
	out := make(map[float64]uint64, len(bounds))
	var cum uint64
	for _, b := range bounds {
		cum += mp.HistogramBucketCounts[index[b]]
		out[b] = cum
	}
	return out
}

// target is the exporter's extractJob and extractInstance: a present
// service.name is the job (prefixed by a present service.namespace) and a
// present service.instance.id the instance, even when empty. When a key is
// absent, telemetry.source or host.name stands in, if set.
func target(res map[string]any) (job, instance string, hasJob, hasInstance bool) {
	if name, ok := res["service.name"]; ok {
		job, hasJob = prommap.AttrString(name), true
		if ns, ok := res["service.namespace"]; ok {
			job = prommap.AttrString(ns) + "/" + job
		}
	} else if src, ok := res["telemetry.source"]; ok {
		job = prommap.AttrString(src)
		hasJob = job != ""
	}
	if id, ok := res["service.instance.id"]; ok {
		instance, hasInstance = prommap.AttrString(id), true
	} else if host, ok := res["host.name"]; ok {
		instance = prommap.AttrString(host)
		hasInstance = instance != ""
	}
	return job, instance, hasJob, hasInstance
}
