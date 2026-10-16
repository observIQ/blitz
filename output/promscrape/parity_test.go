package promscrape

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/observiq/blitz/embed"
	"github.com/observiq/blitz/output"
	"github.com/stretchr/testify/require"
)

// The exposition matches the collector prometheusexporter (v0.161.0
// defaults): every series carries empty otel_scope_* labels (blitz sends an
// empty instrumentation scope), HELP is written even when the description is
// empty, empty attribute values are kept, and job/instance follow the
// resource (telemetry.source/host.name when service.* is absent).
func TestExpositionMatchesExporter(t *testing.T) {
	o := newTestOutput(t)
	v := 2.0
	require.NoError(t, o.WriteMetric(context.Background(), output.MetricRecord{
		Name: "system.memory.usage", Unit: "By", Type: embed.MetricTypeGauge, DoubleValue: &v,
		Metadata: output.MetricPointMetadata{
			Timestamp:  time.Unix(1700000000, 0),
			Attributes: map[string]string{"state": "used", "empty": ""},
			Resource:   map[string]any{"host.name": "athena", "telemetry.source": "hostmetrics", "os.type": "linux"},
		},
	}))
	body := scrape(t, o)
	require.Contains(t, body, "# HELP system_memory_usage_bytes \n# TYPE system_memory_usage_bytes gauge\n")
	require.Contains(t, body, `system_memory_usage_bytes{empty="",instance="athena",job="hostmetrics",otel_scope_name="",otel_scope_schema_url="",otel_scope_version="",state="used"} 2`)
	require.Contains(t, body, "# HELP target_info Target metadata\n# TYPE target_info gauge\n")
	require.Contains(t, body, `target_info{host_name="athena",instance="athena",job="hostmetrics",os_type="linux",telemetry_source="hostmetrics"} 1`)
}

// target_info needs both job and instance, as the exporter's
// resourceSignature does, and leaves the service.* attributes out of its
// labels.
func TestTargetInfoNeedsJobAndInstance(t *testing.T) {
	o := newTestOutput(t)
	v := 1.0
	write := func(res map[string]any) {
		require.NoError(t, o.WriteMetric(context.Background(), output.MetricRecord{
			Name: "up", Type: embed.MetricTypeGauge, DoubleValue: &v,
			Metadata: output.MetricPointMetadata{Timestamp: time.Unix(1700000000, 0), Resource: res},
		}))
	}
	write(map[string]any{"service.name": "cart"})
	write(map[string]any{"service.namespace": "shop", "service.name": "pay", "service.instance.id": "pay-1"})
	body := scrape(t, o)
	require.Equal(t, 1, strings.Count(body, "\ntarget_info{"))
	require.Contains(t, body, `target_info{instance="pay-1",job="shop/pay"} 1`)
}

// A point older than the stored one for the same series is ignored; a newer
// one replaces it.
func TestOlderPointIsIgnored(t *testing.T) {
	o := newTestOutput(t)
	write := func(sec int64, val float64) {
		require.NoError(t, o.WriteMetric(context.Background(), output.MetricRecord{
			Name: "load", Type: embed.MetricTypeGauge, DoubleValue: &val,
			Metadata: output.MetricPointMetadata{Timestamp: time.Unix(sec, 0)},
		}))
	}
	write(100, 1)
	write(90, 2)
	require.Contains(t, scrape(t, o), "\nload{otel_scope_name=\"\",otel_scope_schema_url=\"\",otel_scope_version=\"\"} 1\n")
	write(110, 3)
	require.Contains(t, scrape(t, o), "\nload{otel_scope_name=\"\",otel_scope_schema_url=\"\",otel_scope_version=\"\"} 3\n")
}

// A name first seen as one type keeps it: a later point of another type
// under the same name (a gauge, then a histogram) is dropped, and the first
// HELP stays.
func TestFirstTypeAndHelpWin(t *testing.T) {
	o := newTestOutput(t)
	g := 1.0
	require.NoError(t, o.WriteMetric(context.Background(), output.MetricRecord{
		Name: "jobs", Description: "first", Type: embed.MetricTypeGauge, DoubleValue: &g,
		Metadata: output.MetricPointMetadata{Timestamp: time.Unix(1700000000, 0), Attributes: map[string]string{"q": "a"}},
	}))
	scrape(t, o)
	require.NoError(t, o.WriteMetric(context.Background(), output.MetricRecord{
		Name: "jobs", Description: "second", Type: embed.MetricTypeHistogram, HistogramCount: 1, HistogramBucketCounts: []uint64{1},
		Metadata: output.MetricPointMetadata{Timestamp: time.Unix(1700000000, 0), Attributes: map[string]string{"q": "b"}},
	}))
	body := scrape(t, o)
	require.Contains(t, body, "# HELP jobs first\n")
	require.NotContains(t, body, `q="b"`)
}

