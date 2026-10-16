package promscrape

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/observiq/blitz/embed"
	"github.com/observiq/blitz/output"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/noop"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

// slowClock reports every interval as 500µs, so a scrape's latency is
// sub-millisecond.
type slowClock struct{ clockwork.Clock }

func (slowClock) Since(time.Time) time.Duration { return 500 * time.Microsecond }

func gauge(name string, v float64, attrs map[string]string, res map[string]any) output.MetricRecord {
	return output.MetricRecord{
		Name: name, Type: embed.MetricTypeGauge, DoubleValue: &v,
		Metadata: output.MetricPointMetadata{Timestamp: time.Unix(1700000000, 0), Attributes: attrs, Resource: res},
	}
}

func collect(t *testing.T, reader *sdkmetric.ManualReader) map[string]metricdata.Aggregation {
	t.Helper()
	var rm metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(context.Background(), &rm))
	out := map[string]metricdata.Aggregation{}
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			out[m.Name] = m.Data
		}
	}
	return out
}

// Each scrape records its count, the series exposed, the body size, and a
// sub-millisecond latency; each write counts one entry received, including a
// point the output rejects.
func TestScrapeTelemetry(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	tel := embed.TelemetrySettings{MeterProvider: sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))}
	o, err := newWithClock("127.0.0.1:0", "", false, 0, tel, nil, slowClock{clockwork.NewRealClock()})
	require.NoError(t, err)
	t.Cleanup(func() { _ = o.Stop(context.Background()) })
	require.NoError(t, o.WriteMetric(context.Background(), gauge("a", 1, nil, nil)))
	require.NoError(t, o.WriteMetric(context.Background(), gauge("b", 1, nil, nil)))
	require.Error(t, o.WriteMetric(context.Background(), output.MetricRecord{Name: "c", Type: "bogus"}))

	// The empty path defaulted to /metrics. Ask for an uncompressed body so
	// the bytes on the wire are the body's length (gzip is negotiated).
	req, err := http.NewRequest(http.MethodGet, "http://"+o.Addr()+"/metrics", nil)
	require.NoError(t, err)
	req.Header.Set("Accept-Encoding", "identity")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	_ = resp.Body.Close()
	body := string(raw)

	got := collect(t, reader)
	require.Equal(t, int64(1), got["blitz.output.prometheus_scrape.scrapes"].(metricdata.Sum[int64]).DataPoints[0].Value)
	require.Equal(t, int64(2), got["blitz.output.prometheus_scrape.series_exposed"].(metricdata.Gauge[int64]).DataPoints[0].Value)
	require.Equal(t, int64(len(body)), got["blitz.output.prometheus_scrape.exposition_bytes"].(metricdata.Histogram[int64]).DataPoints[0].Sum)
	require.InDelta(t, 0.5, got["blitz.output.prometheus_scrape.scrape_latency"].(metricdata.Histogram[float64]).DataPoints[0].Sum, 1e-9)
	var received int64
	for _, dp := range got["blitz.output.entries_received"].(metricdata.Sum[int64]).DataPoints {
		received += dp.Value
	}
	require.Equal(t, int64(3), received)
}

// failingProvider fails every instrument of one meter scope.
type failingProvider struct {
	noop.MeterProvider
	scope string
}

func (p failingProvider) Meter(name string, _ ...metric.MeterOption) metric.Meter {
	if name == p.scope {
		return failingMeter{}
	}
	return noop.Meter{}
}

type failingMeter struct{ noop.Meter }

var errInstrument = errors.New("instrument refused")

func (failingMeter) Int64Counter(string, ...metric.Int64CounterOption) (metric.Int64Counter, error) {
	return nil, errInstrument
}
func (failingMeter) Int64Gauge(string, ...metric.Int64GaugeOption) (metric.Int64Gauge, error) {
	return nil, errInstrument
}
func (failingMeter) Int64Histogram(string, ...metric.Int64HistogramOption) (metric.Int64Histogram, error) {
	return nil, errInstrument
}
func (failingMeter) Float64Counter(string, ...metric.Float64CounterOption) (metric.Float64Counter, error) {
	return nil, errInstrument
}
func (failingMeter) Float64Histogram(string, ...metric.Float64HistogramOption) (metric.Float64Histogram, error) {
	return nil, errInstrument
}

