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
}

var instrumentsByType = map[catalog.SecurityType][]instrument{
	catalog.SecCD: {
		{"BAC-CD-90D", "060505AA1", "Bank of America", "5.25", "A-1", YieldTypeInterestBearing, PriceTypePercentOfPar, "DYZXFR"},
		{"JPM-CD-180D", "46625HBA1", "JPMorgan Chase", "5.10", "A-1+", YieldTypeInterestBearing, PriceTypePercentOfPar, "DYZXFR"},
		{"WFC-CD-30D", "949746AA1", "Wells Fargo", "5.30", "A-1", YieldTypeInterestBearing, PriceTypePercentOfPar, "DYZXFR"},
	},
	catalog.SecCP: {
		{"GE-CP-30D", "36962GAA1", "GE Capital", "", "A-1+", YieldTypeDiscount, PriceTypePerYield, "DYXXFR"},
		{"IBM-CP-60D", "459200CC1", "IBM", "", "A-1", YieldTypeDiscount, PriceTypePerYield, "DYXXFR"},
		{"CVS-CP-90D", "126650AA1", "CVS Health", "", "A-2", YieldTypeDiscount, PriceTypePerYield, "DYXXFR"},
	},
	catalog.SecBA: {
		{"BAC-BA-30D", "060505BA1", "Bank of America", "", "A-1", YieldTypeDiscount, PriceTypePerYield, "DYXXFR"},
		{"JPM-BA-90D", "46625HCC1", "JPMorgan Chase", "", "A-1+", YieldTypeDiscount, PriceTypePerYield, "DYXXFR"},
	},
	catalog.SecBN: {
		{"BAC-BN-180D", "060505CA1", "Bank of America", "5.20", "A-1", YieldTypeInterestBearing, PriceTypePercentOfPar, "DYZXFR"},
		{"JPM-BN-90D", "46625HDD1", "JPMorgan Chase", "5.15", "A-1+", YieldTypeInterestBearing, PriceTypePercentOfPar, "DYZXFR"},
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
		return instrument{Symbol: "MM-FALLBACK", Cusip: "000000AAA", Issuer: "Generic", Yield: YieldTypeDiscount, Price: PriceTypePerYield, Rating: "A-1", CFICode: "DYXXFR"}
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

// issueBaseDate anchors deterministic IssueDate/MaturityDate emission.
var issueBaseDate = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

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
	// Money market matures within ≤1y per convention. Add the random
	// days budget to the deterministic issueBaseDate so MaturityDate
	// reflects actual days-to-maturity rather than collapsing onto the
	// 15th of a month.
	return func(r *rand.Rand, _ *catalog.GenerateCtx) catalog.Field {
		days := 7 + r.Intn(359) // 7-365 days // #nosec G404
		matur := issueBaseDate.AddDate(0, 0, days)
		return catalog.Field{Tag: TagMaturityDate, Value: matur.Format("20060102")}
	}
}
func issueDateField() catalog.FieldGenerator {
	return func(_ *rand.Rand, _ *catalog.GenerateCtx) catalog.Field {
		return catalog.Field{Tag: TagIssueDate, Value: issueBaseDate.Format("20060102")}
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
	return func(r *rand.Rand, _ *catalog.GenerateCtx) catalog.Field {
		mm := 1 + r.Intn(100) // $1MM - $100MM // #nosec G404
		return catalog.Field{Tag: app.TagOrderQty, Value: fmt.Sprintf("%d000000", mm)}
	}
}
func leavesQty() catalog.FieldGenerator {
	return func(r *rand.Rand, _ *catalog.GenerateCtx) catalog.Field {
		mm := 1 + r.Intn(100) // #nosec G404
		return catalog.Field{Tag: app.TagLeavesQty, Value: fmt.Sprintf("%d000000", mm)}
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
