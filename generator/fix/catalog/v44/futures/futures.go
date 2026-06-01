// Package futures registers FIX 4.4 MessageDefinitions for the Futures
// asset category (SecurityType = FUT). Listed-futures Instrument
// component carries MaturityMonthYear (200), ContractMultiplier (231),
// SecurityExchange (207), and CFICode (461) following ISO 10962.
//
// All randomness sources from the supplied *rand.Rand.
package futures

import (
	"fmt"
	"math/rand"

	"github.com/observiq/blitz/generator/fix/catalog"
	"github.com/observiq/blitz/generator/fix/catalog/v44/app"
)

// Futures-specific tag numbers.
const (
	TagMaturityMonthYear  catalog.Tag = 200
	TagContractMultiplier catalog.Tag = 231
	TagSecurityExchange   catalog.Tag = 207
	TagCFICode            catalog.Tag = 461
)

// contract is one row in the futures table.
type contract struct {
	Symbol             string // exchange ticker (e.g. ES, NQ, CL)
	Exchange           string // MIC code (XCME, XCBT, XNYM, XCEC)
	ContractMultiplier int    // points or units per contract
	CFICode            string
	TickSize           string // for documentation; not emitted
}

// futuresContracts lists representative listed-futures contracts
// across asset classes (equity index, energy, metals, rates, ags).
var futuresContracts = []contract{
	{"ES", "XCME", 50, "FFICSX", "0.25"},           // E-mini S&P 500
	{"NQ", "XCME", 20, "FFICSX", "0.25"},           // E-mini Nasdaq-100
	{"YM", "XCBT", 5, "FFICSX", "1"},               // E-mini Dow
	{"CL", "XNYM", 1000, "FFRCSX", "0.01"},         // WTI Crude Oil
	{"NG", "XNYM", 10000, "FFRCSX", "0.001"},       // Natural Gas
	{"GC", "XCEC", 100, "FFMCSX", "0.10"},          // Gold
	{"SI", "XCEC", 5000, "FFMCSX", "0.005"},        // Silver
	{"HG", "XCEC", 25000, "FFMCSX", "0.0005"},      // Copper
	{"ZN", "XCBT", 1000, "FFDCSX", "0.015625"},     // 10-Year T-Note
	{"ZB", "XCBT", 1000, "FFDCSX", "0.03125"},      // 30-Year T-Bond
	{"ZF", "XCBT", 1000, "FFDCSX", "0.0078125"},    // 5-Year T-Note
	{"ZC", "XCBT", 5000, "FFACSX", "0.0025"},       // Corn
	{"ZS", "XCBT", 5000, "FFACSX", "0.0025"},       // Soybeans
	{"ZW", "XCBT", 5000, "FFACSX", "0.0025"},       // Wheat
	{"6E", "XCME", 125000, "FFFCSX", "0.00005"},    // Euro FX futures
	{"6J", "XCME", 12500000, "FFFCSX", "0.000005"}, // Japanese Yen futures
	{"BTC", "XCME", 5, "FFCCSX", "5"},              // Bitcoin futures
	{"VX", "XCBF", 1000, "FFXCSX", "0.05"},         // VIX futures
}

// expiryCodes are the 12 monthly expiration codes used in futures
// "ESM26" style symbology (March = H, June = M, September = U,
// December = Z are the common quarterly cycle).
var expiryCodes = []string{
	"F", "G", "H", "J", "K", "M", "N", "Q", "U", "V", "X", "Z",
}

func init() {
	registerAll()
}

func Reregister() {
	catalog.ResetForTest()
	registerAll()
}

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
			AssetCategory: catalog.AssetCategoryFutures,
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
			[]catalog.FieldGenerator{
				orderID(), clOrdID(), execID(),
				catalog.LiteralField(app.TagExecType, app.ExecTypeNew),
				catalog.LiteralField(app.TagOrdStatus, app.OrdStatusNew),
			},
			instrumentBlock(),
			[]catalog.FieldGenerator{
				side(), orderQty(),
				catalog.IntField(app.TagCumQty, 0),
				leavesQty(),
				catalog.LiteralField(app.TagAvgPx, "0.00"),
			},
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

