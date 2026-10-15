// Package govbonds registers FIX 4.4 MessageDefinitions for the
// Government fixed-income asset category. SecurityType values:
// TBILL (≤1y discount yield), TNOTE (2-10y semi-annual coupon),
// TBOND (20-30y semi-annual coupon), TIPS (CPI-indexed coupon),
// TINT (interest strip from coupon bond).
//
// All randomness sources from the supplied *rand.Rand.
package govbonds

import (
	"fmt"
	"math/rand"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/observiq/blitz/generator/fix/catalog"
	"github.com/observiq/blitz/generator/fix/catalog/v44/app"
)

// Fixed-income tags.
const (
	TagCouponRate     catalog.Tag = 223
	TagMaturityDate   catalog.Tag = 541
	TagIssueDate      catalog.Tag = 225
	TagFactor         catalog.Tag = 228 // For TIPS CPI factor; 1.0 for non-indexed.
	TagCountryOfIssue catalog.Tag = 470
	TagCFICode        catalog.Tag = 461
	TagSecurityID     catalog.Tag = 48
	TagSecurityIDSrc  catalog.Tag = 22
	TagYieldType      catalog.Tag = 235
)

// SecurityIDSource for US treasuries: CUSIP.
const SecurityIDSourceCUSIP = "1"

// YieldType (235) values relevant to govbonds.
const (
	YieldTypeBondEquivalent = "BOND" // bond-equivalent yield (notes/bonds)
	YieldTypeDiscount       = "DISC" // discount yield (bills)
	YieldTypeREALYIELD      = "REAL" // real yield (TIPS)
)

// bond is one row in the gov-bonds table. A bill's term is TermWeeks; every
// other row's is MaturityYrs. Symbol may hold {Y}, the maturity year (a STRIP
// is named for when it pays).
//
// Real is the row's latest original issue on TreasuryDirect: while it is
// outstanding on the trade date, a message carries its real CUSIP, coupon,
// and dates. Otherwise (and always for bills, which mature within weeks) the
// message describes a synthetic issue: dates on the term's real maturity
// calendar (Months, EOM) and a CUSIP with Prefix, a private-use issue number,
// and a valid check digit, so it names no real security. Real ages as issues
// mature; refresh it from TreasuryDirect's securities search to keep current
// issues real.
type bond struct {
	Symbol      string
	Prefix      string // CUSIP issuer prefix for a synthetic issue
	CouponPct   string // synthetic issue's coupon; empty for TBILL/TINT
	MaturityYrs int    // years to maturity at issue
	YieldType   string
	CFICode     string
	TermWeeks   int          // bills only
	Months      []time.Month // maturity months; nil means every month
	EOM         bool         // matures on the month's last day, else the 15th
	IssueAt     issueRule
	Real        *issue
}

// issueRule is when an issue settles relative to its dated date (maturity
// minus term), as TreasuryDirect's original issues show.
type issueRule int

const (
	issueOnDated     issueRule = iota // the dated date, or the next weekday
	issueMonthEnd                     // the dated month's last day, or the next weekday (20-year bonds)
	issueLastWeekday                  // the dated month's last weekday (TIPS)
)

// issue is one real Treasury security, from TreasuryDirect. A STRIP's issue
// date is its parent security's.
type issue struct {
	Cusip, CouponPct string
	Issue, Maturity  time.Time
}

func day(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, time.UTC) }

// quarterly is the Feb/May/Aug/Nov 15 cycle of 10-year notes, 20- and
// 30-year bonds, and their coupon STRIPS.
var quarterly = []time.Month{time.February, time.May, time.August, time.November}

