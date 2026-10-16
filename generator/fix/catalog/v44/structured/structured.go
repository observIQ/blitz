// Package structured registers FIX 4.4 MessageDefinitions for the
// Structured Products asset category — ABS, MBS, TMBS, CMBS, CDO.
package structured

import (
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"time"

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
	// TermYears is the years from issue to (expected) maturity.
	TermYears int
}

var instrumentsByType = map[catalog.SecurityType][]instrument{
	catalog.SecABS: {
		{"CARD-ABS-{Y}A", "000000901", "5.250", "POOL-CARD-{Y}A", "A1", "AAA", "DAPSFR", 5},
		{"AUTO-ABS-{Y}B", "000000919", "4.875", "POOL-AUTO-{Y}B", "A2", "AAA", "DAPSFR", 4},
		{"SLABS-{Y}C", "000000927", "5.625", "POOL-SL-{Y}C", "A1", "AA", "DAPSFR", 20},
	},
	catalog.SecMBS: {
		{"FNMA-30YR-{Y}", "31418A900", "5.500", "POOL-FNMA-{Y}A", "", "AAA", "DAMSFR", 30},
		{"FHLMC-15YR-{Y}", "3132AB904", "4.500", "POOL-FHLMC-{Y}B", "", "AAA", "DAMSFR", 15},
		{"GNMA-30YR-{Y}", "36202A909", "5.250", "POOL-GNMA-{Y}C", "", "AAA", "DAMSFR", 30},
	},
	catalog.SecTMBS: {
		{"TBA-FNMA-30YR-5.5", "01F052904", "5.500", "TBA-FNMA-30Y", "", "AAA", "DAMSFR", 30},
		{"TBA-GNMA-30YR-5.0", "21H050904", "5.000", "TBA-GNMA-30Y", "", "AAA", "DAMSFR", 30},
	},
	catalog.SecCMBS: {
		{"BBCMS-{Y}-C5", "05544R904", "5.250", "POOL-CMBS-{Y}C5", "A2", "AAA", "DAMSFR", 10},
		{"BANK-{Y}-BNK48", "06054G908", "5.500", "POOL-CMBS-{Y}B48", "A3", "AAA", "DAMSFR", 10},
	},
	catalog.SecCDO: {
		{"OAKWOOD-CLO-{Y}-1", "67402J903", "6.500", "POOL-CLO-{Y}-1", "A-1", "AAA", "DBOSFR", 13},
		{"MAGNETITE-CLO-{Y}", "55946T909", "6.250", "POOL-CLO-{Y}-2", "B", "A", "DBOSFR", 13},
		{"VOYA-CLO-{Y}", "928962901", "6.750", "POOL-CLO-{Y}-3", "C", "BBB", "DBOSFR", 13},
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
		return instrument{Symbol: "STRUCT-FALLBACK", Cusip: "000000935", Coupon: "5.000", Rating: "AAA", CFICode: "DAPSFR", TermYears: 10}
	}
	return is[r.Intn(len(is))] // #nosec G404
}

// pickKey is the GenerateCtx.Memo key for the message's picked
// (SecurityType, instrument) pair, so every instrument field describes one row.
type pickKey struct{}

type pickedPair struct {
	SecType         catalog.SecurityType
	Inst            instrument
	Issue, Maturity time.Time
}

// vintage renders a symbol or pool's {Y} as the deal's issue year.
func (p pickedPair) vintage(s string) string {
	return strings.ReplaceAll(s, "{Y}", strconv.Itoa(p.Issue.Year()))
}

func pickedInstrument(r *rand.Rand, ctx *catalog.GenerateCtx) pickedPair {
	if ctx.Memo == nil {
		ctx.Memo = map[any]any{}
	}
	if v, ok := ctx.Memo[pickKey{}]; ok {
		return v.(pickedPair)
	}
	st := pickSecurityType(r)
	p := pickedPair{SecType: st, Inst: pickInstrument(r, st)}
	// Outstanding on the trade date: issue and maturity span the term.
	p.Issue, p.Maturity = catalog.BondDates(r, ctx.TradeDate(), 12*max(p.Inst.TermYears, 1))
	ctx.Memo[pickKey{}] = p
	return p
}

func symbolField() catalog.FieldGenerator {
	return func(r *rand.Rand, ctx *catalog.GenerateCtx) catalog.Field {
		p := pickedInstrument(r, ctx)
		return catalog.Field{Tag: app.TagSymbol, Value: p.vintage(p.Inst.Symbol)}
	}
}
func securityTypeField() catalog.FieldGenerator {
	return func(r *rand.Rand, ctx *catalog.GenerateCtx) catalog.Field {
		return catalog.Field{Tag: app.TagSecurityType, Value: string(pickedInstrument(r, ctx).SecType)}
	}
}
func securityIDField() catalog.FieldGenerator {
	return func(r *rand.Rand, ctx *catalog.GenerateCtx) catalog.Field {
		return catalog.Field{Tag: TagSecurityID, Value: pickedInstrument(r, ctx).Inst.Cusip}
	}
}
func couponRateField() catalog.FieldGenerator {
	return func(r *rand.Rand, ctx *catalog.GenerateCtx) catalog.Field {
		return catalog.Field{Tag: TagCouponRate, Value: pickedInstrument(r, ctx).Inst.Coupon}
	}
}
func maturityDateField() catalog.FieldGenerator {
	return func(r *rand.Rand, ctx *catalog.GenerateCtx) catalog.Field {
		return catalog.Field{Tag: TagMaturityDate, Value: pickedInstrument(r, ctx).Maturity.Format("20060102")}
	}
}
func issueDateField() catalog.FieldGenerator {
	return func(r *rand.Rand, ctx *catalog.GenerateCtx) catalog.Field {
		return catalog.Field{Tag: TagIssueDate, Value: pickedInstrument(r, ctx).Issue.Format("20060102")}
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
	return func(r *rand.Rand, ctx *catalog.GenerateCtx) catalog.Field {
		p := pickedInstrument(r, ctx)
		return catalog.Field{Tag: TagPool, Value: p.vintage(p.Inst.Pool)}
	}
}
func trancheField() catalog.FieldGenerator {
	return func(r *rand.Rand, ctx *catalog.GenerateCtx) catalog.Field {
		t := pickedInstrument(r, ctx).Inst.Tranche
		if t == "" {
			t = "A1" // sensible default for pass-through MBS-style with no formal tranche label
		}
		return catalog.Field{Tag: TagTranche, Value: t}
	}
}
func creditRatingField() catalog.FieldGenerator {
	return func(r *rand.Rand, ctx *catalog.GenerateCtx) catalog.Field {
		return catalog.Field{Tag: TagCreditRating, Value: pickedInstrument(r, ctx).Inst.Rating}
	}
}
func cfiCodeField() catalog.FieldGenerator {
	return func(r *rand.Rand, ctx *catalog.GenerateCtx) catalog.Field {
		return catalog.Field{Tag: TagCFICode, Value: pickedInstrument(r, ctx).Inst.CFICode}
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
