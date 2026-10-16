package structured

import (
	"math/rand"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/observiq/blitz/generator/fix/catalog"
	"github.com/observiq/blitz/generator/fix/catalog/v44/app"
)

func reregisterAll() {
	app.Reregister()
	registerAll()
}

func TestEveryStructuredSecurityTypeHasInstruments(t *testing.T) {
	for _, st := range catalog.SecurityTypesByCategory(catalog.AssetCategoryStructured) {
		if len(instrumentsByType[st]) == 0 {
			t.Errorf("Structured SecurityType %q has no instruments", st)
		}
	}
}

func TestAllStructuredMessagesRegistered(t *testing.T) {
	reregisterAll()
	for _, mt := range []string{
		app.MsgTypeNewOrderSingle, app.MsgTypeExecutionReport,
		app.MsgTypeOrderCancelRequest, app.MsgTypeOrderCancelReplaceRequest,
		app.MsgTypeOrderStatusRequest,
	} {
		if catalog.Get(catalog.MessageKey{Version: catalog.V44, MsgType: mt, AssetCategory: catalog.AssetCategoryStructured}) == nil {
			t.Errorf("Structured %s not registered", mt)
		}
	}
}

func TestStructuredInstrumentBlock(t *testing.T) {
	reregisterAll()
	def := catalog.Get(catalog.MessageKey{Version: catalog.V44, MsgType: app.MsgTypeNewOrderSingle, AssetCategory: catalog.AssetCategoryStructured})
	tags := buildTagMap(def.Fields, rand.New(rand.NewSource(1)))
	required := []catalog.Tag{
		app.TagSymbol, app.TagSecurityType, TagSecurityID, TagSecurityIDSrc,
		TagCouponRate, TagMaturityDate, TagIssueDate, TagFactor,
		TagPool, TagTranche, TagCountryOfIssue, TagCreditRating, TagCFICode,
	}
	for _, want := range required {
		if _, ok := tags[want]; !ok {
			t.Errorf("Structured NewOrderSingle missing tag %d", want)
		}
	}
}

func TestStructuredFactorRange(t *testing.T) {
	reregisterAll()
	def := catalog.Get(catalog.MessageKey{Version: catalog.V44, MsgType: app.MsgTypeNewOrderSingle, AssetCategory: catalog.AssetCategoryStructured})
	for seed := int64(1); seed <= 50; seed++ {
		tags := buildTagMap(def.Fields, rand.New(rand.NewSource(seed)))
		f := tags[TagFactor]
		if len(f) != 6 || f[:2] != "0." {
			t.Errorf("seed=%d: Factor %q must be 0.NNNN form (six chars)", seed, f)
		}
	}
}

func TestStructuredSecurityTypeStaysInCategory(t *testing.T) {
	reregisterAll()
	def := catalog.Get(catalog.MessageKey{Version: catalog.V44, MsgType: app.MsgTypeNewOrderSingle, AssetCategory: catalog.AssetCategoryStructured})
	for seed := int64(1); seed <= 100; seed++ {
		tags := buildTagMap(def.Fields, rand.New(rand.NewSource(seed)))
		st := catalog.SecurityType(tags[app.TagSecurityType])
		if st.Category() != catalog.AssetCategoryStructured {
			t.Errorf("seed=%d: SecurityType %q has category %s, want Structured", seed, st, st.Category())
		}
	}
}

func TestStructuredDeterminismFromSeed(t *testing.T) {
	reregisterAll()
	def := catalog.Get(catalog.MessageKey{Version: catalog.V44, MsgType: app.MsgTypeNewOrderSingle, AssetCategory: catalog.AssetCategoryStructured})
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
	ctx := &catalog.GenerateCtx{Version: catalog.V44, AssetCategory: catalog.AssetCategoryStructured}
	for _, g := range gens {
		f := g(r, ctx)
		out[f.Tag] = f.Value
	}
	return out
}

