package corpbonds

import (
	"fmt"
	"math/rand"
	"slices"
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

func TestEveryCorpBondSecurityTypeHasInstruments(t *testing.T) {
	for _, st := range catalog.SecurityTypesByCategory(catalog.AssetCategoryCorpBonds) {
		if len(corpBondsByType[st]) == 0 {
			t.Errorf("CorpBonds SecurityType %q has no bonds", st)
		}
	}
}

func TestAllCorpBondsMessagesRegistered(t *testing.T) {
	reregisterAll()
	for _, mt := range []string{
		app.MsgTypeNewOrderSingle, app.MsgTypeExecutionReport,
		app.MsgTypeOrderCancelRequest, app.MsgTypeOrderCancelReplaceRequest,
		app.MsgTypeOrderStatusRequest,
	} {
		if catalog.Get(catalog.MessageKey{Version: catalog.V44, MsgType: mt, AssetCategory: catalog.AssetCategoryCorpBonds}) == nil {
			t.Errorf("CorpBonds %s not registered", mt)
		}
	}
}

func TestCorpBondsInstrumentBlock(t *testing.T) {
	reregisterAll()
	def := catalog.Get(catalog.MessageKey{Version: catalog.V44, MsgType: app.MsgTypeNewOrderSingle, AssetCategory: catalog.AssetCategoryCorpBonds})
	tags := buildTagMap(def.Fields, rand.New(rand.NewSource(1)))
	required := []catalog.Tag{
		app.TagSymbol, app.TagSecurityType, TagSecurityID, TagSecurityIDSrc,
		TagCouponRate, TagMaturityDate, TagIssueDate, TagCountryOfIssue,
		TagCreditRating, TagCFICode, TagPriceType,
	}
	for _, want := range required {
		if _, ok := tags[want]; !ok {
			t.Errorf("CorpBonds NewOrderSingle missing tag %d", want)
		}
	}
}

func TestCorpBondsSecurityTypeStaysInCategory(t *testing.T) {
	reregisterAll()
	def := catalog.Get(catalog.MessageKey{Version: catalog.V44, MsgType: app.MsgTypeNewOrderSingle, AssetCategory: catalog.AssetCategoryCorpBonds})
	for seed := int64(1); seed <= 100; seed++ {
		tags := buildTagMap(def.Fields, rand.New(rand.NewSource(seed)))
		st := catalog.SecurityType(tags[app.TagSecurityType])
		if st.Category() != catalog.AssetCategoryCorpBonds {
			t.Errorf("seed=%d: SecurityType %q has category %s, want CorpBonds", seed, st, st.Category())
		}
	}
}

func TestCorpBondsDeterminismFromSeed(t *testing.T) {
	reregisterAll()
	def := catalog.Get(catalog.MessageKey{Version: catalog.V44, MsgType: app.MsgTypeNewOrderSingle, AssetCategory: catalog.AssetCategoryCorpBonds})
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
	ctx := &catalog.GenerateCtx{Version: catalog.V44, AssetCategory: catalog.AssetCategoryCorpBonds}
	for _, g := range gens {
		f := g(r, ctx)
		out[f.Tag] = f.Value
	}
	return out
}

// TestCorpBondsInstrumentBlockCoherent pins that every instrument field of a
// message describes one bond of the message's SecurityType.
func TestCorpBondsInstrumentBlockCoherent(t *testing.T) {
	reregisterAll()
	def := catalog.Get(catalog.MessageKey{Version: catalog.V44, MsgType: app.MsgTypeNewOrderSingle, AssetCategory: catalog.AssetCategoryCorpBonds})
	for seed := int64(0); seed < 300; seed++ {
		tags := buildTagMap(def.Fields, rand.New(rand.NewSource(seed)))
		st := catalog.SecurityType(tags[app.TagSecurityType])
		rows := corpBondsByType[st]
		if len(rows) == 0 {
			rows = []bond{pickBond(nil, st)} // fallback row; draws nothing from r
		}
		i := slices.IndexFunc(rows, func(b bond) bool { return b.Cusip == tags[TagSecurityID] })
		if i < 0 {
			t.Fatalf("seed %d: CUSIP %q is not a %s", seed, tags[TagSecurityID], st)
		}
		b := rows[i]
		if want := strings.ReplaceAll(b.Symbol, "{YY}", tags[TagMaturityDate][2:4]); tags[app.TagSymbol] != want {
			t.Fatalf("seed %d: symbol %q, want %q", seed, tags[app.TagSymbol], want)
		}
		if tags[TagCouponRate] != b.Coupon ||
			tags[TagCountryOfIssue] != b.Country || tags[TagCreditRating] != b.Rating || tags[TagCFICode] != b.CFICode {
			t.Fatalf("seed %d: %s message mixes bonds: %v", seed, st, tags)
		}
	}
}

// TestCorpBondDatesMatchTheBond pins a corporate or municipal bond message's
// dates to its bond: issue and maturity span the bond's term in years, the
// bond is outstanding on the trade date, and the symbol's two-digit year is
// its maturity year.
func TestCorpBondDatesMatchTheBond(t *testing.T) {
	reregisterAll()
	def := catalog.Get(catalog.MessageKey{Version: catalog.V44, MsgType: app.MsgTypeNewOrderSingle, AssetCategory: catalog.AssetCategoryCorpBonds})
	trade := time.Date(2031, 6, 18, 14, 0, 0, 0, time.UTC)
	for seed := int64(0); seed < 300; seed++ {
		tags := map[catalog.Tag]string{}
		ctx := &catalog.GenerateCtx{Version: catalog.V44, AssetCategory: catalog.AssetCategoryCorpBonds, Now: trade}
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
		rows := corpBondsByType[st]
		i := slices.IndexFunc(rows, func(b bond) bool { return b.Cusip == tags[TagSecurityID] })
		require.GreaterOrEqual(t, i, 0, "seed %d", seed)
		require.Equal(t, iss.AddDate(rows[i].Maturity, 0, 0), mat, "seed %d: term", seed)
		require.True(t, strings.HasSuffix(tags[app.TagSymbol], fmt.Sprintf("-%02d", mat.Year()%100)), "seed %d: symbol %q, maturity %d", seed, tags[app.TagSymbol], mat.Year())
	}
}
