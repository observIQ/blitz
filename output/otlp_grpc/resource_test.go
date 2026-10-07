package otlpgrpc

import (
	"context"
	"testing"
	"time"

	"github.com/observiq/blitz/output"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	logspb "go.opentelemetry.io/proto/otlp/logs/v1"
	metricspb "go.opentelemetry.io/proto/otlp/metrics/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
	"go.uber.org/zap"
)

func attrString(t *testing.T, attrs []*commonpb.KeyValue, key string) (string, bool) {
	t.Helper()
	for _, kv := range attrs {
		if kv.Key == key {
			sv, ok := kv.Value.Value.(*commonpb.AnyValue_StringValue)
			if !ok {
				t.Fatalf("attribute %q should be a string, got %T", key, kv.Value.Value)
			}
			return sv.StringValue, true
		}
	}
	return "", false
}

func assertResource(t *testing.T, attrs []*commonpb.KeyValue, want map[string]string) {
	t.Helper()
	for key, w := range want {
		got, ok := attrString(t, attrs, key)
		if !ok || got != w {
			t.Errorf("resource %q = %q (present %v), want %q", key, got, ok, w)
		}
	}
}

func newTestOutput(t *testing.T) *OTLPGrpc {
	t.Helper()
	o, err := New(zap.NewNop())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return o
}

// A log record's Metadata.Resource must become the OTLP resource, so
// collectors can route on resource.attributes["telemetry.source"].
func TestWrite_LogResourceReachesOTLPResource(t *testing.T) {
	o := newTestOutput(t)

	err := o.Write(context.Background(), output.LogRecord{
		Message: "GET / 200",
		Metadata: output.LogRecordMetadata{
			Timestamp:  time.Now(),
			Resource:   map[string]any{"host.name": "web-01", "telemetry.source": "nginx"},
			Attributes: map[string]any{"http.method": "GET"},
		},
	})
	if err != nil {
		t.Fatalf("Write: %v", err)
	}

	req := o.buildOTLPRequest([]*entry[*logspb.LogRecord]{<-o.dataChan})
	if len(req.ResourceLogs) != 1 {
		t.Fatalf("want 1 ResourceLogs, got %d", len(req.ResourceLogs))
	}
	rl := req.ResourceLogs[0]
	assertResource(t, rl.Resource.Attributes, map[string]string{
		"service.name":     "blitz",
		"host.name":        "web-01",
		"telemetry.source": "nginx",
	})

	if got, _ := attrString(t, rl.ScopeLogs[0].LogRecords[0].Attributes, "http.method"); got != "GET" {
		t.Errorf("record attribute http.method = %q, want GET", got)
	}
}

// Records from different generators sharing one worker batch must not be
// merged under a single resource.
func TestBuildOTLPRequest_GroupsByResource(t *testing.T) {
	o := &OTLPGrpc{}
	entries := []*entry[*logspb.LogRecord]{
		newEntry(&logspb.LogRecord{}, map[string]any{"telemetry.source": "nginx"}),
		newEntry(&logspb.LogRecord{}, map[string]any{"telemetry.source": "okta"}),
		newEntry(&logspb.LogRecord{}, map[string]any{"telemetry.source": "nginx"}),
		newEntry(&logspb.LogRecord{}, nil),
		nil,
	}

	req := o.buildOTLPRequest(entries)
	if len(req.ResourceLogs) != 3 {
		t.Fatalf("want 3 ResourceLogs (nginx, okta, none), got %d", len(req.ResourceLogs))
	}

	want := []struct {
		source string
		count  int
	}{{"nginx", 2}, {"okta", 1}, {"", 1}}
	for i, w := range want {
		rl := req.ResourceLogs[i]
		if got, _ := attrString(t, rl.Resource.Attributes, "telemetry.source"); got != w.source {
			t.Errorf("ResourceLogs[%d] telemetry.source = %q, want %q", i, got, w.source)
		}
		if n := len(rl.ScopeLogs[0].LogRecords); n != w.count {
			t.Errorf("ResourceLogs[%d] has %d records, want %d", i, n, w.count)
		}
	}

	// A record with no resource still identifies itself as blitz.
	if got, _ := attrString(t, req.ResourceLogs[2].Resource.Attributes, "service.name"); got != "blitz" {
		t.Errorf("resource-less ResourceLogs service.name = %q, want blitz", got)
	}
}

// A metric's Metadata.Resource must become the OTLP resource, so host metrics
// arrive with host.name, and metrics from different resources stay apart.
func TestWriteMetric_ResourceReachesOTLPResource(t *testing.T) {
	o := newTestOutput(t)
	v := int64(1)

	for _, host := range []string{"host-a", "host-b", "host-a"} {
		err := o.WriteMetric(context.Background(), output.MetricRecord{
			Name:     "system.cpu.time",
			Type:     output.MetricTypeSum,
			IntValue: &v,
			Metadata: output.MetricPointMetadata{
				Timestamp: time.Now(),
				Resource:  map[string]any{"host.name": host, "telemetry.source": "hostmetrics"},
			},
		})
		if err != nil {
			t.Fatalf("WriteMetric: %v", err)
		}
	}

	rms := buildMetricRequests([]*entry[*metricspb.Metric]{<-o.metricChan, <-o.metricChan, <-o.metricChan})
	if len(rms) != 2 {
		t.Fatalf("want 2 ResourceMetrics (host-a, host-b), got %d", len(rms))
	}
	assertResource(t, rms[0].Resource.Attributes, map[string]string{
		"service.name":     "blitz",
		"host.name":        "host-a",
		"telemetry.source": "hostmetrics",
	})
	if n := len(rms[0].ScopeMetrics[0].Metrics); n != 2 {
		t.Errorf("host-a ResourceMetrics has %d metrics, want 2", n)
	}
	assertResource(t, rms[1].Resource.Attributes, map[string]string{"host.name": "host-b"})
}

// A span's Metadata.Resource must become the OTLP resource, including a
// generator-provided service.name in place of the blitz default.
func TestWriteTrace_ResourceReachesOTLPResource(t *testing.T) {
	o := newTestOutput(t)

	err := o.WriteTrace(context.Background(), output.TraceRecord{
		TraceID:   "0102030405060708090a0b0c0d0e0f10",
		SpanID:    "0102030405060708",
		Name:      "GET /checkout",
		StartTime: time.Now(),
		EndTime:   time.Now(),
		Metadata: output.SpanMetadata{
			Resource: map[string]any{"service.name": "checkout", "host.name": "app-01"},
		},
	})
	if err != nil {
		t.Fatalf("WriteTrace: %v", err)
	}

	rss := buildTraceRequests([]*entry[*tracepb.Span]{<-o.traceChan})
	if len(rss) != 1 {
		t.Fatalf("want 1 ResourceSpans, got %d", len(rss))
	}
	assertResource(t, rss[0].Resource.Attributes, map[string]string{
		"service.name": "checkout",
		"host.name":    "app-01",
	})
}

// A generator that sets its own service.name must not get a second one.
func TestResourceAttributes_KeepsGeneratorServiceName(t *testing.T) {
	n := 0
	for _, kv := range resourceAttributes(map[string]any{"service.name": "checkout"}) {
		if kv.Key == "service.name" {
			n++
			if kv.Value.GetStringValue() != "checkout" {
				t.Errorf("service.name = %q, want checkout", kv.Value.GetStringValue())
			}
		}
	}
	if n != 1 {
		t.Errorf("want exactly one service.name attribute, got %d", n)
	}
}
