package promscrape

import (
	"math"
	"strings"
	"testing"

	"github.com/observiq/blitz/internal/prommap"
	"github.com/stretchr/testify/require"
)

func TestEncodeGauge(t *testing.T) {
	fams := []prommap.MetricFamily{{
		Name: "system_cpu_utilization",
		Type: prommap.TypeGauge,
		Help: "CPU utilization",
		Samples: []prommap.Sample{
			{Name: "system_cpu_utilization", Labels: []prommap.Label{{Name: "cpu", Value: "0"}}, Value: 0.5, TimestampMS: 1700000000000},
		},
	}}

	got := string(encode(fams, false))
	want := "# HELP system_cpu_utilization CPU utilization\n" +
		"# TYPE system_cpu_utilization gauge\n" +
		"system_cpu_utilization{cpu=\"0\"} 0.5\n"
	require.Equal(t, want, got)
}

func TestEncodeCounterNoLabels(t *testing.T) {
	fams := []prommap.MetricFamily{{
		Name: "http_requests_total",
		Type: prommap.TypeCounter,
		Samples: []prommap.Sample{
			{Name: "http_requests_total", Value: 1027},
		},
	}}

	got := string(encode(fams, false))
	want := "# TYPE http_requests_total counter\n" +
		"http_requests_total 1027\n"
	require.Equal(t, want, got)
}

func TestEncodeHistogram(t *testing.T) {
	fams := []prommap.MetricFamily{{
		Name: "request_duration_seconds",
		Type: prommap.TypeHistogram,
		Help: "latency",
		Samples: []prommap.Sample{
			{Name: "request_duration_seconds_bucket", Labels: []prommap.Label{{Name: "le", Value: "0.5"}}, Value: 3},
			{Name: "request_duration_seconds_bucket", Labels: []prommap.Label{{Name: "le", Value: "+Inf"}}, Value: 5},
			{Name: "request_duration_seconds_sum", Value: 1.2},
			{Name: "request_duration_seconds_count", Value: 5},
		},
	}}

	got := string(encode(fams, false))
	want := "# HELP request_duration_seconds latency\n" +
		"# TYPE request_duration_seconds histogram\n" +
		"request_duration_seconds_bucket{le=\"0.5\"} 3\n" +
		"request_duration_seconds_bucket{le=\"+Inf\"} 5\n" +
		"request_duration_seconds_sum 1.2\n" +
		"request_duration_seconds_count 5\n"
	require.Equal(t, want, got)
}

func TestEncodeEmitTimestamps(t *testing.T) {
	fams := []prommap.MetricFamily{{
		Name: "g",
		Type: prommap.TypeGauge,
		Samples: []prommap.Sample{
			{Name: "g", Value: 2, TimestampMS: 1700000000000},
		},
	}}

	got := string(encode(fams, true))
	want := "# TYPE g gauge\n" +
		"g 2 1700000000000\n"
	require.Equal(t, want, got)
}

func TestEncodeEscaping(t *testing.T) {
	fams := []prommap.MetricFamily{{
		Name: "g",
		Type: prommap.TypeGauge,
		Help: "line\\one\ntwo",
		Samples: []prommap.Sample{
			{Name: "g", Labels: []prommap.Label{{Name: "path", Value: "a\\b\nc\"d"}}, Value: 1},
		},
	}}

	got := string(encode(fams, false))
	want := "# HELP g line\\\\one\\ntwo\n" +
		"# TYPE g gauge\n" +
		"g{path=\"a\\\\b\\nc\\\"d\"} 1\n"
	require.Equal(t, want, got)
}

func TestEncodeSpecialFloats(t *testing.T) {
	fams := []prommap.MetricFamily{{
		Name: "g",
		Type: prommap.TypeGauge,
		Samples: []prommap.Sample{
			{Name: "g", Value: math.Inf(1)},
		},
	}}

	got := string(encode(fams, false))
	require.Contains(t, got, "g +Inf\n")
}

func TestEncodeStableFamilyOrder(t *testing.T) {
	fams := []prommap.MetricFamily{
		{Name: "zzz", Type: prommap.TypeGauge, Samples: []prommap.Sample{{Name: "zzz", Value: 1}}},
		{Name: "aaa", Type: prommap.TypeGauge, Samples: []prommap.Sample{{Name: "aaa", Value: 1}}},
	}

	got := string(encode(fams, false))
	require.Less(t, strings.Index(got, "aaa"), strings.Index(got, "zzz"))
}
