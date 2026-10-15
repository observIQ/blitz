package govbonds

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

func TestEveryGovBondSecurityTypeHasInstruments(t *testing.T) {
	for _, st := range catalog.SecurityTypesByCategory(catalog.AssetCategoryGovBonds) {
		if len(govBondsByType[st]) == 0 {
			t.Errorf("GovBonds SecurityType %q has no bonds", st)
		}
		for i, b := range govBondsByType[st] {
			if b.Symbol == "" {
				t.Errorf("%s row %d: empty Symbol", st, i)
			}
			if len(b.Prefix) != 6 {
				t.Errorf("%s row %d (%s): CUSIP prefix %q must be 6 chars", st, i, b.Symbol, b.Prefix)
			}
			if b.Real != nil && !cusipCheckDigitOK(b.Real.Cusip) {
				t.Errorf("%s row %d (%s): real CUSIP %q fails its check digit", st, i, b.Symbol, b.Real.Cusip)
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

// TestGovBondsInstrumentBlockCoherent pins that every instrument field of a
// message describes one bond of the message's SecurityType.
func TestGovBondsInstrumentBlockCoherent(t *testing.T) {
	reregisterAll()
	def := catalog.Get(catalog.MessageKey{Version: catalog.V44, MsgType: app.MsgTypeNewOrderSingle, AssetCategory: catalog.AssetCategoryGovBonds})
	for seed := int64(0); seed < 300; seed++ {
		tags := buildTagMap(def.Fields, rand.New(rand.NewSource(seed)))
		st := catalog.SecurityType(tags[app.TagSecurityType])
		i := slices.IndexFunc(govBondsByType[st], func(b bond) bool {
			return strings.ReplaceAll(b.Symbol, "{Y}", tags[TagMaturityDate][:4]) == tags[app.TagSymbol] &&
				(b.Real != nil && b.Real.Cusip == tags[TagSecurityID] || strings.HasPrefix(tags[TagSecurityID], b.Prefix))
		})
		if i < 0 {
			t.Fatalf("seed %d: %s %s is not a %s row", seed, tags[app.TagSymbol], tags[TagSecurityID], st)
		}
		b := govBondsByType[st][i]
		coupon := b.CouponPct
		if b.Real != nil && b.Real.Cusip == tags[TagSecurityID] {
			coupon = b.Real.CouponPct
		}
		if coupon == "" {
			coupon = "0.000"
		}
		if tags[TagCouponRate] != coupon || tags[TagYieldType] != b.YieldType || tags[TagCFICode] != b.CFICode {
			t.Fatalf("seed %d: %s message mixes bonds: %v", seed, b.Symbol, tags)
		}
		if (st == catalog.SecTIPS) != (tags[TagFactor] != "1.000") {
			t.Fatalf("seed %d: %s factor %s", seed, st, tags[TagFactor])
		}
	}
}

// TestGovBondDatesMatchTheBond pins a gov-bond message's dates to its bond:
// a bill matures within its week count on its auction weekday, a note or bond
// settles on or up to 17 days after its dated date one term before maturity
// (a weekend roll, a 20-year bond's or TIPS' month-end), and a STRIP's symbol
// names its maturity year; every bond is outstanding on the trade date.
func TestGovBondDatesMatchTheBond(t *testing.T) {
	reregisterAll()
	def := catalog.Get(catalog.MessageKey{Version: catalog.V44, MsgType: app.MsgTypeNewOrderSingle, AssetCategory: catalog.AssetCategoryGovBonds})
	trade := time.Date(2031, 6, 18, 14, 0, 0, 0, time.UTC)
	for seed := int64(0); seed < 300; seed++ {
		tags := map[catalog.Tag]string{}
		ctx := &catalog.GenerateCtx{Version: catalog.V44, AssetCategory: catalog.AssetCategoryGovBonds, Now: trade}
		r := rand.New(rand.NewSource(seed))
		for _, g := range def.Fields {
			f := g(r, ctx)
			tags[f.Tag] = f.Value
		}
		iss, err := time.Parse("20060102", tags[TagIssueDate])
		require.NoError(t, err)
		mat, err := time.Parse("20060102", tags[TagMaturityDate])
		require.NoError(t, err)
		require.True(t, iss.Before(trade) && mat.After(trade), "seed %d: %s %s..%s not outstanding", seed, tags[app.TagSymbol], iss.Format("20060102"), mat.Format("20060102"))
		sym := tags[app.TagSymbol]
		switch {
		case strings.HasPrefix(sym, "T-BILL-"):
			weeks, err := strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(sym, "T-BILL-"), "W"))
			require.NoError(t, err)
			require.Equal(t, iss.AddDate(0, 0, 7*weeks), mat, "seed %d: %s", seed, sym)
		case strings.HasPrefix(sym, "STRIP-INT-"):
			require.Equal(t, "STRIP-INT-"+strconv.Itoa(mat.Year()), sym, "seed %d", seed)
		case realIssues[sym][0] == tags[TagSecurityID]:
			// A real issue's dates are its own (TestOutstandingRealIssuesCarryTheirRealData).
		default:
			years, err := strconv.Atoi(sym[strings.LastIndex(sym, "-")+1 : len(sym)-1])
			require.NoError(t, err, sym)
			lag := iss.Sub(mat.AddDate(-years, 0, 0))
			require.True(t, lag >= 0 && lag <= 17*24*time.Hour, "seed %d: %s issued %s for %s", seed, sym, iss.Format("20060102"), mat.Format("20060102"))
		}
	}
}