// instrumentBlock emits the futures Instrument component: Symbol (55),
// SecurityType (167=FUT), MaturityMonthYear (200), ContractMultiplier
// (231), SecurityExchange (207), CFICode (461).
func instrumentBlock() []catalog.FieldGenerator {
	return []catalog.FieldGenerator{
		symbolField(),
		catalog.LiteralField(app.TagSecurityType, string(catalog.SecFUT)),
		maturityMonthYearField(),
		contractMultiplierField(),
		securityExchangeField(),
		cfiCodeField(),
	}
}

func pickContract(r *rand.Rand) contract {
	return futuresContracts[r.Intn(len(futuresContracts))] // #nosec G404
}

func symbolField() catalog.FieldGenerator {
	return func(r *rand.Rand, _ *catalog.GenerateCtx) catalog.Field {
		c := pickContract(r)
		return catalog.Field{Tag: app.TagSymbol, Value: c.Symbol}
	}
}

func maturityMonthYearField() catalog.FieldGenerator {
	return func(r *rand.Rand, _ *catalog.GenerateCtx) catalog.Field {
		// Pick from current + next 11 months; year cycles 2026-2028.
		year := 2026 + r.Intn(3) // #nosec G404
		month := 1 + r.Intn(12)  // #nosec G404
		return catalog.Field{
			Tag:   TagMaturityMonthYear,
			Value: fmt.Sprintf("%d%02d", year, month),
		}
	}
}

func contractMultiplierField() catalog.FieldGenerator {
	return func(r *rand.Rand, _ *catalog.GenerateCtx) catalog.Field {
		c := pickContract(r)
		return catalog.Field{Tag: TagContractMultiplier, Value: fmt.Sprintf("%d", c.ContractMultiplier)}
	}
}

func securityExchangeField() catalog.FieldGenerator {
	return func(r *rand.Rand, _ *catalog.GenerateCtx) catalog.Field {
		c := pickContract(r)
		return catalog.Field{Tag: TagSecurityExchange, Value: c.Exchange}
	}
}

func cfiCodeField() catalog.FieldGenerator {
	return func(r *rand.Rand, _ *catalog.GenerateCtx) catalog.Field {
		c := pickContract(r)
		return catalog.Field{Tag: TagCFICode, Value: c.CFICode}
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
	choices := []string{app.OrdTypeMarket, app.OrdTypeLimit, app.OrdTypeStop, app.OrdTypeStopLimit}
	return func(r *rand.Rand, _ *catalog.GenerateCtx) catalog.Field {
		return catalog.Field{Tag: app.TagOrdType, Value: choices[r.Intn(len(choices))]} // #nosec G404
	}
}

func tif() catalog.FieldGenerator {
	choices := []string{app.TIFDay, app.TIFGTC, app.TIFIOC}
	return func(r *rand.Rand, _ *catalog.GenerateCtx) catalog.Field {
		return catalog.Field{Tag: app.TagTimeInForce, Value: choices[r.Intn(len(choices))]} // #nosec G404
	}
}

func orderQty() catalog.FieldGenerator {
	return func(r *rand.Rand, _ *catalog.GenerateCtx) catalog.Field {
		qty := 1 + r.Intn(1000) // 1-1000 contracts // #nosec G404
		return catalog.Field{Tag: app.TagOrderQty, Value: fmt.Sprintf("%d", qty)}
	}
}

func leavesQty() catalog.FieldGenerator {
	return func(r *rand.Rand, _ *catalog.GenerateCtx) catalog.Field {
		qty := 1 + r.Intn(1000) // #nosec G404
		return catalog.Field{Tag: app.TagLeavesQty, Value: fmt.Sprintf("%d", qty)}
	}
}

func price() catalog.FieldGenerator {
	return func(r *rand.Rand, _ *catalog.GenerateCtx) catalog.Field {
		// Futures prices vary widely. Use a quarter-tick range.
		ticks := 100 + r.Intn(999900) // #nosec G404
		whole := ticks / 100
		frac := ticks % 100
		return catalog.Field{Tag: app.TagPrice, Value: fmt.Sprintf("%d.%02d", whole, frac)}
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
