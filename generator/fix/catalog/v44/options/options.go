// Package options registers FIX 4.4 MessageDefinitions for the Options
// asset category (SecurityType = OPT). Listed-options Instrument
// component carries StrikePrice (202), PutOrCall (201), MaturityDate
// (541), OptAttribute (206), ContractMultiplier (231),
// SecurityExchange (207), and CFICode (461).
//
// Multi-leg combos (NoLegs / LegSymbol etc.) are deferred to PR #16's
// StateTracker work — v1 emits single-leg options only.
package options

import (
	"fmt"
	"math/rand"

	"github.com/observiq/blitz/generator/fix/catalog"
	"github.com/observiq/blitz/generator/fix/catalog/v44/app"
)

// Options-specific tag numbers.
const (
	TagStrikePrice        catalog.Tag = 202
	TagPutOrCall          catalog.Tag = 201
	TagMaturityDate       catalog.Tag = 541
	TagOptAttribute       catalog.Tag = 206
	TagContractMultiplier catalog.Tag = 231
	TagSecurityExchange   catalog.Tag = 207
	TagCFICode            catalog.Tag = 461
)

// PutOrCall (tag 201).
const (
	PutOrCallPut  = "0"
	PutOrCallCall = "1"
)

// OptAttribute values (tag 206) for exercise style.
const (
	OptAttributeAmerican = "A"
	OptAttributeEuropean = "E"
)

// underlying represents one underlier the option tracks.
type underlying struct {
	Symbol   string
	Exchange string // MIC for the listed option's exchange
}

