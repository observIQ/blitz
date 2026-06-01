package govbonds

import (
	"math/rand"
	"testing"

	"github.com/observiq/blitz/generator/fix/catalog"
	"github.com/observiq/blitz/generator/fix/catalog/v44/app"
)

func reregisterAll() {
	app.Reregister()
	registerAll()
}

func TestEveryGovBondSecurityTypeHasInstruments(t *testing.T) {
	for _, st := range catalog.SecurityTypesByCategory(catalog.AssetCategoryGovBonds) {
		if len(govBondsByType[st]) == 0 {
			t.Errorf("GovBonds SecurityType %q has no bonds", st)
		}
		for i, b := range govBondsByType[st] {
			if b.Symbol == "" {
				t.Errorf("%s row %d: empty Symbol", st, i)
			}
			if len(b.Cusip) != 9 {
				t.Errorf("%s row %d (%s): Cusip %q must be 9 chars", st, i, b.Symbol, b.Cusip)
			}
			if len(b.CFICode) != 6 {
				t.Errorf("%s row %d (%s): CFICode %q must be 6 chars", st, i, b.Symbol, b.CFICode)
			}
		}
	}
}

func TestAllGovBondsMessagesRegistered(t *testing.T) {
	reregisterAll()
	for _, mt := range []string{
		app.MsgTypeNewOrderSingle, app.MsgTypeExecutionReport,
		app.MsgTypeOrderCancelRequest, app.MsgTypeOrderCancelReplaceRequest,
		app.MsgTypeOrderStatusRequest,
	} {
		if catalog.Get(catalog.MessageKey{Version: catalog.V44, MsgType: mt, AssetCategory: catalog.AssetCategoryGovBonds}) == nil {
			t.Errorf("GovBonds %s not registered", mt)
		}
	}
}

func TestGovBondsInstrumentBlock(t *testing.T) {
	reregisterAll()
	def := catalog.Get(catalog.MessageKey{Version: catalog.V44, MsgType: app.MsgTypeNewOrderSingle, AssetCategory: catalog.AssetCategoryGovBonds})
	if def == nil {
		t.Fatal("GovBonds NewOrderSingle not registered")
	}
	tags := buildTagMap(def.Fields, rand.New(rand.NewSource(1)))
	required := []catalog.Tag{
		app.TagSymbol, app.TagSecurityType, TagSecurityID, TagSecurityIDSrc,
		TagCouponRate, TagMaturityDate, TagIssueDate, TagFactor,
		TagCountryOfIssue, TagYieldType, TagCFICode,
	}
	for _, want := range required {
		if _, ok := tags[want]; !ok {
			t.Errorf("GovBonds NewOrderSingle missing tag %d", want)
		}
	}
	if tags[TagCountryOfIssue] != "US" {
		t.Errorf("CountryOfIssue = %q, want US", tags[TagCountryOfIssue])
	}
}

func TestGovBondsSecurityTypeStaysInCategory(t *testing.T) {
	reregisterAll()
	def := catalog.Get(catalog.MessageKey{Version: catalog.V44, MsgType: app.MsgTypeNewOrderSingle, AssetCategory: catalog.AssetCategoryGovBonds})
	for seed := int64(1); seed <= 100; seed++ {
		tags := buildTagMap(def.Fields, rand.New(rand.NewSource(seed)))
		st := catalog.SecurityType(tags[app.TagSecurityType])
		if st.Category() != catalog.AssetCategoryGovBonds {
			t.Errorf("seed=%d: SecurityType %q has category %s, want GovBonds", seed, st, st.Category())
		}
	}
}

func TestGovBondsDeterminismFromSeed(t *testing.T) {
	reregisterAll()
	def := catalog.Get(catalog.MessageKey{Version: catalog.V44, MsgType: app.MsgTypeNewOrderSingle, AssetCategory: catalog.AssetCategoryGovBonds})
	a := buildTagMap(def.Fields, rand.New(rand.NewSource(42)))
	b := buildTagMap(def.Fields, rand.New(rand.NewSource(42)))
	for k, va := range a {
		if vb, ok := b[k]; !ok || va != vb {
			t.Errorf("seed-42 disagreement at tag %d: %q vs %q", k, va, vb)
		}
	}
}

func buildTagMap(gens []catalog.FieldGenerator, r *rand.Rand) map[catalog.Tag]string {
	out := make(map[catalog.Tag]string, len(gens))
	ctx := &catalog.GenerateCtx{Version: catalog.V44, AssetCategory: catalog.AssetCategoryGovBonds}
	for _, g := range gens {
		f := g(r, ctx)
		out[f.Tag] = f.Value
	}
	return out
}