// A self-telemetry build failure, of the shared output metrics or the scrape
// metrics, disables recording with a Warn and leaves the output serving.
func TestTelemetryFailureDisablesRecording(t *testing.T) {
	for _, scope := range []string{"output", "promscrape"} {
		core, logs := observer.New(zap.WarnLevel)
		o, err := New("127.0.0.1:0", "/metrics", false, 0, embed.TelemetrySettings{MeterProvider: failingProvider{scope: scope}}, zap.New(core))
		require.NoError(t, err)
		t.Cleanup(func() { _ = o.Stop(context.Background()) })
		require.Nil(t, o.metrics, scope)
		require.Equal(t, 1, logs.FilterMessage("prometheus-scrape metrics disabled").Len(), scope)
		require.NoError(t, o.WriteMetric(context.Background(), gauge("a", 1, nil, nil)))
		require.Contains(t, scrape(t, o), "\na{", scope)
	}
}

// WriteMetric refuses a point that has no Prometheus name or an unsupported
// type.
func TestWriteMetricRejects(t *testing.T) {
	o := newTestOutput(t)
	require.ErrorContains(t, o.WriteMetric(context.Background(), gauge("", 1, nil, nil)), "map metric")
	require.ErrorContains(t, o.WriteMetric(context.Background(), output.MetricRecord{Name: "x", Type: "summary"}), "unsupported metric type")
	require.ErrorContains(t, o.WriteMetric(context.Background(), output.MetricRecord{
		Name: "h", Type: embed.MetricTypeHistogram, HistogramBucketBounds: []float64{1}, HistogramBucketCounts: []uint64{1},
	}), "bucket counts")
}

// Series and per-name HELP/type state expire after metricExpiration without
// an update, and an update refreshes the clock.
func TestSeriesExpire(t *testing.T) {
	clk := clockwork.NewFakeClock()
	o, err := newWithClock("127.0.0.1:0", "/metrics", false, time.Minute, embed.TelemetrySettings{}, nil, clk)
	require.NoError(t, err)
	t.Cleanup(func() { _ = o.Stop(context.Background()) })
	require.NoError(t, o.WriteMetric(context.Background(), gauge("old", 1, nil, nil)))
	require.Contains(t, scrape(t, o), "\nold{")
	clk.Advance(50 * time.Second)
	require.NoError(t, o.WriteMetric(context.Background(), gauge("fresh", 1, nil, nil)))
	clk.Advance(20 * time.Second)
	body := scrape(t, o)
	require.NotContains(t, body, "old")
	require.Contains(t, body, "\nfresh{")
	// The expired name's HELP and type are forgotten: it may return as a
	// histogram.
	require.NoError(t, o.WriteMetric(context.Background(), output.MetricRecord{
		Name: "old", Type: embed.MetricTypeHistogram, HistogramCount: 1, HistogramBucketCounts: []uint64{1},
		Metadata: output.MetricPointMetadata{Timestamp: time.Unix(1700000000, 0)},
	}))
	require.Contains(t, scrape(t, o), "# TYPE old histogram")
}

// Integer values, and points with neither value, expose as numbers.
func TestScalarValues(t *testing.T) {
	o := newTestOutput(t)
	n := int64(7)
	require.NoError(t, o.WriteMetric(context.Background(), output.MetricRecord{
		Name: "requests", Type: embed.MetricTypeCounter, IntValue: &n,
		Metadata: output.MetricPointMetadata{Timestamp: time.Unix(1700000000, 0)},
	}))
	require.NoError(t, o.WriteMetric(context.Background(), output.MetricRecord{
		Name: "unset", Type: embed.MetricTypeGauge,
		Metadata: output.MetricPointMetadata{Timestamp: time.Unix(1700000000, 0)},
	}))
	body := scrape(t, o)
	require.Contains(t, body, "# TYPE requests_total counter\nrequests_total{")
	require.Contains(t, body, "} 7\n")
	require.Contains(t, body, "\nunset{otel_scope_name=\"\",otel_scope_schema_url=\"\",otel_scope_version=\"\"} 0\n")
}

