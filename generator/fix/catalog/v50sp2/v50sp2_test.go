package v50sp2

import (
	"math/rand"
	"testing"

	"github.com/observiq/blitz/generator/fix/catalog"
	"github.com/observiq/blitz/generator/fix/catalog/v44/app"
)

func TestV50SP2EntryExistsForEveryV44Entry(t *testing.T) {
	for _, def := range catalog.AllDefinitions() {
		if def.Version != catalog.V44 {
			continue
		}
		if catalog.Get(catalog.MessageKey{
			Version: catalog.V50SP2, MsgType: def.MsgType, AssetCategory: def.AssetCategory,
		}) == nil {
			t.Errorf("V44 entry (%s, %s) has no V50SP2 mirror", def.MsgType, def.AssetCategory)
		}
	}
}

func TestApplVerIDOnApplicationMessages(t *testing.T) {
	def := catalog.Get(catalog.MessageKey{
		Version: catalog.V50SP2, MsgType: app.MsgTypeNewOrderSingle, AssetCategory: catalog.AssetCategoryEquities,
	})
	if def == nil {
		t.Fatal("V50SP2 Equities NewOrderSingle not registered")
	}
	r := rand.New(rand.NewSource(1))
	ctx := &catalog.GenerateCtx{Version: catalog.V50SP2, AssetCategory: catalog.AssetCategoryEquities}
	if first := def.Fields[0](r, ctx); first.Tag != TagApplVerID || first.Value != ApplVerIDFIX50SP2 {
		t.Errorf("V50SP2 NewOrderSingle first field = %+v, want ApplVerID=9", first)
	}
}

func TestDefaultApplVerIDOnLogon(t *testing.T) {
	def := catalog.Get(catalog.MessageKey{
		Version: catalog.V50SP2, MsgType: "A", AssetCategory: catalog.AssetCategoryUnknown,
	})
	if def == nil {
		t.Fatal("V50SP2 Logon not registered")
	}
	r := rand.New(rand.NewSource(1))
	ctx := &catalog.GenerateCtx{Version: catalog.V50SP2}
	var hasDefaultApplVerID bool
	for _, g := range def.Fields {
		f := g(r, ctx)
		if f.Tag == TagDefaultApplVerID && f.Value == ApplVerIDFIX50SP2 {
			hasDefaultApplVerID = true
		}
	}
	if !hasDefaultApplVerID {
		t.Error("V50SP2 Logon missing DefaultApplVerID=9")
	}
}

func TestSessionMessagesDoNotCarryApplVerID(t *testing.T) {
	// Heartbeats and ResendRequests under FIXT do NOT carry ApplVerID.
	for _, mt := range []string{"0", "1", "2", "3", "4", "5"} {
		def := catalog.Get(catalog.MessageKey{
			Version: catalog.V50SP2, MsgType: mt, AssetCategory: catalog.AssetCategoryUnknown,
		})
		if def == nil {
			continue
		}
		r := rand.New(rand.NewSource(1))
		ctx := &catalog.GenerateCtx{Version: catalog.V50SP2}
		for _, g := range def.Fields {
			f := g(r, ctx)
			if f.Tag == TagApplVerID {
				t.Errorf("V50SP2 session msg %s incorrectly carries ApplVerID", mt)
			}
		}
	}
}

func TestV50SP2BeginStringIsFIXT(t *testing.T) {
	if catalog.V50SP2.BeginString() != "FIXT.1.1" {
		t.Errorf("V50SP2.BeginString() = %q, want FIXT.1.1", catalog.V50SP2.BeginString())
	}
}

func TestV50SP2ApplVerIDIs9(t *testing.T) {
	if catalog.V50SP2.ApplVerID() != "9" {
		t.Errorf("V50SP2.ApplVerID() = %q, want 9", catalog.V50SP2.ApplVerID())
	}
}