// TestStructuredInstrumentBlockCoherent pins that every instrument field of a
// message describes one instrument of the message's SecurityType.
func TestStructuredInstrumentBlockCoherent(t *testing.T) {
	reregisterAll()
	def := catalog.Get(catalog.MessageKey{Version: catalog.V44, MsgType: app.MsgTypeNewOrderSingle, AssetCategory: catalog.AssetCategoryStructured})
	for seed := int64(0); seed < 300; seed++ {
		tags := buildTagMap(def.Fields, rand.New(rand.NewSource(seed)))
		st := catalog.SecurityType(tags[app.TagSecurityType])
		i := slices.IndexFunc(instrumentsByType[st], func(in instrument) bool { return in.Cusip == tags[TagSecurityID] })
		if i < 0 {
			t.Fatalf("seed %d: CUSIP %q is not a %s", seed, tags[TagSecurityID], st)
		}
		in := instrumentsByType[st][i]
		year := tags[TagIssueDate][:4]
		in.Symbol, in.Pool = strings.ReplaceAll(in.Symbol, "{Y}", year), strings.ReplaceAll(in.Pool, "{Y}", year)
		if tags[app.TagSymbol] != in.Symbol {
			t.Fatalf("seed %d: symbol %q, want %q", seed, tags[app.TagSymbol], in.Symbol)
		}
		tranche := in.Tranche
		if tranche == "" {
			tranche = "A1"
		}
		if tags[TagCouponRate] != in.Coupon || tags[TagPool] != in.Pool ||
			tags[TagTranche] != tranche || tags[TagCreditRating] != in.Rating || tags[TagCFICode] != in.CFICode {
			t.Fatalf("seed %d: %s message mixes instruments: %v", seed, in.Symbol, tags)
		}
	}
}

// TestStructuredDatesMatchTheInstrument pins a structured-product message's
// dates to its deal: issue and legal final maturity span the deal's term in
// years, the deal is outstanding on the trade date, and a vintage year in the
// symbol or pool is its issue year.
func TestStructuredDatesMatchTheInstrument(t *testing.T) {
	reregisterAll()
	def := catalog.Get(catalog.MessageKey{Version: catalog.V44, MsgType: app.MsgTypeNewOrderSingle, AssetCategory: catalog.AssetCategoryStructured})
	trade := time.Date(2031, 6, 18, 14, 0, 0, 0, time.UTC)
	for seed := int64(0); seed < 300; seed++ {
		tags := map[catalog.Tag]string{}
		ctx := &catalog.GenerateCtx{Version: catalog.V44, AssetCategory: catalog.AssetCategoryStructured, Now: trade}
		r := rand.New(rand.NewSource(seed))
		for _, g := range def.Fields {
			f := g(r, ctx)
			tags[f.Tag] = f.Value
		}
		iss, err := time.Parse("20060102", tags[TagIssueDate])
		require.NoError(t, err)
		mat, err := time.Parse("20060102", tags[TagMaturityDate])
		require.NoError(t, err)
		require.True(t, iss.Before(trade) && mat.After(trade), "seed %d: not outstanding", seed)
		st := catalog.SecurityType(tags[app.TagSecurityType])
		rows := instrumentsByType[st]
		i := slices.IndexFunc(rows, func(in instrument) bool { return in.Cusip == tags[TagSecurityID] })
		require.GreaterOrEqual(t, i, 0, "seed %d", seed)
		require.Equal(t, iss.AddDate(rows[i].TermYears, 0, 0), mat, "seed %d: term", seed)
		year := strconv.Itoa(iss.Year())
		for _, tag := range []catalog.Tag{app.TagSymbol, TagPool} {
			want := strings.ReplaceAll(map[catalog.Tag]string{app.TagSymbol: rows[i].Symbol, TagPool: rows[i].Pool}[tag], "{Y}", year)
			require.Equal(t, want, tags[tag], "seed %d", seed)
		}
	}
}