// Maturity calendars and real issues: TreasuryDirect original issues,
// 2023-01-01 to 2026-10-06.
var govBondsByType = map[catalog.SecurityType][]bond{
	catalog.SecTBILL: {
		{Symbol: "T-BILL-4W", Prefix: "912797", YieldType: YieldTypeDiscount, CFICode: "DBTGFR", TermWeeks: 4},
		{Symbol: "T-BILL-13W", Prefix: "912797", YieldType: YieldTypeDiscount, CFICode: "DBTGFR", TermWeeks: 13},
		{Symbol: "T-BILL-26W", Prefix: "912797", YieldType: YieldTypeDiscount, CFICode: "DBTGFR", TermWeeks: 26},
		{Symbol: "T-BILL-52W", Prefix: "912797", YieldType: YieldTypeDiscount, CFICode: "DBTGFR", TermWeeks: 52},
	},
	catalog.SecTNOTE: {
		{Symbol: "T-NOTE-2Y", Prefix: "91282C", CouponPct: "4.750", MaturityYrs: 2, YieldType: YieldTypeBondEquivalent, CFICode: "DBTGFR", EOM: true,
			Real: &issue{"91282CRP8", "4.750", day(2026, 9, 30), day(2028, 9, 30)}},
		{Symbol: "T-NOTE-3Y", Prefix: "91282C", CouponPct: "4.375", MaturityYrs: 3, YieldType: YieldTypeBondEquivalent, CFICode: "DBTGFR",
			Real: &issue{"91282CRL7", "4.375", day(2026, 9, 15), day(2029, 9, 15)}},
		{Symbol: "T-NOTE-5Y", Prefix: "91282C", CouponPct: "5.000", MaturityYrs: 5, YieldType: YieldTypeBondEquivalent, CFICode: "DBTGFR", EOM: true,
			Real: &issue{"91282CRN3", "5.000", day(2026, 9, 30), day(2031, 9, 30)}},
		{Symbol: "T-NOTE-7Y", Prefix: "91282C", CouponPct: "5.000", MaturityYrs: 7, YieldType: YieldTypeBondEquivalent, CFICode: "DBTGFR", EOM: true,
			Real: &issue{"91282CRM5", "5.000", day(2026, 9, 30), day(2033, 9, 30)}},
		{Symbol: "T-NOTE-10Y", Prefix: "91282C", CouponPct: "4.625", MaturityYrs: 10, YieldType: YieldTypeBondEquivalent, CFICode: "DBTGFR", Months: quarterly,
			Real: &issue{"91282CRF0", "4.625", day(2026, 8, 17), day(2036, 8, 15)}},
	},
	catalog.SecTBOND: {
		{Symbol: "T-BOND-20Y", Prefix: "912810", CouponPct: "5.125", MaturityYrs: 20, YieldType: YieldTypeBondEquivalent, CFICode: "DBTGFR", Months: quarterly, IssueAt: issueMonthEnd,
			Real: &issue{"912810UX4", "5.125", day(2026, 8, 31), day(2046, 8, 15)}},
		{Symbol: "T-BOND-30Y", Prefix: "912810", CouponPct: "5.125", MaturityYrs: 30, YieldType: YieldTypeBondEquivalent, CFICode: "DBTGFR", Months: quarterly,
			Real: &issue{"912810UW6", "5.125", day(2026, 8, 17), day(2056, 8, 15)}},
	},
	catalog.SecTIPS: {
		{Symbol: "TIPS-5Y", Prefix: "91282C", CouponPct: "1.250", MaturityYrs: 5, YieldType: YieldTypeREALYIELD, CFICode: "DBTGXR", IssueAt: issueLastWeekday,
			Months: []time.Month{time.April, time.October},
			Real:   &issue{"91282CQP9", "1.250", day(2026, 4, 30), day(2031, 4, 15)}},
		{Symbol: "TIPS-10Y", Prefix: "91282C", CouponPct: "2.375", MaturityYrs: 10, YieldType: YieldTypeREALYIELD, CFICode: "DBTGXR", IssueAt: issueLastWeekday,
			Months: []time.Month{time.January, time.July},
			Real:   &issue{"91282CRE3", "2.375", day(2026, 7, 31), day(2036, 7, 15)}},
		{Symbol: "TIPS-30Y", Prefix: "912810", CouponPct: "2.375", MaturityYrs: 30, YieldType: YieldTypeREALYIELD, CFICode: "DBTGXR", IssueAt: issueLastWeekday,
			Months: []time.Month{time.February},
			Real:   &issue{"912810US5", "2.375", day(2026, 2, 27), day(2056, 2, 15)}},
	},
	catalog.SecTINT: {
		// Stripped interest payments (CFI DBTGZR) of a 7-year note and a
		// 30-year bond.
		{Symbol: "STRIP-INT-{Y}", Prefix: "912834", MaturityYrs: 7, YieldType: YieldTypeBondEquivalent, CFICode: "DBTGZR", EOM: true,
			Real: &issue{"912834K80", "", day(2026, 6, 30), day(2033, 6, 30)}},
		{Symbol: "STRIP-INT-{Y}", Prefix: "912834", MaturityYrs: 30, YieldType: YieldTypeBondEquivalent, CFICode: "DBTGZR", Months: quarterly,
			Real: &issue{"912834ZX9", "", day(2026, 5, 15), day(2056, 5, 15)}},
	},
}

