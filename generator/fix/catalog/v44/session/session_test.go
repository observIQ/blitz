package session

import (
	"math/rand"
	"testing"

	"github.com/observiq/blitz/generator/fix/catalog"
)

func TestAllSessionMessagesRegistered(t *testing.T) {
	Reregister()

	wanted := []string{
		MsgTypeLogon,
		MsgTypeHeartbeat,
		MsgTypeLogout,
		MsgTypeResendRequest,
		MsgTypeSequenceReset,
		MsgTypeTestRequest,
		MsgTypeReject,
	}
	for _, mt := range wanted {
		def := catalog.Get(catalog.MessageKey{
			Version:       catalog.V44,
			MsgType:       mt,
			AssetCategory: catalog.AssetCategoryUnknown,
		})
		if def == nil {
			t.Errorf("MsgType %q not registered", mt)
		}
	}
}

func TestLogonHasEncryptAndHeartbeat(t *testing.T) {
	Reregister()
	def := catalog.Get(catalog.MessageKey{Version: catalog.V44, MsgType: MsgTypeLogon, AssetCategory: catalog.AssetCategoryUnknown})
	if def == nil {
		t.Fatal("Logon not registered")
	}

	r := rand.New(rand.NewSource(1))
	ctx := &catalog.GenerateCtx{Version: catalog.V44}

	tags := make(map[catalog.Tag]string)
	for _, g := range def.Fields {
		f := g(r, ctx)
		tags[f.Tag] = f.Value
	}

	if v, ok := tags[TagEncryptMethod]; !ok || v != "0" {
		t.Errorf("Logon EncryptMethod = %q (ok=%v), want \"0\"", v, ok)
	}
	if v, ok := tags[TagHeartBtInt]; !ok || v != "30" {
		t.Errorf("Logon HeartBtInt = %q (ok=%v), want \"30\"", v, ok)
	}
}

func TestHeartbeatBodyEmpty(t *testing.T) {
	Reregister()
	def := catalog.Get(catalog.MessageKey{Version: catalog.V44, MsgType: MsgTypeHeartbeat, AssetCategory: catalog.AssetCategoryUnknown})
	if def == nil {
		t.Fatal("Heartbeat not registered")
	}
	if len(def.Fields) != 0 {
		t.Errorf("Heartbeat body should be empty, got %d fields", len(def.Fields))
	}
}

func TestTestRequestIDFormatAndDeterminism(t *testing.T) {
	Reregister()
	def := catalog.Get(catalog.MessageKey{Version: catalog.V44, MsgType: MsgTypeTestRequest, AssetCategory: catalog.AssetCategoryUnknown})
	if def == nil {
		t.Fatal("TestRequest not registered")
	}
	if len(def.Fields) != 1 {
		t.Fatalf("TestRequest body should have 1 field, got %d", len(def.Fields))
	}

	// Same seed → same TestReqID.
	a := def.Fields[0](rand.New(rand.NewSource(42)), &catalog.GenerateCtx{})
	b := def.Fields[0](rand.New(rand.NewSource(42)), &catalog.GenerateCtx{})
	if a != b {
		t.Errorf("TestRequest not deterministic: %+v vs %+v", a, b)
	}
	if a.Tag != TagTestReqID {
		t.Errorf("TestRequest field tag = %d, want %d", a.Tag, TagTestReqID)
	}
	if len(a.Value) != 8 || a.Value[:3] != "TR-" {
		t.Errorf("TestRequest value = %q, want TR-NNNNN", a.Value)
	}

	// Different seed → different value (sanity check the RNG is wired).
	c := def.Fields[0](rand.New(rand.NewSource(7)), &catalog.GenerateCtx{})
	if a == c {
		t.Errorf("TestRequest values identical across different seeds: %+v", a)
	}
}

func TestRejectCarriesRefSeqNumAndReason(t *testing.T) {
	Reregister()
	def := catalog.Get(catalog.MessageKey{Version: catalog.V44, MsgType: MsgTypeReject, AssetCategory: catalog.AssetCategoryUnknown})
	if def == nil {
		t.Fatal("Reject not registered")
	}

	r := rand.New(rand.NewSource(1))
	ctx := &catalog.GenerateCtx{Version: catalog.V44}
	tags := make(map[catalog.Tag]string)
	for _, g := range def.Fields {
		f := g(r, ctx)
		tags[f.Tag] = f.Value
	}

	if _, ok := tags[TagRefSeqNum]; !ok {
		t.Error("Reject missing RefSeqNum (45)")
	}
	if _, ok := tags[TagSessionRejectReason]; !ok {
		t.Error("Reject missing SessionRejectReason (373)")
	}
}

func TestResendRequestHasSeqRange(t *testing.T) {
	Reregister()
	def := catalog.Get(catalog.MessageKey{Version: catalog.V44, MsgType: MsgTypeResendRequest, AssetCategory: catalog.AssetCategoryUnknown})
	if def == nil {
		t.Fatal("ResendRequest not registered")
	}
	r := rand.New(rand.NewSource(1))
	tags := make(map[catalog.Tag]bool)
	for _, g := range def.Fields {
		f := g(r, &catalog.GenerateCtx{Version: catalog.V44})
		tags[f.Tag] = true
	}
	for _, want := range []catalog.Tag{TagBeginSeqNo, TagEndSeqNo} {
		if !tags[want] {
			t.Errorf("ResendRequest missing tag %d", want)
		}
	}
}
