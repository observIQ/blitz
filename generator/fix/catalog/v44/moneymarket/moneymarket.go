// Package moneymarket registers FIX 4.4 MessageDefinitions for the
// Money Market asset category — CD (certificate of deposit), CP
// (commercial paper), BA (banker's acceptance), BN (banker's note).
//
// Money market instruments mature in ≤ 1 year. Pricing convention is
// discount yield for CP/BA, interest-bearing for CD/BN.
package moneymarket

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/observiq/blitz/generator/fix/catalog"
	"github.com/observiq/blitz/generator/fix/catalog/v44/app"
)

const (
	TagCouponRate     catalog.Tag = 223
	TagMaturityDate   catalog.Tag = 541
	TagIssueDate      catalog.Tag = 225
	TagCountryOfIssue catalog.Tag = 470
	TagCreditRating   catalog.Tag = 255
	TagCFICode        catalog.Tag = 461
	TagSecurityID     catalog.Tag = 48
	TagSecurityIDSrc  catalog.Tag = 22
	TagYieldType      catalog.Tag = 235
	TagPriceType      catalog.Tag = 423
	TagIssuer         catalog.Tag = 106
)

const SecurityIDSourceCUSIP = "1"

// YieldType (235) for money market.
const (
	YieldTypeDiscount        = "DISC" // CP, BA
	YieldTypeInterestBearing = "INT"  // CD, BN
)

// PriceType (423).
const (
	PriceTypePercentOfPar = "1"
	PriceTypePerYield     = "9"
)

type instrument struct {
	Symbol  string
	Cusip   string
	Issuer  string
	Coupon  string // empty for discount instruments
	Rating  string
	Yield   string
	Price   string
	CFICode string
	Tenor   int // days to maturity, the tenor the symbol names
}

var instrumentsByType = map[catalog.SecurityType][]instrument{
	catalog.SecCD: {
		{"BAC-CD-90D", "060505906", "Bank of America", "5.25", "A-1", YieldTypeInterestBearing, PriceTypePercentOfPar, "DYZXFR", 90},
		{"JPM-CD-180D", "46625H910", "JPMorgan Chase", "5.10", "A-1+", YieldTypeInterestBearing, PriceTypePercentOfPar, "DYZXFR", 180},
		{"WFC-CD-30D", "949746903", "Wells Fargo", "5.30", "A-1", YieldTypeInterestBearing, PriceTypePercentOfPar, "DYZXFR", 30},
	},
	catalog.SecCP: {
		{"GE-CP-30D", "36962G906", "GE Capital", "", "A-1+", YieldTypeDiscount, PriceTypePerYield, "DYXXFR", 30},
		{"IBM-CP-60D", "459200903", "IBM", "", "A-1", YieldTypeDiscount, PriceTypePerYield, "DYXXFR", 60},
		{"CVS-CP-90D", "126650902", "CVS Health", "", "A-2", YieldTypeDiscount, PriceTypePerYield, "DYXXFR", 90},
	},
	catalog.SecBA: {
		{"BAC-BA-30D", "060505930", "Bank of America", "", "A-1", YieldTypeDiscount, PriceTypePerYield, "DYXXFR", 30},
		{"JPM-BA-90D", "46625H928", "JPMorgan Chase", "", "A-1+", YieldTypeDiscount, PriceTypePerYield, "DYXXFR", 90},
	},
	catalog.SecBN: {
		{"BAC-BN-180D", "060505914", "Bank of America", "5.20", "A-1", YieldTypeInterestBearing, PriceTypePercentOfPar, "DYZXFR", 180},
		{"JPM-BN-90D", "46625H936", "JPMorgan Chase", "5.15", "A-1+", YieldTypeInterestBearing, PriceTypePercentOfPar, "DYZXFR", 90},
	},
}

func init()       { registerAll() }
func Reregister() { catalog.ResetForTest(); registerAll() }

func registerAll() {
	for _, mt := range []string{
		app.MsgTypeNewOrderSingle, app.MsgTypeExecutionReport,
		app.MsgTypeOrderCancelRequest, app.MsgTypeOrderCancelReplaceRequest,
		app.MsgTypeOrderStatusRequest,
	} {
		catalog.Register(catalog.MessageDefinition{
			Version: catalog.V44, MsgType: mt, AssetCategory: catalog.AssetCategoryMoneyMarket,
			Fields: fieldsFor(mt),
		})
	}
}