// An attribute key that names no label drops only that series, logged at
// Error; resource keys that sanitize to one label are joined with ";" in key
// order, as the exporter's target_info does.
func TestLabelFailuresAndJoins(t *testing.T) {
	core, logs := observer.New(zap.ErrorLevel)
	o, err := New("127.0.0.1:0", "/metrics", false, 0, embed.TelemetrySettings{}, zap.New(core))
	require.NoError(t, err)
	t.Cleanup(func() { _ = o.Stop(context.Background()) })
	host := map[string]any{"host.name": "h1", "telemetry.source": "s", "a.b": "1", "a_b": "2"}
	require.NoError(t, o.WriteMetric(context.Background(), gauge("good", 1, nil, host)))
	require.NoError(t, o.WriteMetric(context.Background(), gauge("bad", 1, map[string]string{"": "x"}, nil)))
	body := scrape(t, o)
	require.Contains(t, body, `target_info{a_b="1;2",host_name="h1",instance="h1",job="s",telemetry_source="s"} 1`)
	require.NotContains(t, body, "\nbad{")
	require.Equal(t, 1, logs.FilterMessage("failed to convert metric").Len())
}

// Two series that differ only in a resource attribute promhttp cannot tell
// apart are a gather error: the handler still serves (ContinueOnError) and
// logs it at Error, as the exporter does.
func TestGatherErrorIsLoggedAndServed(t *testing.T) {
	core, logs := observer.New(zap.ErrorLevel)
	o, err := New("127.0.0.1:0", "/metrics", false, 0, embed.TelemetrySettings{}, zap.New(core))
	require.NoError(t, err)
	t.Cleanup(func() { _ = o.Stop(context.Background()) })
	require.NoError(t, o.WriteMetric(context.Background(), gauge("dup", 1, nil, map[string]any{"os.type": "linux"})))
	require.NoError(t, o.WriteMetric(context.Background(), gauge("dup", 2, nil, map[string]any{"os.type": "windows"})))
	require.Contains(t, scrape(t, o), "\ndup{")
	require.Eventually(t, func() bool { return logs.Len() > 0 }, 2*time.Second, 10*time.Millisecond)
	require.True(t, strings.Contains(logs.All()[0].Message, "collected before"), logs.All()[0].Message)
}

// Concurrent writes and scrapes are safe (run with -race).
func TestConcurrentWritesAndScrapes(t *testing.T) {
	o := newTestOutput(t)
	var wg sync.WaitGroup
	for i := range 4 {
		wg.Add(2)
		go func() {
			defer wg.Done()
			for range 50 {
				assert.NoError(t, o.WriteMetric(context.Background(), gauge("c", float64(i), map[string]string{"w": string(rune('a' + i))}, nil)))
			}
		}()
		go func() {
			defer wg.Done()
			resp, err := http.Get("http://" + o.Addr() + "/metrics")
			if assert.NoError(t, err) {
				assert.Equal(t, http.StatusOK, resp.StatusCode)
				_ = resp.Body.Close()
			}
		}()
	}
	wg.Wait()
}

// A non-monotonic sum is exposed as a gauge, so it shares a name's type with
// a gauge.
func TestSumIsAGauge(t *testing.T) {
	o := newTestOutput(t)
	n := int64(3)
	require.NoError(t, o.WriteMetric(context.Background(), gauge("level", 1, map[string]string{"k": "g"}, nil)))
	require.NoError(t, o.WriteMetric(context.Background(), output.MetricRecord{
		Name: "level", Type: embed.MetricTypeSum, IntValue: &n,
		Metadata: output.MetricPointMetadata{Timestamp: time.Unix(1700000000, 0), Attributes: map[string]string{"k": "s"}},
	}))
	body := scrape(t, o)
	require.Contains(t, body, "# TYPE level gauge\n")
	require.Contains(t, body, `level{k="s",otel_scope_name="",otel_scope_schema_url="",otel_scope_version=""} 3`)
}