func init()       { registerAll() }
func Reregister() { catalog.ResetForTest(); registerAll() }

func registerAll() {
	for _, mt := range []string{
		app.MsgTypeNewOrderSingle,
		app.MsgTypeExecutionReport,
		app.MsgTypeOrderCancelRequest,
		app.MsgTypeOrderCancelReplaceRequest,
		app.MsgTypeOrderStatusRequest,
	} {
		catalog.Register(catalog.MessageDefinition{
			Version:       catalog.V44,
			MsgType:       mt,
			AssetCategory: catalog.AssetCategoryGovBonds,
			Fields:        fieldsFor(mt),
		})
	}
}

func fieldsFor(msgType string) []catalog.FieldGenerator {
	switch msgType {
	case app.MsgTypeNewOrderSingle:
		return concat(
			[]catalog.FieldGenerator{clOrdID()},
			instrumentBlock(),
			[]catalog.FieldGenerator{side(), ordType(), tif(), orderQty(), price(), transactTime()},
		)
	case app.MsgTypeExecutionReport:
		return concat(
			[]catalog.FieldGenerator{orderID(), clOrdID(), execID(),
				catalog.LiteralField(app.TagExecType, app.ExecTypeNew),
				catalog.LiteralField(app.TagOrdStatus, app.OrdStatusNew)},
			instrumentBlock(),
			[]catalog.FieldGenerator{side(), orderQty(),
				catalog.IntField(app.TagCumQty, 0), leavesQty(),
				catalog.LiteralField(app.TagAvgPx, "0.000")},
		)
	case app.MsgTypeOrderCancelRequest:
		return concat(
			[]catalog.FieldGenerator{origClOrdID(), clOrdID()},
			instrumentBlock(),
			[]catalog.FieldGenerator{side(), transactTime()},
		)
	case app.MsgTypeOrderCancelReplaceRequest:
		return concat(
			[]catalog.FieldGenerator{origClOrdID(), clOrdID()},
			instrumentBlock(),
			[]catalog.FieldGenerator{side(), transactTime(), orderQty(), price(), ordType()},
		)
	case app.MsgTypeOrderStatusRequest:
		return concat(
			[]catalog.FieldGenerator{origClOrdID()},
			instrumentBlock(),
			[]catalog.FieldGenerator{side()},
		)
	}
	return nil
}

func concat(parts ...[]catalog.FieldGenerator) []catalog.FieldGenerator {
	var n int
	for _, p := range parts {
		n += len(p)
	}
	out := make([]catalog.FieldGenerator, 0, n)
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}

func instrumentBlock() []catalog.FieldGenerator {
	return []catalog.FieldGenerator{
		symbolField(),
		securityTypeField(),
		securityIDField(),
		catalog.LiteralField(TagSecurityIDSrc, SecurityIDSourceCUSIP),
		couponRateField(),
		maturityDateField(),
		issueDateField(),
		factorField(),
		catalog.LiteralField(TagCountryOfIssue, "US"),
		yieldTypeField(),
		cfiCodeField(),
	}
}