// Two attributes that sanitize to one label name make the point invalid, as
// they do for the exporter's prometheus.NewDesc; other series still serve.
func TestCollidingLabelsDropOnlyThatSeries(t *testing.T) {
	o := newTestOutput(t)
	v := 1.0
	for _, attrs := range []map[string]string{{"a.b": "1", "a_b": "2"}, {"ok": "1"}} {
		require.NoError(t, o.WriteMetric(context.Background(), output.MetricRecord{
			Name: "x", Type: embed.MetricTypeGauge, DoubleValue: &v,
			Metadata: output.MetricPointMetadata{Timestamp: time.Unix(1700000000, 0), Attributes: attrs},
		}))
	}
	body := scrape(t, o)
	require.Contains(t, body, `ok="1"`)
	require.NotContains(t, body, `a_b=`)
}

// Histograms expose cumulative buckets from the explicit bounds, with the
// point's count and sum.
func TestHistogramExposition(t *testing.T) {
	o := newTestOutput(t)
	require.NoError(t, o.WriteMetric(context.Background(), output.MetricRecord{
		Name: "rpc.duration", Unit: "s", Type: embed.MetricTypeHistogram,
		HistogramCount: 6, HistogramSum: 2.5,
		HistogramBucketBounds: []float64{0.1, 1}, HistogramBucketCounts: []uint64{1, 2, 3},
		Metadata: output.MetricPointMetadata{Timestamp: time.Unix(1700000000, 0)},
	}))
	body := scrape(t, o)
	scope := `otel_scope_name="",otel_scope_schema_url="",otel_scope_version=""`
	for _, line := range []string{
		`rpc_duration_seconds_bucket{` + scope + `,le="0.1"} 1`,
		`rpc_duration_seconds_bucket{` + scope + `,le="1"} 3`,
		`rpc_duration_seconds_bucket{` + scope + `,le="+Inf"} 6`,
		`rpc_duration_seconds_sum{` + scope + `} 2.5`,
		`rpc_duration_seconds_count{` + scope + `} 6`,
	} {
		require.Contains(t, body, line+"\n")
	}
}

// emitTimestamps appends each point's millisecond timestamp, as the
// exporter's send_timestamps does.
func TestEmitTimestamps(t *testing.T) {
	o, err := New("127.0.0.1:0", "/metrics", true, 5*time.Minute, embed.TelemetrySettings{}, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = o.Stop(context.Background()) })
	v := 1.0
	require.NoError(t, o.WriteMetric(context.Background(), output.MetricRecord{
		Name: "up", Type: embed.MetricTypeGauge, DoubleValue: &v,
		Metadata: output.MetricPointMetadata{Timestamp: time.UnixMilli(1700000000123)},
	}))
	require.Contains(t, scrape(t, o), `up{otel_scope_name="",otel_scope_schema_url="",otel_scope_version=""} 1 1700000000123`)
}

// A point with no timestamp is stamped at the Unix epoch, as the exporter
// stamps an unset OTLP timestamp, not at year 1.
func TestEmitTimestampsZeroIsEpoch(t *testing.T) {
	o, err := New("127.0.0.1:0", "/metrics", true, 5*time.Minute, embed.TelemetrySettings{}, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = o.Stop(context.Background()) })
	v := 1.0
	require.NoError(t, o.WriteMetric(context.Background(), output.MetricRecord{Name: "up", Type: embed.MetricTypeGauge, DoubleValue: &v}))
	require.Contains(t, scrape(t, o), `up{otel_scope_name="",otel_scope_schema_url="",otel_scope_version=""} 1 0`+"\n")
}

// TestPresentButEmptyServiceKeys pins the exporter's extractJob and
// extractInstance: a present service.name is the job even when empty
// (prefixed by a present namespace), a present service.instance.id is the
// instance even when empty, and neither falls back then. target_info needs
// both non-empty, so only the first resource gets one.
func TestPresentButEmptyServiceKeys(t *testing.T) {
	o := newTestOutput(t)
	require.NoError(t, o.WriteMetric(context.Background(), gauge("a", 1, nil, map[string]any{
		"service.namespace": "ns", "service.name": "", "telemetry.source": "src", "host.name": "h",
	})))
	require.NoError(t, o.WriteMetric(context.Background(), gauge("b", 1, nil, map[string]any{
		"service.name": "svc", "service.instance.id": "", "host.name": "h",
	})))
	body := scrape(t, o)
	require.Contains(t, body, "\na{instance=\"h\",job=\"ns/\",")
	require.Contains(t, body, "\nb{instance=\"\",job=\"svc\",")
	require.Equal(t, 1, strings.Count(body, "target_info{"))
	require.Contains(t, body, `target_info{host_name="h",instance="h",job="ns/",telemetry_source="src"} 1`)
}
