// Package structured registers FIX 4.4 MessageDefinitions for the
// Structured Products asset category — ABS, MBS, TMBS, CMBS, CDO.
package structured

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
	TagFactor         catalog.Tag = 228 // Pool factor: current notional / original notional.
	TagCountryOfIssue catalog.Tag = 470
	TagCreditRating   catalog.Tag = 255
	TagCFICode        catalog.Tag = 461
	TagSecurityID     catalog.Tag = 48
	TagSecurityIDSrc  catalog.Tag = 22
	TagPool           catalog.Tag = 691  // Pool ID
	TagTranche        catalog.Tag = 1212 // Tranche (CDO attachment-point label, conventionally encoded)
)

const SecurityIDSourceCUSIP = "1"

type instrument struct {
	Symbol  string
	Cusip   string
	Coupon  string
	Pool    string
	Tranche string
	Rating  string
	CFICode string
}

var instrumentsByType = map[catalog.SecurityType][]instrument{
	catalog.SecABS: {
		{"CARD-ABS-2026A", "000000AA1", "5.250", "POOL-CARD-2026A", "A1", "AAA", "DAPSFR"},
		{"AUTO-ABS-2026B", "000000AB9", "4.875", "POOL-AUTO-2026B", "A2", "AAA", "DAPSFR"},
		{"SLABS-2026C", "000000AC7", "5.625", "POOL-SL-2026C", "A1", "AA", "DAPSFR"},
	},
	catalog.SecMBS: {
		{"FNMA-30YR-2026", "31418ABC4", "5.500", "POOL-FNMA-2026A", "", "AAA", "DAMSFR"},
		{"FHLMC-15YR-2026", "3132ABCD7", "4.500", "POOL-FHLMC-2026B", "", "AAA", "DAMSFR"},
		{"GNMA-30YR-2026", "36202ABCD9", "5.250", "POOL-GNMA-2026C", "", "AAA", "DAMSFR"},
	},
	catalog.SecTMBS: {
		{"TBA-FNMA-30YR-5.5", "01F052694", "5.500", "TBA-FNMA-30Y", "", "AAA", "DAMSFR"},
		{"TBA-GNMA-30YR-5.0", "21H050695", "5.000", "TBA-GNMA-30Y", "", "AAA", "DAMSFR"},
	},
	catalog.SecCMBS: {
		{"BBCMS-2026-C5", "05544RAA1", "5.250", "POOL-CMBS-2026C5", "A2", "AAA", "DAMSFR"},
		{"BANK-2026-BNK48", "06054GAA1", "5.500", "POOL-CMBS-2026B48", "A3", "AAA", "DAMSFR"},
	},
	catalog.SecCDO: {
		{"OAKWOOD-CLO-2026-1", "67402JAA1", "6.500", "POOL-CLO-2026-1", "A-1", "AAA", "DBOSFR"},
		{"MAGNETITE-CLO-2026", "55946TAA1", "6.250", "POOL-CLO-2026-2", "B", "A", "DBOSFR"},
		{"VOYA-CLO-2026", "928962AA1", "6.750", "POOL-CLO-2026-3", "C", "BBB", "DBOSFR"},
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
			Version: catalog.V44, MsgType: mt, AssetCategory: catalog.AssetCategoryStructured,
			Fields: fieldsFor(mt),
		})
	}
}

func fieldsFor(msgType string) []catalog.FieldGenerator {
	switch msgType {
	case app.MsgTypeNewOrderSingle:
		return concat([]catalog.FieldGenerator{clOrdID()}, instrumentBlock(),
			[]catalog.FieldGenerator{side(), ordType(), tif(), orderQty(), price(), transactTime()})
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
		factorField(), poolField(), trancheField(),
		catalog.LiteralField(TagCountryOfIssue, "US"),
		creditRatingField(), cfiCodeField(),
	}
}

func pickSecurityType(r *rand.Rand) catalog.SecurityType {
	types := []catalog.SecurityType{
		catalog.SecABS, catalog.SecMBS, catalog.SecTMBS, catalog.SecCMBS, catalog.SecCDO,
	}
	return types[r.Intn(len(types))] // #nosec G404
}

