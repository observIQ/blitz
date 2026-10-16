package otcderivs

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

func TestEveryOTCSecurityTypeHasSwaps(t *testing.T) {
	for _, st := range catalog.SecurityTypesByCategory(catalog.AssetCategoryOTCDerivs) {
		if len(swapsByType[st]) == 0 {
			t.Errorf("OTC %q has no swaps", st)
		}
	}
}

func TestAllOTCMessagesRegistered(t *testing.T) {
	reregisterAll()
	for _, mt := range []string{
		app.MsgTypeNewOrderSingle, app.MsgTypeExecutionReport,
		app.MsgTypeOrderCancelRequest, app.MsgTypeOrderCancelReplaceRequest,
		app.MsgTypeOrderStatusRequest,
	} {
		if catalog.Get(catalog.MessageKey{Version: catalog.V44, MsgType: mt, AssetCategory: catalog.AssetCategoryOTCDerivs}) == nil {
			t.Errorf("OTC %s not registered", mt)
		}
	}
}

func TestOTCInstrumentBlock(t *testing.T) {
	reregisterAll()
	def := catalog.Get(catalog.MessageKey{Version: catalog.V44, MsgType: app.MsgTypeNewOrderSingle, AssetCategory: catalog.AssetCategoryOTCDerivs})
	// Always-present instrument-block tags. UnderlyingSecurityID and its
	// source appear only for an underlier with an identifier (tested in
	// tagconformance_test.go).
	required := []catalog.Tag{
		app.TagSymbol, app.TagSecurityType, TagSecurityID, TagSecurityIDSrc,
		TagCouponRate, TagMaturityDate, TagUnderlyingSymbol,
		TagCFICode,
	}
	tags := buildTagMap(def.Fields, rand.New(rand.NewSource(1)))
	for _, want := range required {
		if _, ok := tags[want]; !ok {
			t.Errorf("OTC NewOrderSingle missing tag %d", want)
		}
	}
}

// TestOTCInstrumentBlockCoherent asserts that every instrument-block
// field in a single message agrees on the same picked (SecurityType,
// swap) — Symbol/SecurityID/Underlying/CFICode all come
// from one row of swapsByType, not independent picks per field.
func TestOTCInstrumentBlockCoherent(t *testing.T) {
	reregisterAll()
	def := catalog.Get(catalog.MessageKey{Version: catalog.V44, MsgType: app.MsgTypeNewOrderSingle, AssetCategory: catalog.AssetCategoryOTCDerivs})
	for seed := int64(1); seed <= 50; seed++ {
		tags := buildTagMap(def.Fields, rand.New(rand.NewSource(seed)))
		st := catalog.SecurityType(tags[app.TagSecurityType])
		rows := swapsByType[st]
		// Build a lookup from Symbol to the full row so we can verify
		// every emitted instrument-block tag points at the same row.
		var match *swap
		for i := range rows {
			if rows[i].Symbol == tags[app.TagSymbol] {
				match = &rows[i]
				break
			}
		}
		if match == nil {
			t.Errorf("seed=%d: Symbol %q does not belong to SecurityType %q", seed, tags[app.TagSymbol], st)
			continue
		}
		if got := tags[TagSecurityID]; got != match.ID {
			t.Errorf("seed=%d st=%s: SecurityID=%q desyncs from Symbol row (want %q)", seed, st, got, match.ID)
		}
		if got := tags[TagUnderlyingSymbol]; got != match.Underlying {
			t.Errorf("seed=%d st=%s: UnderlyingSymbol=%q desyncs from Symbol row (want %q)", seed, st, got, match.Underlying)
		}
		if got := tags[TagCFICode]; got != match.CFICode {
			t.Errorf("seed=%d st=%s: CFICode=%q desyncs from Symbol row (want %q)", seed, st, got, match.CFICode)
		}
	}
}

// TestOTCPriceFormatByType: CDS prices are integer bps strings; other
// swap types are N.NNNN decimal NPV/spread.
func TestOTCPriceFormatByType(t *testing.T) {
	reregisterAll()
	def := catalog.Get(catalog.MessageKey{Version: catalog.V44, MsgType: app.MsgTypeNewOrderSingle, AssetCategory: catalog.AssetCategoryOTCDerivs})
	sawCDS, sawNonCDS := false, false
	for seed := int64(1); seed <= 200; seed++ {
		tags := buildTagMap(def.Fields, rand.New(rand.NewSource(seed)))
		st := catalog.SecurityType(tags[app.TagSecurityType])
		px := tags[app.TagPrice]
		if st == catalog.SecCDS {
			sawCDS = true
			if strings.ContainsRune(px, '.') {
				t.Errorf("seed=%d CDS price %q must be integer bps, not decimal", seed, px)
			}
		} else {
			sawNonCDS = true
			dot := strings.IndexByte(px, '.')
			if dot < 0 || len(px)-dot != 5 {
				t.Errorf("seed=%d %s price %q must be N.NNNN format", seed, st, px)
			}
		}
	}
	if !sawCDS || !sawNonCDS {
		t.Fatalf("test did not exercise both CDS and non-CDS code paths (cds=%v, nonCds=%v)", sawCDS, sawNonCDS)
	}
}

func TestOTCSecurityTypeStaysInCategory(t *testing.T) {
	reregisterAll()
	def := catalog.Get(catalog.MessageKey{Version: catalog.V44, MsgType: app.MsgTypeNewOrderSingle, AssetCategory: catalog.AssetCategoryOTCDerivs})
	for seed := int64(1); seed <= 100; seed++ {
		tags := buildTagMap(def.Fields, rand.New(rand.NewSource(seed)))
		st := catalog.SecurityType(tags[app.TagSecurityType])
		if st.Category() != catalog.AssetCategoryOTCDerivs {
			t.Errorf("seed=%d: SecurityType %q has category %s, want OTCDerivs", seed, st, st.Category())
		}
	}
}

func TestOTCDeterminismFromSeed(t *testing.T) {
	reregisterAll()
	def := catalog.Get(catalog.MessageKey{Version: catalog.V44, MsgType: app.MsgTypeNewOrderSingle, AssetCategory: catalog.AssetCategoryOTCDerivs})
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
	ctx := &catalog.GenerateCtx{Version: catalog.V44, AssetCategory: catalog.AssetCategoryOTCDerivs}
	for _, g := range gens {
		f := g(r, ctx)
		out[f.Tag] = f.Value
	}
	return out
}