func pickSecurityType(r *rand.Rand) catalog.SecurityType {
	types := []catalog.SecurityType{
		catalog.SecTBILL, catalog.SecTNOTE, catalog.SecTBOND,
		catalog.SecTIPS, catalog.SecTINT,
	}
	return types[r.Intn(len(types))] // #nosec G404
}

func pickBond(r *rand.Rand, st catalog.SecurityType) bond {
	bs := govBondsByType[st]
	return bs[r.Intn(len(bs))] // #nosec G404 -- every SecurityType has rows (TestEveryGovBondSecurityTypeHasInstruments)
}

// pickKey is the GenerateCtx.Memo key for the message's picked
// (SecurityType, bond) pair, so every instrument field describes one row.
type pickKey struct{}

type pickedPair struct {
	SecType          catalog.SecurityType
	Bond             bond
	Cusip, CouponPct string
	Issue, Maturity  time.Time
}

// symbol is the bond's symbol with {Y} rendered as its maturity year.
func (p pickedPair) symbol() string {
	return strings.ReplaceAll(p.Bond.Symbol, "{Y}", strconv.Itoa(p.Maturity.Year()))
}

func pickedBond(r *rand.Rand, ctx *catalog.GenerateCtx) pickedPair {
	if ctx.Memo == nil {
		ctx.Memo = map[any]any{}
	}
	if v, ok := ctx.Memo[pickKey{}]; ok {
		return v.(pickedPair)
	}
	st := pickSecurityType(r)
	p := pickedPair{SecType: st, Bond: pickBond(r, st)}
	trade := ctx.TradeDate()
	switch b := p.Bond; {
	case b.Real != nil && b.Real.Issue.Before(trade) && b.Real.Maturity.After(trade):
		p.Cusip, p.CouponPct, p.Issue, p.Maturity = b.Real.Cusip, b.Real.CouponPct, b.Real.Issue, b.Real.Maturity
	case b.TermWeeks > 0:
		// A bill is issued and matures on its auction weekday.
		day := time.Thursday
		if b.TermWeeks == 4 {
			day = time.Tuesday
		}
		p.Issue, p.Maturity = catalog.BillDates(r, trade, b.TermWeeks, day)
		p.Cusip = syntheticCUSIP(r, b.Prefix)
	default:
		p.Issue, p.Maturity = calendarDates(r, trade, b)
		p.Cusip, p.CouponPct = syntheticCUSIP(r, b.Prefix), b.CouponPct
	}
	ctx.Memo[pickKey{}] = p
	return p
}

// calendarDates draws a synthetic issue of b outstanding on trade: it
// matures on b's maturity calendar within its term after trade and settled
// (issueDate) before trade.
func calendarDates(r *rand.Rand, trade time.Time, b bond) (issue, maturity time.Time) {
	months := 12 * max(b.MaturityYrs, 1)
	var dates []time.Time
	for k := 1; k <= months; k++ {
		m := time.Date(trade.Year(), trade.Month()+time.Month(k), 1, 0, 0, 0, 0, time.UTC)
		if b.Months != nil && !slices.Contains(b.Months, m.Month()) {
			continue
		}
		d := m.AddDate(0, 0, 14)
		if b.EOM {
			d = m.AddDate(0, 1, -1)
		}
		if d.After(trade) && issueDate(b, d).Before(trade) {
			dates = append(dates, d)
		}
	}
	maturity = dates[r.Intn(len(dates))] // #nosec G404 -- seeded *rand.Rand
	return issueDate(b, maturity), maturity
}

// issueDate is when an issue of b maturing on maturity settled, by b's
// IssueAt rule from its dated date one term earlier (the matching month-end
// for a month-end issue). Market holidays are not skipped.
func issueDate(b bond, maturity time.Time) time.Time {
	months := 12 * max(b.MaturityYrs, 1)
	dated := maturity.AddDate(0, -months, 0)
	monthEnd := time.Date(maturity.Year(), maturity.Month()-time.Month(months)+1, 0, 0, 0, 0, 0, time.UTC)
	if b.EOM {
		dated = monthEnd
	}
	switch b.IssueAt {
	case issueMonthEnd:
		return nextWeekday(monthEnd)
	case issueLastWeekday:
		for monthEnd.Weekday() == time.Saturday || monthEnd.Weekday() == time.Sunday {
			monthEnd = monthEnd.AddDate(0, 0, -1)
		}
		return monthEnd
	}
	return nextWeekday(dated)
}