// optionUnderlyings table — common underlyings for listed US options.
// Exchanges: XCBO=Cboe, XASE=NYSE American, XNYS=NYSE.
var optionUnderlyings = []underlying{
	{"AAPL", "XCBO"},
	{"MSFT", "XCBO"},
	{"GOOGL", "XCBO"},
	{"AMZN", "XCBO"},
	{"TSLA", "XCBO"},
	{"NVDA", "XCBO"},
	{"META", "XCBO"},
	{"SPY", "XCBO"}, // SPDR S&P 500 ETF options
	{"QQQ", "XCBO"}, // Nasdaq-100 ETF options
	{"IWM", "XCBO"}, // Russell 2000 ETF options
	{"SPX", "XCBO"}, // S&P 500 index options
	{"NDX", "XCBO"}, // Nasdaq-100 index options
	{"VIX", "XCBO"}, // VIX index options
	{"RUT", "XCBO"}, // Russell 2000 index options
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
			AssetCategory: catalog.AssetCategoryOptions,
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

// instrumentBlock emits the options Instrument component: Symbol (55),
// SecurityType (167=OPT), StrikePrice (202), PutOrCall (201),
// MaturityDate (541), OptAttribute (206), ContractMultiplier (231),
// SecurityExchange (207), CFICode (461).
func instrumentBlock() []catalog.FieldGenerator {
	return []catalog.FieldGenerator{
		symbolField(),
		catalog.LiteralField(app.TagSecurityType, string(catalog.SecOPT)),
		strikePriceField(),
		putOrCallField(),
		maturityDateField(),
		optAttributeField(),
		contractMultiplierField(),
		securityExchangeField(),
		cfiCodeField(),
	}
}

func pickUnderlying(r *rand.Rand) underlying {
	return optionUnderlyings[r.Intn(len(optionUnderlyings))] // #nosec G404
}

func symbolField() catalog.FieldGenerator {
	return func(r *rand.Rand, _ *catalog.GenerateCtx) catalog.Field {
		u := pickUnderlying(r)
		return catalog.Field{Tag: app.TagSymbol, Value: u.Symbol}
	}
}

func strikePriceField() catalog.FieldGenerator {
	return func(r *rand.Rand, _ *catalog.GenerateCtx) catalog.Field {
		// Strike between $5 and $1000 in $5 increments.
		strike := 5 * (1 + r.Intn(200)) // #nosec G404
		return catalog.Field{Tag: TagStrikePrice, Value: fmt.Sprintf("%d.00", strike)}
	}
}

func putOrCallField() catalog.FieldGenerator {
	choices := []string{PutOrCallPut, PutOrCallCall}
	return func(r *rand.Rand, _ *catalog.GenerateCtx) catalog.Field {
		return catalog.Field{Tag: TagPutOrCall, Value: choices[r.Intn(len(choices))]} // #nosec G404
	}
}

func maturityDateField() catalog.FieldGenerator {
	return func(r *rand.Rand, _ *catalog.GenerateCtx) catalog.Field {
		year := 2026 + r.Intn(2) // 2026 or 2027 // #nosec G404
		month := 1 + r.Intn(12)  // #nosec G404
		// Use 3rd Friday convention (close-enough day for v1): always 15th-21st.
		day := 15 + r.Intn(7) // #nosec G404
		return catalog.Field{
			Tag:   TagMaturityDate,
			Value: fmt.Sprintf("%d%02d%02d", year, month, day),
		}
	}
}

func optAttributeField() catalog.FieldGenerator {
	// US listed equity options are predominantly American style; SPX
	// and a few index options are European. We weight 90/10.
	return func(r *rand.Rand, _ *catalog.GenerateCtx) catalog.Field {
		if r.Intn(10) == 0 { // #nosec G404
			return catalog.Field{Tag: TagOptAttribute, Value: OptAttributeEuropean}
		}
		return catalog.Field{Tag: TagOptAttribute, Value: OptAttributeAmerican}
	}
}

func contractMultiplierField() catalog.FieldGenerator {
	// Standard listed-options contract is 100 shares of the underlier.
	return catalog.LiteralField(TagContractMultiplier, "100")
}

func securityExchangeField() catalog.FieldGenerator {
	return func(r *rand.Rand, _ *catalog.GenerateCtx) catalog.Field {
		u := pickUnderlying(r)
		return catalog.Field{Tag: TagSecurityExchange, Value: u.Exchange}
	}
}

func cfiCodeField() catalog.FieldGenerator {
	// CFI category O (option), group C/P (call/put) — emit a generic
	// representative; per-message coherence with PutOrCall is PR #16's
	// job.
	choices := []string{
		"OCASPS", // Option / Call / American / Stock / Physical / Standard
		"OPASPS", // Option / Put / American / Stock / Physical / Standard
		"OCESPS", // Option / Call / European / Stock / Physical / Standard
	}
	return func(r *rand.Rand, _ *catalog.GenerateCtx) catalog.Field {
		return catalog.Field{Tag: TagCFICode, Value: choices[r.Intn(len(choices))]} // #nosec G404
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
	choices := []string{app.TIFDay, app.TIFGTC, app.TIFIOC}
	return func(r *rand.Rand, _ *catalog.GenerateCtx) catalog.Field {
		return catalog.Field{Tag: app.TagTimeInForce, Value: choices[r.Intn(len(choices))]} // #nosec G404
	}
}

func orderQty() catalog.FieldGenerator {
	return func(r *rand.Rand, _ *catalog.GenerateCtx) catalog.Field {
		// Realistic options: 1-100 contracts.
		qty := 1 + r.Intn(100) // #nosec G404
		return catalog.Field{Tag: app.TagOrderQty, Value: fmt.Sprintf("%d", qty)}
	}
}

func leavesQty() catalog.FieldGenerator {
	return func(r *rand.Rand, _ *catalog.GenerateCtx) catalog.Field {
		qty := 1 + r.Intn(100) // #nosec G404
		return catalog.Field{Tag: app.TagLeavesQty, Value: fmt.Sprintf("%d", qty)}
	}
}

func price() catalog.FieldGenerator {
	return func(r *rand.Rand, _ *catalog.GenerateCtx) catalog.Field {
		// Option premium: $0.05 - $50.00.
		cents := 5 + r.Intn(4995) // #nosec G404
		return catalog.Field{Tag: app.TagPrice, Value: fmt.Sprintf("%d.%02d", cents/100, cents%100)}
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
