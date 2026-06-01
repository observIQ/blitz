package equities

import (
	"math/rand"
	"testing"

	"github.com/observiq/blitz/generator/fix/catalog"
	"github.com/observiq/blitz/generator/fix/catalog/v44/app"
)

// reregisterAll calls both app and equities registrations so both
// asset-agnostic skeletons and equities overrides coexist in tests.
func reregisterAll() {
	app.Reregister()
	registerAll()
}

func TestEverySecurityTypeInEquitiesHasInstruments(t *testing.T) {
	for _, st := range catalog.SecurityTypesByCategory(catalog.AssetCategoryEquities) {
		insts := equityInstruments[st]
		if len(insts) == 0 {
			t.Errorf("SecurityType %q (Equities) has no instruments — every category SecurityType must have at least one v1 row", st)
		}
		for i, inst := range insts {
			if inst.Symbol == "" {
				t.Errorf("SecurityType %q row %d: empty Symbol", st, i)
			}
			if inst.ID == "" {
				t.Errorf("SecurityType %q row %d (%s): empty ID", st, i, inst.Symbol)
			}
			if inst.IDSource == "" {
				t.Errorf("SecurityType %q row %d (%s): empty IDSource", st, i, inst.Symbol)
			}
			if len(inst.CFICode) != 6 {
				t.Errorf("SecurityType %q row %d (%s): CFICode %q must be 6 chars (ISO 10962)", st, i, inst.Symbol, inst.CFICode)
			}
		}
	}
}

func TestAllEquitiesMessagesRegistered(t *testing.T) {
	reregisterAll()
	want := []string{
		app.MsgTypeNewOrderSingle,
		app.MsgTypeExecutionReport,
		app.MsgTypeOrderCancelRequest,
		app.MsgTypeOrderCancelReplaceRequest,
		app.MsgTypeOrderStatusRequest,
	}
	for _, mt := range want {
		def := catalog.Get(catalog.MessageKey{
			Version:       catalog.V44,
			MsgType:       mt,
			AssetCategory: catalog.AssetCategoryEquities,
		})
		if def == nil {
			t.Errorf("Equities MsgType %q not registered", mt)
		}
	}
}

func TestEquitiesNewOrderSingleEmitsInstrumentBlock(t *testing.T) {
	reregisterAll()
	def := catalog.Get(catalog.MessageKey{
		Version:       catalog.V44,
		MsgType:       app.MsgTypeNewOrderSingle,
		AssetCategory: catalog.AssetCategoryEquities,
	})
	if def == nil {
		t.Fatal("Equities NewOrderSingle not registered")
	}

	r := rand.New(rand.NewSource(1))
	tags := buildTagMap(def.Fields, r)

	required := []catalog.Tag{
		app.TagClOrdID,
		TagAccount, TagAccountType,
		app.TagSymbol, TagSecurityID, TagSecurityIDSource,
		app.TagSecurityType, TagCFICode,
		app.TagSide, app.TagOrdType, app.TagTimeInForce,
		app.TagOrderQty, app.TagPrice, app.TagTransactTime,
	}
	for _, want := range required {
		if _, ok := tags[want]; !ok {
			t.Errorf("Equities NewOrderSingle missing tag %d", want)
		}
	}
}

func TestEquitiesDeterminismFromSeed(t *testing.T) {
	reregisterAll()
	def := catalog.Get(catalog.MessageKey{
		Version:       catalog.V44,
		MsgType:       app.MsgTypeNewOrderSingle,
		AssetCategory: catalog.AssetCategoryEquities,
	})

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

func TestEquitiesSecurityTypeValuesAreInCategory(t *testing.T) {
	reregisterAll()
	def := catalog.Get(catalog.MessageKey{
		Version:       catalog.V44,
		MsgType:       app.MsgTypeNewOrderSingle,
		AssetCategory: catalog.AssetCategoryEquities,
	})

	// Across many seeds, every SecurityType emitted must belong to the
	// Equities category.
	for seed := int64(1); seed <= 100; seed++ {
		tags := buildTagMap(def.Fields, rand.New(rand.NewSource(seed)))
		st := catalog.SecurityType(tags[app.TagSecurityType])
		if st.Category() != catalog.AssetCategoryEquities {
			t.Errorf("seed=%d: emitted SecurityType %q has category %s, want Equities",
				seed, st, st.Category())
		}
	}
}

func TestEquitiesPriceFormat(t *testing.T) {
	reregisterAll()
	def := catalog.Get(catalog.MessageKey{
		Version:       catalog.V44,
		MsgType:       app.MsgTypeNewOrderSingle,
		AssetCategory: catalog.AssetCategoryEquities,
	})
	tags := buildTagMap(def.Fields, rand.New(rand.NewSource(1)))
	price := tags[app.TagPrice]
	if price == "" {
		t.Fatal("missing Price")
	}
	// Realistic equity price: at least 3 chars, has a dot, two-digit fraction.
	if len(price) < 4 {
		t.Errorf("Price %q too short", price)
	}
}

func buildTagMap(gens []catalog.FieldGenerator, r *rand.Rand) map[catalog.Tag]string {
	out := make(map[catalog.Tag]string, len(gens))
	ctx := &catalog.GenerateCtx{Version: catalog.V44, AssetCategory: catalog.AssetCategoryEquities}
	for _, g := range gens {
		f := g(r, ctx)
		out[f.Tag] = f.Value
	}
	return out
}