func fieldsFor(msgType string) []catalog.FieldGenerator {
	switch msgType {
	case app.MsgTypeNewOrderSingle:
		return concat([]catalog.FieldGenerator{clOrdID()}, instrumentBlock(),
			[]catalog.FieldGenerator{side(), ordType(), tif(), orderQty(), price(), priceTypeField(), transactTime()})
	case app.MsgTypeExecutionReport:
		return concat(
			[]catalog.FieldGenerator{orderID(), clOrdID(), execID(),
				catalog.LiteralField(app.TagExecType, app.ExecTypeNew),
				catalog.LiteralField(app.TagOrdStatus, app.OrdStatusNew)},
			instrumentBlock(),
			[]catalog.FieldGenerator{side(), orderQty(),
				catalog.IntField(app.TagCumQty, 0), leavesQty(),
				catalog.LiteralField(app.TagAvgPx, "0.000")})
	case app.MsgTypeOrderCancelRequest:
		return concat([]catalog.FieldGenerator{origClOrdID(), clOrdID()}, instrumentBlock(),
			[]catalog.FieldGenerator{side(), transactTime()})
	case app.MsgTypeOrderCancelReplaceRequest:
		return concat([]catalog.FieldGenerator{origClOrdID(), clOrdID()}, instrumentBlock(),
			[]catalog.FieldGenerator{side(), transactTime(), orderQty(), price(), ordType()})
	case app.MsgTypeOrderStatusRequest:
		return concat([]catalog.FieldGenerator{origClOrdID()}, instrumentBlock(),
			[]catalog.FieldGenerator{side()})
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
		symbolField(), securityTypeField(), securityIDField(),
		catalog.LiteralField(TagSecurityIDSrc, SecurityIDSourceCUSIP),
		issuerField(), couponRateField(), maturityDateField(), issueDateField(),
		catalog.LiteralField(TagCountryOfIssue, "US"),
		creditRatingField(), yieldTypeField(), cfiCodeField(),
	}
}

func pickSecurityType(r *rand.Rand) catalog.SecurityType {
	types := []catalog.SecurityType{
		catalog.SecCD, catalog.SecCP, catalog.SecBA, catalog.SecBN,
	}
	return types[r.Intn(len(types))] // #nosec G404
}

func pickInstrument(r *rand.Rand, st catalog.SecurityType) instrument {
	is := instrumentsByType[st]
	if len(is) == 0 {
		return instrument{Symbol: "MM-FALLBACK", Cusip: "000000935", Issuer: "Generic", Yield: YieldTypeDiscount, Price: PriceTypePerYield, Rating: "A-1", CFICode: "DYXXFR", Tenor: 30}
	}
	return is[r.Intn(len(is))] // #nosec G404
}

// pickKey is the Memo key for the picked (SecurityType, instrument)
// pair so every instrument-block field in one message agrees on a
// single row. Keeps YieldType/PriceType coherent with SecurityType
// (CD/BN → interest-bearing; CP/BA → discount).
type pickKey struct{}

type pickedPair struct {
	SecType    catalog.SecurityType
	Instrument instrument
}

func pickedInstrument(r *rand.Rand, ctx *catalog.GenerateCtx) pickedPair {
	if ctx.Memo == nil {
		ctx.Memo = map[any]any{}
	}
	if v, ok := ctx.Memo[pickKey{}]; ok {
		return v.(pickedPair)
	}
	st := pickSecurityType(r)
	p := pickedPair{SecType: st, Instrument: pickInstrument(r, st)}
	ctx.Memo[pickKey{}] = p
	return p
}

