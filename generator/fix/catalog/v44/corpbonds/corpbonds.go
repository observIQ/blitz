// Package corpbonds registers FIX 4.4 MessageDefinitions for the
// Corporate / Credit Fixed Income asset category. SecurityType
// values: CORP (corporate bond), CB (convertible), MUNI (municipal),
// MUNIFIDC, GO (general obligation), REV (revenue).
package corpbonds

import (
	"fmt"
	"math/rand"
	"strings"
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
)

const SecurityIDSourceCUSIP = "1"

// PriceType (423) — bonds quote on a price basis OR yield basis;
// "1" = % of par (the most common bond price quoting convention).
const PriceTypePercentOfPar = "1"

// bond is one row in the corp-bonds table. Symbol holds {YY}, the two-digit
// maturity year; Maturity is the term in years at issue.
type bond struct {
	Symbol      string
	Cusip       string
	Coupon      string
	Maturity    int // years out at issue
	Country     string
	Rating      string
	CFICode     string
	Convertible bool
}

var corpBondsByType = map[catalog.SecurityType][]bond{
	catalog.SecCORP: {
		{"AAPL-{YY}", "037833902", "3.250", 10, "US", "AA+", "DBFNFR", false},
		{"MSFT-{YY}", "594918906", "3.450", 10, "US", "AAA", "DBFNFR", false},
		{"GS-{YY}", "38141G906", "4.500", 8, "US", "BBB+", "DBFNFR", false},
		{"JPM-{YY}", "46625H902", "3.875", 10, "US", "A", "DBFNFR", false},
		{"DIS-{YY}", "25468P908", "3.350", 8, "US", "A-", "DBFNFR", false},
	},
	catalog.SecCB: {
		{"TWLO-CB-{YY}", "90138F904", "0.375", 5, "US", "BB", "DBCNFR", true},
		{"WORK-CB-{YY}", "98138H903", "0.500", 5, "US", "BB+", "DBCNFR", true},
		{"SPLK-CB-{YY}", "84860W904", "1.125", 6, "US", "BB", "DBCNFR", true},
	},
	catalog.SecMUNI: {
		{"NYC-MUNI-{YY}", "64966M901", "3.000", 7, "US", "AA", "DBSGFR", false},
		{"LA-MUNI-{YY}", "544445901", "2.875", 8, "US", "AA-", "DBSGFR", false},
	},
	catalog.SecMUNIFIDC: {
		{"MUNI-FDIC-{YY}", "999999907", "3.250", 8, "US", "AAA", "DBSGFR", false},
	},
	catalog.SecGO: {
		{"NYC-GO-{YY}", "649683901", "3.100", 6, "US", "AA+", "DBSGFR", false},
		{"CA-GO-{YY}", "13063D900", "2.950", 8, "US", "AA", "DBSGFR", false},
	},
	catalog.SecREV: {
		{"PA-TURNPIKE-{YY}", "709224901", "3.500", 10, "US", "A+", "DBSGFR", false},
		{"NJ-EDA-{YY}", "646136903", "3.300", 7, "US", "A", "DBSGFR", false},
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
			Version: catalog.V44, MsgType: mt, AssetCategory: catalog.AssetCategoryCorpBonds,
			Fields: fieldsFor(mt),
		})
	}
}

func fieldsFor(msgType string) []catalog.FieldGenerator {
	switch msgType {
	case app.MsgTypeNewOrderSingle:
		return concat([]catalog.FieldGenerator{clOrdID()}, instrumentBlock(),
			[]catalog.FieldGenerator{side(), ordType(), tif(), orderQty(), price(), priceType(), transactTime()})
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
		couponRateField(), maturityDateField(), issueDateField(),
		countryField(), creditRatingField(), cfiCodeField(),
	}
}

func pickSecurityType(r *rand.Rand) catalog.SecurityType {
	types := []catalog.SecurityType{
		catalog.SecCORP, catalog.SecCB, catalog.SecMUNI, catalog.SecMUNIFIDC,
		catalog.SecGO, catalog.SecREV,
	}
	return types[r.Intn(len(types))] // #nosec G404
}

func pickBond(r *rand.Rand, st catalog.SecurityType) bond {
	bs := corpBondsByType[st]
	if len(bs) == 0 {
		return bond{Symbol: "CORP-FALLBACK-{YY}", Cusip: "037833902", Coupon: "3.000", Maturity: 10, Country: "US", Rating: "BBB", CFICode: "DBFNFR"}
	}
	return bs[r.Intn(len(bs))] // #nosec G404
}

// pickKey is the GenerateCtx.Memo key for the message's picked
// (SecurityType, bond) pair, so every instrument field describes one row.
type pickKey struct{}

type pickedPair struct {
	SecType         catalog.SecurityType
	Bond            bond
	Issue, Maturity time.Time
}

// symbol is the bond's symbol with {YY} rendered as its maturity year.
func (p pickedPair) symbol() string {
	return strings.ReplaceAll(p.Bond.Symbol, "{YY}", fmt.Sprintf("%02d", p.Maturity.Year()%100))
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
	// Outstanding on the trade date: issue and maturity span the term.
	p.Issue, p.Maturity = catalog.BondDates(r, ctx.TradeDate(), 12*max(p.Bond.Maturity, 1))
	ctx.Memo[pickKey{}] = p
	return p
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
		return catalog.Field{Tag: TagSecurityID, Value: pickedBond(r, ctx).Bond.Cusip}
	}
}
func couponRateField() catalog.FieldGenerator {
	return func(r *rand.Rand, ctx *catalog.GenerateCtx) catalog.Field {
		return catalog.Field{Tag: TagCouponRate, Value: pickedBond(r, ctx).Bond.Coupon}
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
func countryField() catalog.FieldGenerator {
	return func(r *rand.Rand, ctx *catalog.GenerateCtx) catalog.Field {
		return catalog.Field{Tag: TagCountryOfIssue, Value: pickedBond(r, ctx).Bond.Country}
	}
}
func creditRatingField() catalog.FieldGenerator {
	return func(r *rand.Rand, ctx *catalog.GenerateCtx) catalog.Field {
		return catalog.Field{Tag: TagCreditRating, Value: pickedBond(r, ctx).Bond.Rating}
	}
}
func cfiCodeField() catalog.FieldGenerator {
	return func(r *rand.Rand, ctx *catalog.GenerateCtx) catalog.Field {
		return catalog.Field{Tag: TagCFICode, Value: pickedBond(r, ctx).Bond.CFICode}
	}
}
func priceType() catalog.FieldGenerator {
	return catalog.LiteralField(TagPriceType, PriceTypePercentOfPar)
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
	ticks := 1 + r.Intn(1000) // #nosec G404
	return fmt.Sprintf("%d000", ticks)
}
func leavesQty() catalog.FieldGenerator {
	return func(r *rand.Rand, ctx *catalog.GenerateCtx) catalog.Field {
		return catalog.Field{Tag: app.TagLeavesQty, Value: catalog.OrderQty(r, ctx, drawQty)}
	}
}
func price() catalog.FieldGenerator {
	return func(r *rand.Rand, _ *catalog.GenerateCtx) catalog.Field {
		// Bond clean price as % of par: 80.000 to 120.000.
		hundredths := 80000 + r.Intn(40000) // #nosec G404
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
