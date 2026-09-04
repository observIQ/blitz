package resource

import (
	"reflect"
	"testing"
)

func mapPtr(m map[string]any) uintptr { return reflect.ValueOf(m).Pointer() }

func TestStaticResourcesConstructorCopies(t *testing.T) {
	base := map[string]any{"host.name": "thor-web-01", "telemetry.source": "apache"}
	s := NewStaticResources(base)

	// Mutating the input after construction must not leak into the static set.
	base["host.name"] = "mutated"
	base["injected"] = "x"

	got := s.Record()
	if got["host.name"] != "thor-web-01" {
		t.Errorf("host.name = %q, want thor-web-01 (constructor must copy)", got["host.name"])
	}
	if _, ok := got["injected"]; ok {
		t.Error("post-construction input mutation leaked into the static set")
	}
}

func TestStaticResourcesRecordNoDynamicIsSharedAndReadOnly(t *testing.T) {
	s := NewStaticResources(map[string]any{"host.name": "thor-web-01", "telemetry.source": "apache"})

	a := s.Record()
	b := s.Record()

	// Zero-allocation path: repeated no-dynamic calls return the SAME map.
	if mapPtr(a) != mapPtr(b) {
		t.Error("Record() with no dynamic pairs should return the shared static map, not a fresh copy")
	}
	if a["telemetry.source"] != "apache" {
		t.Errorf("telemetry.source = %q, want apache", a["telemetry.source"])
	}
}

func TestStaticResourcesRecordWithDynamicMergesWithoutMutatingStatic(t *testing.T) {
	s := NewStaticResources(map[string]any{"host.name": "thor-web-01", "telemetry.source": "wel"})

	rec := s.Record("wel.channel", "Security", "wel.role", "dc")

	// Merged map carries both static and dynamic.
	if rec["host.name"] != "thor-web-01" || rec["wel.channel"] != "Security" || rec["wel.role"] != "dc" {
		t.Errorf("merged record missing expected keys: %#v", rec)
	}
	// It must be a distinct map from the shared static one.
	if mapPtr(rec) == mapPtr(s.Record()) {
		t.Error("Record(dynamic...) must return a fresh map, not the shared static map")
	}
	// The static set must be untouched by the merge.
	if _, ok := s.Record()["wel.channel"]; ok {
		t.Error("dynamic pair leaked into the static set")
	}
}

func TestStaticResourcesRecordOddArgsIgnoresTrailing(t *testing.T) {
	s := NewStaticResources(map[string]any{"telemetry.source": "json"})

	// A single unpaired arg is treated as "no complete dynamic pair": shared static.
	if mapPtr(s.Record("dangling")) != mapPtr(s.Record()) {
		t.Error("a single unpaired dynamic arg should yield the shared static map")
	}
	// An odd count keeps complete pairs and drops the trailing unpaired key.
	rec := s.Record("json.type", "pii", "dangling")
	if rec["json.type"] != "pii" {
		t.Errorf("json.type = %q, want pii", rec["json.type"])
	}
	if _, ok := rec["dangling"]; ok {
		t.Error("trailing unpaired key should be dropped")
	}
}