func symbolField() catalog.FieldGenerator {
	return func(r *rand.Rand, ctx *catalog.GenerateCtx) catalog.Field {
		return catalog.Field{Tag: app.TagSymbol, Value: pickedInstrument(r, ctx).Instrument.Symbol}
	}
}
func securityTypeField() catalog.FieldGenerator {
	return func(r *rand.Rand, ctx *catalog.GenerateCtx) catalog.Field {
		return catalog.Field{Tag: app.TagSecurityType, Value: string(pickedInstrument(r, ctx).SecType)}
	}
}
func securityIDField() catalog.FieldGenerator {
	return func(r *rand.Rand, ctx *catalog.GenerateCtx) catalog.Field {
		return catalog.Field{Tag: TagSecurityID, Value: pickedInstrument(r, ctx).Instrument.Cusip}
	}
}
func issuerField() catalog.FieldGenerator {
	return func(r *rand.Rand, ctx *catalog.GenerateCtx) catalog.Field {
		return catalog.Field{Tag: TagIssuer, Value: pickedInstrument(r, ctx).Instrument.Issuer}
	}
}
func couponRateField() catalog.FieldGenerator {
	return func(r *rand.Rand, ctx *catalog.GenerateCtx) catalog.Field {
		c := pickedInstrument(r, ctx).Instrument.Coupon
		if c == "" {
			c = "0.000"
		}
		return catalog.Field{Tag: TagCouponRate, Value: c}
	}
}
func maturityDateField() catalog.FieldGenerator {
	// Paper matures at the tenor its symbol names: the issue date plus
	// that many days, rolled to a business day.
	return func(r *rand.Rand, ctx *catalog.GenerateCtx) catalog.Field {
		days := pickedInstrument(r, ctx).Instrument.Tenor
		matur := catalog.NextBusinessDay(issueDate(ctx).AddDate(0, 0, days))
		return catalog.Field{Tag: TagMaturityDate, Value: matur.Format("20060102")}
	}
}

// issueDate is primary-market issuance: the trade date, rolled to a business
// day, since paper is not issued on a weekend.
func issueDate(ctx *catalog.GenerateCtx) time.Time {
	return catalog.NextBusinessDay(ctx.TradeDate())
}

func issueDateField() catalog.FieldGenerator {
	return func(_ *rand.Rand, ctx *catalog.GenerateCtx) catalog.Field {
		return catalog.Field{Tag: TagIssueDate, Value: issueDate(ctx).Format("20060102")}
	}
}
func creditRatingField() catalog.FieldGenerator {
	return func(r *rand.Rand, ctx *catalog.GenerateCtx) catalog.Field {
		return catalog.Field{Tag: TagCreditRating, Value: pickedInstrument(r, ctx).Instrument.Rating}
	}
}
func yieldTypeField() catalog.FieldGenerator {
	return func(r *rand.Rand, ctx *catalog.GenerateCtx) catalog.Field {
		return catalog.Field{Tag: TagYieldType, Value: pickedInstrument(r, ctx).Instrument.Yield}
	}
}
func cfiCodeField() catalog.FieldGenerator {
	return func(r *rand.Rand, ctx *catalog.GenerateCtx) catalog.Field {
		return catalog.Field{Tag: TagCFICode, Value: pickedInstrument(r, ctx).Instrument.CFICode}
	}
}
func priceTypeField() catalog.FieldGenerator {
	return func(r *rand.Rand, ctx *catalog.GenerateCtx) catalog.Field {
		return catalog.Field{Tag: TagPriceType, Value: pickedInstrument(r, ctx).Instrument.Price}
	}
}

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
	return catalog.LiteralField(app.TagOrdType, app.OrdTypeLimit)
}
func tif() catalog.FieldGenerator {
	return catalog.LiteralField(app.TagTimeInForce, app.TIFDay)
}
func orderQty() catalog.FieldGenerator {
	return func(r *rand.Rand, ctx *catalog.GenerateCtx) catalog.Field {
		return catalog.Field{Tag: app.TagOrderQty, Value: catalog.OrderQty(r, ctx, drawQty)}
	}
}

func drawQty(r *rand.Rand) string {
	mm := 1 + r.Intn(100) // $1MM - $100MM // #nosec G404
	return fmt.Sprintf("%d000000", mm)
}
func leavesQty() catalog.FieldGenerator {
	return func(r *rand.Rand, ctx *catalog.GenerateCtx) catalog.Field {
		return catalog.Field{Tag: app.TagLeavesQty, Value: catalog.OrderQty(r, ctx, drawQty)}
	}
}
func price() catalog.FieldGenerator {
	return func(r *rand.Rand, _ *catalog.GenerateCtx) catalog.Field {
		// Money market prices typically 95.000-100.000 (deep discount for short-dated).
		hundredths := 95000 + r.Intn(5000) // #nosec G404
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
