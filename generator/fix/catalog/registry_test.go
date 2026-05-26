package catalog

import (
	"strings"
	"testing"
)

func TestRegistryRoundTrip(t *testing.T) {
	ResetForTest()
	def := MessageDefinition{
		Version:       V44,
		MsgType:       "0", // Heartbeat
		AssetCategory: AssetCategoryUnknown,
	}
	Register(def)

	got := Get(MessageKey{Version: V44, MsgType: "0", AssetCategory: AssetCategoryUnknown})
	if got == nil {
		t.Fatal("Get returned nil for registered definition")
	}
	if got.MsgType != "0" {
		t.Errorf("retrieved MsgType = %q, want %q", got.MsgType, "0")
	}
}

func TestRegistryMissReturnsNil(t *testing.T) {
	ResetForTest()
	if got := Get(MessageKey{Version: V44, MsgType: "X", AssetCategory: AssetCategoryEquities}); got != nil {
		t.Errorf("Get for unregistered key returned %+v, want nil", got)
	}
}

func TestRegistryDuplicatePanics(t *testing.T) {
	ResetForTest()
	def := MessageDefinition{
		Version:       V44,
		MsgType:       "D",
		AssetCategory: AssetCategoryEquities,
	}
	Register(def)
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic on duplicate registration")
		}
		s, ok := r.(string)
		if !ok {
			t.Fatalf("panic value not string: %v", r)
		}
		if !strings.Contains(s, "duplicate") {
			t.Errorf("panic message %q does not mention duplicate", s)
		}
	}()
	Register(def)
}

func TestSameMsgTypeDifferentCategoryCoexist(t *testing.T) {
	ResetForTest()
	Register(MessageDefinition{Version: V44, MsgType: "D", AssetCategory: AssetCategoryEquities})
	Register(MessageDefinition{Version: V44, MsgType: "D", AssetCategory: AssetCategoryFX})

	eq := Get(MessageKey{Version: V44, MsgType: "D", AssetCategory: AssetCategoryEquities})
	fx := Get(MessageKey{Version: V44, MsgType: "D", AssetCategory: AssetCategoryFX})
	if eq == nil || fx == nil {
		t.Fatalf("expected both definitions to coexist, got eq=%v fx=%v", eq, fx)
	}
	if eq.AssetCategory == fx.AssetCategory {
		t.Errorf("definitions collapsed into one bucket")
	}
}

func TestAllDefinitionsReturnsSnapshot(t *testing.T) {
	ResetForTest()
	Register(MessageDefinition{Version: V44, MsgType: "0", AssetCategory: AssetCategoryUnknown})
	Register(MessageDefinition{Version: V44, MsgType: "1", AssetCategory: AssetCategoryUnknown})

	defs := AllDefinitions()
	if len(defs) != 2 {
		t.Fatalf("AllDefinitions() = %d, want 2", len(defs))
	}

	// Mutating the returned slice must not affect the registry.
	defs[0] = nil
	defs2 := AllDefinitions()
	if len(defs2) != 2 {
		t.Errorf("registry corrupted by external mutation; AllDefinitions now = %d", len(defs2))
	}
}