// nextWeekday is d, or the Monday after when d falls on a weekend.
func nextWeekday(d time.Time) time.Time {
	for d.Weekday() == time.Saturday || d.Weekday() == time.Sunday {
		d = d.AddDate(0, 0, 1)
	}
	return d
}

// syntheticCUSIP is prefix plus a private-use issue number and the CUSIP
// check digit: well formed, but never a real security, since CUSIP reserves
// issue numbers 90-99 and 9A-9Y for private use.
func syntheticCUSIP(r *rand.Rand, prefix string) string {
	const second = "0123456789ABCDEFGHJKLMNPQRSTUVWXY"      // CUSIPs skip I and O
	c := prefix + "9" + string(second[r.Intn(len(second))]) // #nosec G404
	return c + string(catalog.CUSIPCheckDigit(c))
}

func symbolField() catalog.FieldGenerator {
	return func(r *rand.Rand, ctx *catalog.GenerateCtx) catalog.Field {
		return catalog.Field{Tag: app.TagSymbol, Value: pickedBond(r, ctx).symbol()}
	}
}

func securityTypeField() catalog.FieldGenerator {
	return func(r *rand.Rand, ctx *catalog.GenerateCtx) catalog.Field {
		return catalog.Field{Tag: app.TagSecurityType, Value: string(pickedBond(r, ctx).SecType)}
	}
}

func securityIDField() catalog.FieldGenerator {
	return func(r *rand.Rand, ctx *catalog.GenerateCtx) catalog.Field {
		return catalog.Field{Tag: TagSecurityID, Value: pickedBond(r, ctx).Cusip}
	}
}

func couponRateField() catalog.FieldGenerator {
	return func(r *rand.Rand, ctx *catalog.GenerateCtx) catalog.Field {
		v := pickedBond(r, ctx).CouponPct
		if v == "" {
			v = "0.000" // T-bills and strips have no coupon
		}
		return catalog.Field{Tag: TagCouponRate, Value: v}
	}
}

func maturityDateField() catalog.FieldGenerator {
	return func(r *rand.Rand, ctx *catalog.GenerateCtx) catalog.Field {
		return catalog.Field{Tag: TagMaturityDate, Value: pickedBond(r, ctx).Maturity.Format("20060102")}
	}
}

func issueDateField() catalog.FieldGenerator {
	return func(r *rand.Rand, ctx *catalog.GenerateCtx) catalog.Field {
		return catalog.Field{Tag: TagIssueDate, Value: pickedBond(r, ctx).Issue.Format("20060102")}
	}
}

func factorField() catalog.FieldGenerator {
	// For TIPS, factor reflects CPI accumulation since issue (typically
	// 1.000 - 1.250). For non-indexed, 1.000.
	return func(r *rand.Rand, ctx *catalog.GenerateCtx) catalog.Field {
		st := pickedBond(r, ctx).SecType
		if st == catalog.SecTIPS {
			// Range 1.000 - 1.250.
			tenThousandths := 10000 + r.Intn(2500) // #nosec G404
			return catalog.Field{
				Tag:   TagFactor,
				Value: fmt.Sprintf("%d.%04d", tenThousandths/10000, tenThousandths%10000),
			}
		}
		return catalog.Field{Tag: TagFactor, Value: "1.000"}
	}
}

func yieldTypeField() catalog.FieldGenerator {
	return func(r *rand.Rand, ctx *catalog.GenerateCtx) catalog.Field {
		b := pickedBond(r, ctx).Bond
		return catalog.Field{Tag: TagYieldType, Value: b.YieldType}
	}
}

func cfiCodeField() catalog.FieldGenerator {
	return func(r *rand.Rand, ctx *catalog.GenerateCtx) catalog.Field {
		b := pickedBond(r, ctx).Bond
		return catalog.Field{Tag: TagCFICode, Value: b.CFICode}
	}
}