// A target whose labels client_golang refuses (invalid UTF-8) is skipped
// with an Error; its series still serve.
func TestInvalidTargetInfoIsSkipped(t *testing.T) {
	core, logs := observer.New(zap.ErrorLevel)
	o, err := New("127.0.0.1:0", "/metrics", false, 0, embed.TelemetrySettings{}, zap.New(core))
	require.NoError(t, err)
	t.Cleanup(func() { _ = o.Stop(context.Background()) })
	require.NoError(t, o.WriteMetric(context.Background(), gauge("m", 1, nil, map[string]any{"host.name": "h", "telemetry.source": "s", "os.type": "\xff"})))
	scrape(t, o)
	require.Equal(t, 1, logs.FilterMessage("failed to convert target_info").Len())
}

// A resource key that is no valid label name drops every target_info from
// the scrape, as the exporter's createTargetInfoMetrics returns nothing on
// that error.
func TestBadResourceKeyDropsEveryTargetInfo(t *testing.T) {
	core, logs := observer.New(zap.ErrorLevel)
	o, err := New("127.0.0.1:0", "/metrics", false, 0, embed.TelemetrySettings{}, zap.New(core))
	require.NoError(t, err)
	t.Cleanup(func() { _ = o.Stop(context.Background()) })
	require.NoError(t, o.WriteMetric(context.Background(), gauge("m", 1, nil, map[string]any{"host.name": "h1", "telemetry.source": "s"})))
	require.NoError(t, o.WriteMetric(context.Background(), gauge("m", 1, nil, map[string]any{"host.name": "h2", "telemetry.source": "s", "": "x"})))
	body := scrape(t, o)
	require.NotContains(t, body, "target_info")
	require.Contains(t, body, `instance="h1"`)
	require.Equal(t, 1, logs.FilterMessage("failed to convert target_info").Len())
}

// The server stopping for any reason but Stop is logged at Warn.
func TestServeFailureIsLogged(t *testing.T) {
	core, logs := observer.New(zap.WarnLevel)
	o, err := New("127.0.0.1:0", "/metrics", false, 0, embed.TelemetrySettings{}, zap.New(core))
	require.NoError(t, err)
	require.NoError(t, o.ln.Close())
	require.Eventually(t, func() bool { return logs.FilterMessage("prometheus-scrape server stopped").Len() == 1 }, 2*time.Second, 10*time.Millisecond)
	_ = o.Stop(context.Background())
}

// Describe sends nothing: the collector is unchecked, so promhttp gathers
// whatever Collect emits.
func TestDescribeSendsNothing(t *testing.T) {
	ch := make(chan *prometheus.Desc, 1)
	(&collector{}).Describe(ch)
	require.Empty(t, ch)
}

// series_exposed counts the Prometheus series a scrape exposes: a gauge is
// one, and a histogram with two bounds is five (three buckets with +Inf, sum,
// and count). A repeated bound is one bucket, so it adds no series.
func TestSeriesExposedCountsEverySeries(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	tel := embed.TelemetrySettings{MeterProvider: sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))}
	o, err := newWithClock("127.0.0.1:0", "", false, 0, tel, nil, clockwork.NewRealClock())
	require.NoError(t, err)
	t.Cleanup(func() { _ = o.Stop(context.Background()) })
	require.NoError(t, o.WriteMetric(context.Background(), gauge("a", 1, nil, nil)))
	require.NoError(t, o.WriteMetric(context.Background(), output.MetricRecord{
		Name: "rpc.duration", Unit: "s", Type: embed.MetricTypeHistogram,
		HistogramCount: 6, HistogramSum: 2.5,
		HistogramBucketBounds: []float64{0.1, 1}, HistogramBucketCounts: []uint64{1, 2, 3},
		Metadata: output.MetricPointMetadata{Timestamp: time.Unix(1700000000, 0)},
	}))
	require.NoError(t, o.WriteMetric(context.Background(), output.MetricRecord{
		Name: "rpc.size", Unit: "By", Type: embed.MetricTypeHistogram,
		HistogramCount: 7, HistogramSum: 9,
		HistogramBucketBounds: []float64{1, 1, 10}, HistogramBucketCounts: []uint64{1, 2, 3, 1},
		Metadata: output.MetricPointMetadata{Timestamp: time.Unix(1700000000, 0)},
	}))
	scrape(t, o)

	got := collect(t, reader)
	require.Equal(t, int64(11), got["blitz.output.prometheus_scrape.series_exposed"].(metricdata.Gauge[int64]).DataPoints[0].Value)
}

