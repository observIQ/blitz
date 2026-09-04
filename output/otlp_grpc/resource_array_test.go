package otlpgrpc

import (
	"testing"

	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
)

// A []string resource attribute (e.g. host.ip / host.mac) must serialize to an
// OTLP ArrayValue of StringValues, not a stringified slice (PIPE-1253).
func TestBuildMetricRequest_ArrayResourceAttribute(t *testing.T) {
	rm := buildMetricRequest(nil, map[string]any{
		"host.ip": []string{"10.0.0.1", "10.0.0.2"},
	})

	var hostIP *commonpb.AnyValue
	for _, kv := range rm.Resource.Attributes {
		if kv.Key == "host.ip" {
			hostIP = kv.Value
		}
	}
	if hostIP == nil {
		t.Fatal("host.ip resource attribute not found")
	}

	arr, ok := hostIP.Value.(*commonpb.AnyValue_ArrayValue)
	if !ok {
		t.Fatalf("host.ip should serialize as an ArrayValue, got %T", hostIP.Value)
	}
	if len(arr.ArrayValue.Values) != 2 {
		t.Fatalf("host.ip array should have 2 elements, got %d", len(arr.ArrayValue.Values))
	}

	got := make([]string, 0, 2)
	for _, v := range arr.ArrayValue.Values {
		sv, ok := v.Value.(*commonpb.AnyValue_StringValue)
		if !ok {
			t.Fatalf("array element should be a StringValue, got %T", v.Value)
		}
		got = append(got, sv.StringValue)
	}
	if got[0] != "10.0.0.1" || got[1] != "10.0.0.2" {
		t.Errorf("array elements = %v, want [10.0.0.1 10.0.0.2]", got)
	}
}
