// Package corpbonds registers FIX 4.4 MessageDefinitions for the
// Corporate / Credit Fixed Income asset category. SecurityType
// values: CORP (corporate bond), CB (convertible), MUNI (municipal),
// MUNIFIDC, GO (general obligation), REV (revenue).
package corpbonds

import (
	"fmt"
	"math/rand"

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
		{"AAPL-30", "037833CA8", "3.250", 10, "US", "AA+", "DBFNFR", false},
		{"MSFT-31", "594918BU6", "3.450", 10, "US", "AAA", "DBFNFR", false},
		{"GS-30", "38141GVN8", "4.500", 8, "US", "BBB+", "DBFNFR", false},
		{"JPM-32", "46625HKB3", "3.875", 10, "US", "A", "DBFNFR", false},
		{"DIS-29", "25468PCK1", "3.350", 8, "US", "A-", "DBFNFR", false},
	},
	catalog.SecCB: {
		{"TWLO-CB-25", "90138FAF2", "0.375", 5, "US", "BB", "DBCNFR", true},
		{"WORK-CB-25", "98138HAA0", "0.500", 5, "US", "BB+", "DBCNFR", true},
		{"SPLK-CB-26", "84860WAH9", "1.125", 6, "US", "BB", "DBCNFR", true},
	},
	catalog.SecMUNI: {
		{"NYC-MUNI-29", "64966MAA1", "3.000", 7, "US", "AA", "DBSGFR", false},
		{"LA-MUNI-30", "544445AA1", "2.875", 8, "US", "AA-", "DBSGFR", false},
	},
	catalog.SecMUNIFIDC: {
		{"MUNI-FDIC-30", "999999AA1", "3.250", 8, "US", "AAA", "DBSGFR", false},
	},
	catalog.SecGO: {
		{"NYC-GO-28", "649683AA1", "3.100", 6, "US", "AA+", "DBSGFR", false},
		{"CA-GO-30", "13063DAA1", "2.950", 8, "US", "AA", "DBSGFR", false},
	},
	catalog.SecREV: {
		{"PA-TURNPIKE-32", "709224AA1", "3.500", 10, "US", "A+", "DBSGFR", false},
		{"NJ-EDA-29", "646136AA1", "3.300", 7, "US", "A", "DBSGFR", false},
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
		return bond{Symbol: "CORP-FALLBACK", Cusip: "037833CA8", Coupon: "3.000", Country: "US", Rating: "BBB", CFICode: "DBFNFR"}
	}
	return bs[r.Intn(len(bs))] // #nosec G404
}

func symbolField() catalog.FieldGenerator {
	return func(r *rand.Rand, _ *catalog.GenerateCtx) catalog.Field {
		return catalog.Field{Tag: app.TagSymbol, Value: pickBond(r, pickSecurityType(r)).Symbol}
	}
}
func securityTypeField() catalog.FieldGenerator {
	return func(r *rand.Rand, _ *catalog.GenerateCtx) catalog.Field {
		return catalog.Field{Tag: app.TagSecurityType, Value: string(pickSecurityType(r))}
	}
}
func securityIDField() catalog.FieldGenerator {
	return func(r *rand.Rand, _ *catalog.GenerateCtx) catalog.Field {
		return catalog.Field{Tag: TagSecurityID, Value: pickBond(r, pickSecurityType(r)).Cusip}
	}
}
func couponRateField() catalog.FieldGenerator {
	return func(r *rand.Rand, _ *catalog.GenerateCtx) catalog.Field {
		return catalog.Field{Tag: TagCouponRate, Value: pickBond(r, pickSecurityType(r)).Coupon}
	}
}
func maturityDateField() catalog.FieldGenerator {
	return func(r *rand.Rand, _ *catalog.GenerateCtx) catalog.Field {
		yrs := 1 + r.Intn(30)                                                                              // #nosec G404
		return catalog.Field{Tag: TagMaturityDate, Value: fmt.Sprintf("%d%02d15", 2026+yrs, 1+r.Intn(12))} // #nosec G404
	}
}
func issueDateField() catalog.FieldGenerator {
	return func(r *rand.Rand, _ *catalog.GenerateCtx) catalog.Field {
		return catalog.Field{Tag: TagIssueDate, Value: fmt.Sprintf("%d%02d15", 2020+r.Intn(6), 1+r.Intn(12))} // #nosec G404
	}
}
func countryField() catalog.FieldGenerator {
	return func(r *rand.Rand, _ *catalog.GenerateCtx) catalog.Field {
		return catalog.Field{Tag: TagCountryOfIssue, Value: pickBond(r, pickSecurityType(r)).Country}
	}
}
func creditRatingField() catalog.FieldGenerator {
	return func(r *rand.Rand, _ *catalog.GenerateCtx) catalog.Field {
		return catalog.Field{Tag: TagCreditRating, Value: pickBond(r, pickSecurityType(r)).Rating}
	}
}
func cfiCodeField() catalog.FieldGenerator {
	return func(r *rand.Rand, _ *catalog.GenerateCtx) catalog.Field {
		return catalog.Field{Tag: TagCFICode, Value: pickBond(r, pickSecurityType(r)).CFICode}
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
	return func(r *rand.Rand, _ *catalog.GenerateCtx) catalog.Field {
		ticks := 1 + r.Intn(1000) // #nosec G404
		return catalog.Field{Tag: app.TagOrderQty, Value: fmt.Sprintf("%d000", ticks)}
	}
}
func leavesQty() catalog.FieldGenerator {
	return func(r *rand.Rand, _ *catalog.GenerateCtx) catalog.Field {
		ticks := 1 + r.Intn(1000) // #nosec G404
		return catalog.Field{Tag: app.TagLeavesQty, Value: fmt.Sprintf("%d000", ticks)}
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
