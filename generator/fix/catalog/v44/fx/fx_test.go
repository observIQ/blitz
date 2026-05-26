package fx

import (
	"math/rand"
	"strings"
	"testing"

	"github.com/observiq/blitz/generator/fix/catalog"
	"github.com/observiq/blitz/generator/fix/catalog/v44/app"
)

func reregisterAll() {
	app.Reregister()
	registerAll()
}

func TestEveryFXSecurityTypeHasPairs(t *testing.T) {
	for _, st := range catalog.SecurityTypesByCategory(catalog.AssetCategoryFX) {
		if len(fxPairs[st]) == 0 {
			t.Errorf("FX SecurityType %q has no pairs in fxPairs table", st)
		}
		for i, p := range fxPairs[st] {
			if len(p.CCY1) != 3 || len(p.CCY2) != 3 {
				t.Errorf("FX %s row %d: CCY codes %q/%q must be ISO 4217 (3 chars)", st, i, p.CCY1, p.CCY2)
			}
			if p.CCY1 == p.CCY2 {
				t.Errorf("FX %s row %d: identical CCY codes %q/%q", st, i, p.CCY1, p.CCY2)
			}
		}
	}
}

func TestAllFXMessagesRegistered(t *testing.T) {
	reregisterAll()
	for _, mt := range []string{
		app.MsgTypeNewOrderSingle, app.MsgTypeExecutionReport,
		app.MsgTypeOrderCancelRequest, app.MsgTypeOrderCancelReplaceRequest,
		app.MsgTypeOrderStatusRequest,
	} {
		if catalog.Get(catalog.MessageKey{Version: catalog.V44, MsgType: mt, AssetCategory: catalog.AssetCategoryFX}) == nil {
			t.Errorf("FX %s not registered", mt)
		}
	}
}

func TestFXNewOrderSingleHasInstrumentBlock(t *testing.T) {
	reregisterAll()
	def := catalog.Get(catalog.MessageKey{Version: catalog.V44, MsgType: app.MsgTypeNewOrderSingle, AssetCategory: catalog.AssetCategoryFX})
	if def == nil {
		t.Fatal("FX NewOrderSingle not registered")
	}
	tags := buildTagMap(def.Fields, rand.New(rand.NewSource(1)))
	required := []catalog.Tag{
		app.TagClOrdID, app.TagSymbol, TagSecurityID, TagSecurityIDSrc,
		app.TagSecurityType, TagCurrency, TagSettlCurrency,
		app.TagSide, app.TagOrdType, app.TagTimeInForce,
		app.TagOrderQty, app.TagPrice, app.TagTransactTime,
		TagSettlDate, TagSettlType,
	}
	for _, want := range required {
		if _, ok := tags[want]; !ok {
			t.Errorf("FX NewOrderSingle missing tag %d", want)
		}
	}
}

func TestFXSymbolIsCurrencyPair(t *testing.T) {
	reregisterAll()
	def := catalog.Get(catalog.MessageKey{Version: catalog.V44, MsgType: app.MsgTypeNewOrderSingle, AssetCategory: catalog.AssetCategoryFX})
	for seed := int64(1); seed <= 50; seed++ {
		tags := buildTagMap(def.Fields, rand.New(rand.NewSource(seed)))
		sym := tags[app.TagSymbol]
		if !strings.Contains(sym, "/") {
			t.Errorf("seed=%d: FX Symbol %q must contain /", seed, sym)
		}
		parts := strings.Split(sym, "/")
		if len(parts) != 2 || len(parts[0]) != 3 || len(parts[1]) != 3 {
			t.Errorf("seed=%d: FX Symbol %q must be CCY/CCY ISO-4217", seed, sym)
		}
	}
}

func TestFXSecurityTypeStaysInCategory(t *testing.T) {
	reregisterAll()
	def := catalog.Get(catalog.MessageKey{Version: catalog.V44, MsgType: app.MsgTypeNewOrderSingle, AssetCategory: catalog.AssetCategoryFX})
	for seed := int64(1); seed <= 100; seed++ {
		tags := buildTagMap(def.Fields, rand.New(rand.NewSource(seed)))
		st := catalog.SecurityType(tags[app.TagSecurityType])
		if st.Category() != catalog.AssetCategoryFX {
			t.Errorf("seed=%d: SecurityType %q has category %s, want FX", seed, st, st.Category())
		}
	}
}

func TestFXPriceIs5DecimalPlaces(t *testing.T) {
	reregisterAll()
	def := catalog.Get(catalog.MessageKey{Version: catalog.V44, MsgType: app.MsgTypeNewOrderSingle, AssetCategory: catalog.AssetCategoryFX})
	tags := buildTagMap(def.Fields, rand.New(rand.NewSource(7)))
	price := tags[app.TagPrice]
	dot := strings.Index(price, ".")
	if dot < 0 || len(price)-dot != 6 {
		t.Errorf("FX Price %q must have 5 decimal places", price)
	}
}

func TestFXDeterminismFromSeed(t *testing.T) {
	reregisterAll()
	def := catalog.Get(catalog.MessageKey{Version: catalog.V44, MsgType: app.MsgTypeNewOrderSingle, AssetCategory: catalog.AssetCategoryFX})
	a := buildTagMap(def.Fields, rand.New(rand.NewSource(42)))
	b := buildTagMap(def.Fields, rand.New(rand.NewSource(42)))
	if len(a) != len(b) {
		t.Fatalf("seed-42 outputs differ in length: %d vs %d", len(a), len(b))
	}
	for k, va := range a {
		if vb, ok := b[k]; !ok || va != vb {
			t.Errorf("seed-42 disagreement at tag %d: %q vs %q", k, va, vb)
		}
	}
}

func buildTagMap(gens []catalog.FieldGenerator, r *rand.Rand) map[catalog.Tag]string {
	out := make(map[catalog.Tag]string, len(gens))
	ctx := &catalog.GenerateCtx{Version: catalog.V44, AssetCategory: catalog.AssetCategoryFX}
	for _, g := range gens {
		f := g(r, ctx)
		out[f.Tag] = f.Value
	}
	return out
}