// Writes prune expired series, so the store stays bounded when nothing
// scrapes.
func TestPutPrunesExpiredSeries(t *testing.T) {
	clk := clockwork.NewFakeClock()
	s := newStore(clk, time.Minute)
	s.put(embed.MetricPoint{Name: "old"})
	clk.Advance(2 * time.Minute)
	s.put(embed.MetricPoint{Name: "fresh"})
	s.mu.Lock()
	defer s.mu.Unlock()
	require.Len(t, s.entries, 1)
	require.Contains(t, s.entries, signature(embed.MetricPoint{Name: "fresh"}))
}

// blockingWriter holds its first body write until release closes.
type blockingWriter struct {
	http.ResponseWriter
	wrote   chan struct{}
	release chan struct{}
	once    sync.Once
}

func (w *blockingWriter) Write(b []byte) (int, error) {
	w.once.Do(func() { close(w.wrote); <-w.release })
	return w.ResponseWriter.Write(b)
}

// A scraper that stops reading stalls only its own scrape, and each scrape
// records the series count of its own Collect, before the response is written.
func TestStalledScrapeDoesNotBlockOthers(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	tel := embed.TelemetrySettings{MeterProvider: sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))}
	o, err := newWithClock("127.0.0.1:0", "", false, 0, tel, nil, clockwork.NewRealClock())
	require.NoError(t, err)
	t.Cleanup(func() { _ = o.Stop(context.Background()) })
	exposed := func() int64 {
		g, ok := collect(t, reader)["blitz.output.prometheus_scrape.series_exposed"].(metricdata.Gauge[int64])
		if !ok {
			return -1
		}
		return g.DataPoints[0].Value
	}
	req := func() *http.Request { return httptest.NewRequest(http.MethodGet, "/metrics", nil) }

	require.NoError(t, o.WriteMetric(context.Background(), gauge("a", 1, nil, nil)))
	stalled := &blockingWriter{ResponseWriter: httptest.NewRecorder(), wrote: make(chan struct{}), release: make(chan struct{})}
	stalledDone := make(chan struct{})
	go func() { o.handleScrape(stalled, req()); close(stalledDone) }()
	<-stalled.wrote
	require.Equal(t, int64(1), exposed(), "the stalled scrape recorded its own count")

	require.NoError(t, o.WriteMetric(context.Background(), gauge("b", 1, nil, nil)))
	done := make(chan struct{})
	go func() { o.handleScrape(httptest.NewRecorder(), req()); close(done) }()
	require.Eventually(t, func() bool {
		select {
		case <-done:
			return true
		default:
			return false
		}
	}, 5*time.Second, 5*time.Millisecond, "a stalled scrape blocked the next one")
	require.Equal(t, int64(2), exposed())
	close(stalled.release)
	<-stalledDone
}

// TestWriteAfterStopIsRefused pins that a write after Stop returns an error
// and is not stored, as remote-write's is: the endpoint no longer serves it.
func TestWriteAfterStopIsRefused(t *testing.T) {
	o, err := New("127.0.0.1:0", "/metrics", false, 0, embed.TelemetrySettings{}, nil)
	require.NoError(t, err)
	require.NoError(t, o.Stop(context.Background()))
	require.ErrorIs(t, o.WriteMetric(context.Background(), gauge("late", 1, nil, nil)), errStopped)
	require.Empty(t, o.st.snapshot())
}
