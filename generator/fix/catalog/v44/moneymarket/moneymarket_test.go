package moneymarket

import (
	"math/rand"
	"regexp"
	"strconv"
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

func TestEveryMMSecurityTypeHasInstruments(t *testing.T) {
	for _, st := range catalog.SecurityTypesByCategory(catalog.AssetCategoryMoneyMarket) {
		if len(instrumentsByType[st]) == 0 {
			t.Errorf("MoneyMarket %q has no instruments", st)
		}
	}
}

func TestAllMMMessagesRegistered(t *testing.T) {
	reregisterAll()
	for _, mt := range []string{
		app.MsgTypeNewOrderSingle, app.MsgTypeExecutionReport,
		app.MsgTypeOrderCancelRequest, app.MsgTypeOrderCancelReplaceRequest,
		app.MsgTypeOrderStatusRequest,
	} {
		if catalog.Get(catalog.MessageKey{Version: catalog.V44, MsgType: mt, AssetCategory: catalog.AssetCategoryMoneyMarket}) == nil {
			t.Errorf("MoneyMarket %s not registered", mt)
		}
	}
}

func TestMMInstrumentBlock(t *testing.T) {
	reregisterAll()
	def := catalog.Get(catalog.MessageKey{Version: catalog.V44, MsgType: app.MsgTypeNewOrderSingle, AssetCategory: catalog.AssetCategoryMoneyMarket})
	tags := buildTagMap(def.Fields, rand.New(rand.NewSource(1)))
	required := []catalog.Tag{
		app.TagSymbol, app.TagSecurityType, TagSecurityID, TagSecurityIDSrc,
		TagIssuer, TagCouponRate, TagMaturityDate, TagIssueDate,
		TagCountryOfIssue, TagCreditRating, TagYieldType, TagCFICode, TagPriceType,
	}
	for _, want := range required {
		if _, ok := tags[want]; !ok {
			t.Errorf("MoneyMarket NewOrderSingle missing tag %d", want)
		}
	}
}

func TestMMSecurityTypeStaysInCategory(t *testing.T) {
	reregisterAll()
	def := catalog.Get(catalog.MessageKey{Version: catalog.V44, MsgType: app.MsgTypeNewOrderSingle, AssetCategory: catalog.AssetCategoryMoneyMarket})
	for seed := int64(1); seed <= 100; seed++ {
		tags := buildTagMap(def.Fields, rand.New(rand.NewSource(seed)))
		st := catalog.SecurityType(tags[app.TagSecurityType])
		if st.Category() != catalog.AssetCategoryMoneyMarket {
			t.Errorf("seed=%d: SecurityType %q has category %s, want MoneyMarket", seed, st, st.Category())
		}
	}
}

func TestMMYieldTypeIsValid(t *testing.T) {
	reregisterAll()
	def := catalog.Get(catalog.MessageKey{Version: catalog.V44, MsgType: app.MsgTypeNewOrderSingle, AssetCategory: catalog.AssetCategoryMoneyMarket})
	for seed := int64(1); seed <= 50; seed++ {
		tags := buildTagMap(def.Fields, rand.New(rand.NewSource(seed)))
		yt := tags[TagYieldType]
		if yt != YieldTypeDiscount && yt != YieldTypeInterestBearing {
			t.Errorf("seed=%d: YieldType %q invalid", seed, yt)
		}
	}
}

// TestMMInstrumentBlockCoherent confirms that within one message, the
// Symbol/SecurityID/Issuer/YieldType/PriceType/CreditRating/CFICode
// all reference the same instrument row — keeping CD↔interest-bearing
// and CP/BA↔discount pairings intact rather than mixing.
func TestMMInstrumentBlockCoherent(t *testing.T) {
	reregisterAll()
	def := catalog.Get(catalog.MessageKey{Version: catalog.V44, MsgType: app.MsgTypeNewOrderSingle, AssetCategory: catalog.AssetCategoryMoneyMarket})
	for seed := int64(1); seed <= 50; seed++ {
		tags := buildTagMap(def.Fields, rand.New(rand.NewSource(seed)))
		st := catalog.SecurityType(tags[app.TagSecurityType])
		rows := instrumentsByType[st]
		var match *instrument
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
		if got := tags[TagSecurityID]; got != match.Cusip {
			t.Errorf("seed=%d st=%s: CUSIP=%q desyncs (want %q)", seed, st, got, match.Cusip)
		}
		if got := tags[TagIssuer]; got != match.Issuer {
			t.Errorf("seed=%d st=%s: Issuer=%q desyncs (want %q)", seed, st, got, match.Issuer)
		}
		if got := tags[TagYieldType]; got != match.Yield {
			t.Errorf("seed=%d st=%s: YieldType=%q desyncs (want %q)", seed, st, got, match.Yield)
		}
		if got := tags[TagPriceType]; got != match.Price {
			t.Errorf("seed=%d st=%s: PriceType=%q desyncs (want %q)", seed, st, got, match.Price)
		}
		if got := tags[TagCreditRating]; got != match.Rating {
			t.Errorf("seed=%d st=%s: Rating=%q desyncs (want %q)", seed, st, got, match.Rating)
		}
		if got := tags[TagCFICode]; got != match.CFICode {
			t.Errorf("seed=%d st=%s: CFICode=%q desyncs (want %q)", seed, st, got, match.CFICode)
		}
	}
}

// TestMMMaturityWithinOneYear pins that every maturity is within
// (issue, issue+1y] of IssueDate 2026-01-01, and that each tenor in the
// table shows up.
func TestMMMaturityWithinOneYear(t *testing.T) {
	reregisterAll()
	def := catalog.Get(catalog.MessageKey{Version: catalog.V44, MsgType: app.MsgTypeNewOrderSingle, AssetCategory: catalog.AssetCategoryMoneyMarket})
	const issue = "20260101"
	maxMatur := "20270101" // strict upper bound (issue + 1y)
	seen := map[string]struct{}{}
	for seed := int64(1); seed <= 200; seed++ {
		tags := buildTagMap(def.Fields, rand.New(rand.NewSource(seed)))
		m := tags[TagMaturityDate]
		if m <= issue || m > maxMatur {
			t.Errorf("seed=%d: MaturityDate %q outside (issue, issue+1y] = (%s, %s]", seed, m, issue, maxMatur)
		}
		seen[m] = struct{}{}
	}
	// One date per tenor in the table (30, 60, 90, 180 days).
	if len(seen) < 4 {
		t.Errorf("MaturityDate took %d distinct dates across 200 seeds, want one per tenor (4)", len(seen))
	}
}

func TestMMDeterminismFromSeed(t *testing.T) {
	reregisterAll()
	def := catalog.Get(catalog.MessageKey{Version: catalog.V44, MsgType: app.MsgTypeNewOrderSingle, AssetCategory: catalog.AssetCategoryMoneyMarket})
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
	// Trade (and issue) on 2026-01-01 so the fixed expected dates below hold.
	ctx := &catalog.GenerateCtx{Version: catalog.V44, AssetCategory: catalog.AssetCategoryMoneyMarket, Now: time.Date(2026, 1, 1, 14, 0, 0, 0, time.UTC)}
	for _, g := range gens {
		f := g(r, ctx)
		out[f.Tag] = f.Value
	}
	return out
}

// TestMMIssuedOnABusinessDay pins that paper traded on a weekend (a live run
// started on a Saturday) is issued the next business day, as maturity and
// settlement dates roll, and matures within a year of that issue date.
func TestMMIssuedOnABusinessDay(t *testing.T) {
	ctx := &catalog.GenerateCtx{Now: time.Date(2026, 10, 17, 14, 0, 0, 0, time.UTC)} // Saturday
	require.Equal(t, "20261019", issueDateField()(rand.New(rand.NewSource(1)), ctx).Value)
	for seed := int64(0); seed < 200; seed++ {
		m := maturityDateField()(rand.New(rand.NewSource(seed)), ctx).Value
		require.True(t, m >= "20261026" && m <= "20271019", "seed %d: maturity %s", seed, m)
	}
}

// TestMMMaturesAtItsSymbolsTenor pins that the tenor a symbol names
// (GE-CP-30D) is the one MaturityDate carries: issue date plus that many
// days, rolled to a business day.
func TestMMMaturesAtItsSymbolsTenor(t *testing.T) {
	tenor := regexp.MustCompile(`-(\d+)D$`)
	for seed := int64(0); seed < 200; seed++ {
		r := rand.New(rand.NewSource(seed))
		ctx := &catalog.GenerateCtx{Now: time.Date(2026, 10, 14, 14, 0, 0, 0, time.UTC)}
		sym := symbolField()(r, ctx).Value
		m := tenor.FindStringSubmatch(sym)
		require.NotNil(t, m, sym)
		days, err := strconv.Atoi(m[1])
		require.NoError(t, err)
		want := catalog.NextBusinessDay(issueDate(ctx).AddDate(0, 0, days)).Format("20060102")
		require.Equal(t, want, maturityDateField()(r, ctx).Value, sym)
	}
}
