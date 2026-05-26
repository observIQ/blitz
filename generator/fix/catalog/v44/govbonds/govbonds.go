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

// bond is one row in the gov-bonds table. Cusip is the US 9-char
// CUSIP; CouponPct is the bond's coupon rate (0 for bills).
type bond struct {
	Symbol      string
	Cusip       string
	CouponPct   string // e.g. "4.250"; empty for TBILL/TINT
	MaturityYrs int    // years to maturity at issue
	YieldType   string
	CFICode     string
}

var govBondsByType = map[catalog.SecurityType][]bond{
	catalog.SecTBILL: {
		{"T-BILL-4W", "912797GZ7", "", 0, YieldTypeDiscount, "DBTGFR"},
		{"T-BILL-13W", "912797HA1", "", 0, YieldTypeDiscount, "DBTGFR"},
		{"T-BILL-26W", "912797HB9", "", 0, YieldTypeDiscount, "DBTGFR"},
		{"T-BILL-52W", "912797HC7", "", 0, YieldTypeDiscount, "DBTGFR"},
	},
	catalog.SecTNOTE: {
		{"T-NOTE-2Y", "91282CKL5", "4.500", 2, YieldTypeBondEquivalent, "DBTGFR"},
		{"T-NOTE-3Y", "91282CKM3", "4.375", 3, YieldTypeBondEquivalent, "DBTGFR"},
		{"T-NOTE-5Y", "91282CKN1", "4.250", 5, YieldTypeBondEquivalent, "DBTGFR"},
		{"T-NOTE-7Y", "91282CKP6", "4.250", 7, YieldTypeBondEquivalent, "DBTGFR"},
		{"T-NOTE-10Y", "91282CKQ4", "4.375", 10, YieldTypeBondEquivalent, "DBTGFR"},
	},
	catalog.SecTBOND: {
		{"T-BOND-20Y", "912810TC8", "4.625", 20, YieldTypeBondEquivalent, "DBTGFR"},
		{"T-BOND-30Y", "912810TD6", "4.500", 30, YieldTypeBondEquivalent, "DBTGFR"},
	},
	catalog.SecTIPS: {
		{"TIPS-5Y", "91282CLR0", "1.625", 5, YieldTypeREALYIELD, "DBTGXR"},
		{"TIPS-10Y", "91282CLS8", "2.000", 10, YieldTypeREALYIELD, "DBTGXR"},
		{"TIPS-30Y", "91282CLT6", "2.125", 30, YieldTypeREALYIELD, "DBTGXR"},
	},
	catalog.SecTINT: {
		// Stripped interest payments — symbols vary; CFI: DBTGZR
		{"STRIP-INT-2030", "912833NK4", "", 0, YieldTypeBondEquivalent, "DBTGZR"},
		{"STRIP-INT-2040", "912833NL2", "", 0, YieldTypeBondEquivalent, "DBTGZR"},
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
	if len(bs) == 0 {
		return bond{Symbol: "T-FALLBACK", Cusip: "912828AAA", YieldType: YieldTypeBondEquivalent, CFICode: "DBTGFR"}
	}
	return bs[r.Intn(len(bs))] // #nosec G404
}

func symbolField() catalog.FieldGenerator {
	return func(r *rand.Rand, _ *catalog.GenerateCtx) catalog.Field {
		b := pickBond(r, pickSecurityType(r))
		return catalog.Field{Tag: app.TagSymbol, Value: b.Symbol}
	}
}

func securityTypeField() catalog.FieldGenerator {
	return func(r *rand.Rand, _ *catalog.GenerateCtx) catalog.Field {
		return catalog.Field{Tag: app.TagSecurityType, Value: string(pickSecurityType(r))}
	}
}

func securityIDField() catalog.FieldGenerator {
	return func(r *rand.Rand, _ *catalog.GenerateCtx) catalog.Field {
		b := pickBond(r, pickSecurityType(r))
		return catalog.Field{Tag: TagSecurityID, Value: b.Cusip}
	}
}

func couponRateField() catalog.FieldGenerator {
	return func(r *rand.Rand, _ *catalog.GenerateCtx) catalog.Field {
		b := pickBond(r, pickSecurityType(r))
		v := b.CouponPct
		if v == "" {
			v = "0.000" // T-bills and strips have no coupon
		}
		return catalog.Field{Tag: TagCouponRate, Value: v}
	}
}

func maturityDateField() catalog.FieldGenerator {
	return func(r *rand.Rand, _ *catalog.GenerateCtx) catalog.Field {
		yrs := 1 + r.Intn(30) // #nosec G404
		year := 2026 + yrs
		month := 1 + r.Intn(12) // #nosec G404
		day := 15
		return catalog.Field{Tag: TagMaturityDate, Value: fmt.Sprintf("%d%02d%02d", year, month, day)}
	}
}

func issueDateField() catalog.FieldGenerator {
	return func(r *rand.Rand, _ *catalog.GenerateCtx) catalog.Field {
		year := 2024 + r.Intn(3) // #nosec G404
		month := 1 + r.Intn(12)  // #nosec G404
		day := 15
		return catalog.Field{Tag: TagIssueDate, Value: fmt.Sprintf("%d%02d%02d", year, month, day)}
	}
}

func factorField() catalog.FieldGenerator {
	// For TIPS, factor reflects CPI accumulation since issue (typically
	// 1.000 - 1.250). For non-indexed, 1.000.
	return func(r *rand.Rand, _ *catalog.GenerateCtx) catalog.Field {
		st := pickSecurityType(r)
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
	return func(r *rand.Rand, _ *catalog.GenerateCtx) catalog.Field {
		b := pickBond(r, pickSecurityType(r))
		return catalog.Field{Tag: TagYieldType, Value: b.YieldType}
	}
}

func cfiCodeField() catalog.FieldGenerator {
	return func(r *rand.Rand, _ *catalog.GenerateCtx) catalog.Field {
		b := pickBond(r, pickSecurityType(r))
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
	return func(r *rand.Rand, _ *catalog.GenerateCtx) catalog.Field {
		// Treasury order size: $100k - $100MM in $100k ticks (face value).
		ticks := 1 + r.Intn(1000) // #nosec G404
		return catalog.Field{Tag: app.TagOrderQty, Value: fmt.Sprintf("%d00000", ticks)}
	}
}
func leavesQty() catalog.FieldGenerator {
	return func(r *rand.Rand, _ *catalog.GenerateCtx) catalog.Field {
		ticks := 1 + r.Intn(1000) // #nosec G404
		return catalog.Field{Tag: app.TagLeavesQty, Value: fmt.Sprintf("%d00000", ticks)}
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