func pickInstrument(r *rand.Rand, st catalog.SecurityType) instrument {
	is := instrumentsByType[st]
	if len(is) == 0 {
		return instrument{Symbol: "STRUCT-FALLBACK", Cusip: "000000AAA", Coupon: "5.000", Rating: "AAA", CFICode: "DAPSFR"}
	}
	return is[r.Intn(len(is))] // #nosec G404
}

func symbolField() catalog.FieldGenerator {
	return func(r *rand.Rand, _ *catalog.GenerateCtx) catalog.Field {
		return catalog.Field{Tag: app.TagSymbol, Value: pickInstrument(r, pickSecurityType(r)).Symbol}
	}
}
func securityTypeField() catalog.FieldGenerator {
	return func(r *rand.Rand, _ *catalog.GenerateCtx) catalog.Field {
		return catalog.Field{Tag: app.TagSecurityType, Value: string(pickSecurityType(r))}
	}
}
func securityIDField() catalog.FieldGenerator {
	return func(r *rand.Rand, _ *catalog.GenerateCtx) catalog.Field {
		return catalog.Field{Tag: TagSecurityID, Value: pickInstrument(r, pickSecurityType(r)).Cusip}
	}
}
func couponRateField() catalog.FieldGenerator {
	return func(r *rand.Rand, _ *catalog.GenerateCtx) catalog.Field {
		return catalog.Field{Tag: TagCouponRate, Value: pickInstrument(r, pickSecurityType(r)).Coupon}
	}
}
func maturityDateField() catalog.FieldGenerator {
	return func(r *rand.Rand, _ *catalog.GenerateCtx) catalog.Field {
		yrs := 10 + r.Intn(20)                                                                             // 10-30y // #nosec G404
		return catalog.Field{Tag: TagMaturityDate, Value: fmt.Sprintf("%d%02d15", 2026+yrs, 1+r.Intn(12))} // #nosec G404
	}
}
func issueDateField() catalog.FieldGenerator {
	return func(r *rand.Rand, _ *catalog.GenerateCtx) catalog.Field {
		return catalog.Field{Tag: TagIssueDate, Value: fmt.Sprintf("%d%02d15", 2022+r.Intn(4), 1+r.Intn(12))} // #nosec G404
	}
}
func factorField() catalog.FieldGenerator {
	// Pool factor: current notional / original notional. Decays as pool
	// pays down. Range 0.5000 (heavily amortized) - 1.0000 (just issued).
	return func(r *rand.Rand, _ *catalog.GenerateCtx) catalog.Field {
		tenThousandths := 5000 + r.Intn(5001) // #nosec G404
		return catalog.Field{Tag: TagFactor, Value: fmt.Sprintf("0.%04d", tenThousandths)}
	}
}
func poolField() catalog.FieldGenerator {
	return func(r *rand.Rand, _ *catalog.GenerateCtx) catalog.Field {
		return catalog.Field{Tag: TagPool, Value: pickInstrument(r, pickSecurityType(r)).Pool}
	}
}
func trancheField() catalog.FieldGenerator {
	return func(r *rand.Rand, _ *catalog.GenerateCtx) catalog.Field {
		t := pickInstrument(r, pickSecurityType(r)).Tranche
		if t == "" {
			t = "A1" // sensible default for pass-through MBS-style with no formal tranche label
		}
		return catalog.Field{Tag: TagTranche, Value: t}
	}
}
func creditRatingField() catalog.FieldGenerator {
	return func(r *rand.Rand, _ *catalog.GenerateCtx) catalog.Field {
		return catalog.Field{Tag: TagCreditRating, Value: pickInstrument(r, pickSecurityType(r)).Rating}
	}
}
func cfiCodeField() catalog.FieldGenerator {
	return func(r *rand.Rand, _ *catalog.GenerateCtx) catalog.Field {
		return catalog.Field{Tag: TagCFICode, Value: pickInstrument(r, pickSecurityType(r)).CFICode}
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
		hundredths := 80000 + r.Intn(40000) // 80.000 - 120.000 // #nosec G404
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