// --- shared generators ---

func clOrdID() catalog.FieldGenerator {
	return func(r *rand.Rand, _ *catalog.GenerateCtx) catalog.Field {
		return catalog.Field{Tag: app.TagClOrdID, Value: fmt.Sprintf("BLZ-%08d", r.Intn(100000000))} // #nosec G404
	}
}
func origClOrdID() catalog.FieldGenerator {
	return func(r *rand.Rand, _ *catalog.GenerateCtx) catalog.Field {
		return catalog.Field{Tag: app.TagOrigClOrdID, Value: fmt.Sprintf("BLZ-%08d", r.Intn(100000000))} // #nosec G404
	}
}
func orderID() catalog.FieldGenerator {
	return func(r *rand.Rand, _ *catalog.GenerateCtx) catalog.Field {
		return catalog.Field{Tag: app.TagOrderID, Value: fmt.Sprintf("ORD-%010d", r.Intn(1000000000))} // #nosec G404
	}
}
func execID() catalog.FieldGenerator {
	return func(r *rand.Rand, _ *catalog.GenerateCtx) catalog.Field {
		return catalog.Field{Tag: app.TagExecID, Value: fmt.Sprintf("EXE-%010d", r.Intn(1000000000))} // #nosec G404
	}
}
func side() catalog.FieldGenerator {
	choices := []string{app.SideBuy, app.SideSell}
	return func(r *rand.Rand, _ *catalog.GenerateCtx) catalog.Field {
		return catalog.Field{Tag: app.TagSide, Value: choices[r.Intn(len(choices))]} // #nosec G404
	}
}
func ordType() catalog.FieldGenerator {
	choices := []string{app.OrdTypeMarket, app.OrdTypeLimit}
	return func(r *rand.Rand, _ *catalog.GenerateCtx) catalog.Field {
		return catalog.Field{Tag: app.TagOrdType, Value: choices[r.Intn(len(choices))]} // #nosec G404
	}
}
func tif() catalog.FieldGenerator {
	choices := []string{app.TIFDay, app.TIFGTC}
	return func(r *rand.Rand, _ *catalog.GenerateCtx) catalog.Field {
		return catalog.Field{Tag: app.TagTimeInForce, Value: choices[r.Intn(len(choices))]} // #nosec G404
	}
}
func orderQty() catalog.FieldGenerator {
	return func(r *rand.Rand, ctx *catalog.GenerateCtx) catalog.Field {
		return catalog.Field{Tag: app.TagOrderQty, Value: catalog.OrderQty(r, ctx, drawQty)}
	}
}

func drawQty(r *rand.Rand) string {
	// Treasury order size: $100k - $100MM in $100k ticks (face value).
	ticks := 1 + r.Intn(1000) // #nosec G404
	return fmt.Sprintf("%d00000", ticks)
}
func leavesQty() catalog.FieldGenerator {
	return func(r *rand.Rand, ctx *catalog.GenerateCtx) catalog.Field {
		return catalog.Field{Tag: app.TagLeavesQty, Value: catalog.OrderQty(r, ctx, drawQty)}
	}
}
func price() catalog.FieldGenerator {
	return func(r *rand.Rand, _ *catalog.GenerateCtx) catalog.Field {
		// Treasury prices in 32nds: "99-16" means 99 + 16/32 = 99.50.
		// For simplicity emit decimal 95.000-105.000.
		hundredths := 95000 + r.Intn(10000) // #nosec G404
		return catalog.Field{Tag: app.TagPrice, Value: fmt.Sprintf("%d.%03d", hundredths/1000, hundredths%1000)}
	}
}
func transactTime() catalog.FieldGenerator {
	return func(_ *rand.Rand, ctx *catalog.GenerateCtx) catalog.Field {
		v := ctx.SendingTime
		if v == "" {
			v = "19700101-00:00:00.000"
		}
		return catalog.Field{Tag: app.TagTransactTime, Value: v}
	}
}
