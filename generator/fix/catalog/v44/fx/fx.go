// Package fx registers FIX 4.4 MessageDefinitions for the FX asset
// category — every SecurityType value mapped to AssetCategoryFX
// (FOR spot, FXFWD forward, FXSWAP, FXNDF non-deliverable forward).
//
// Each MessageDefinition overrides the asset-agnostic skeleton with
// FX-specific Instrument component fields: Symbol as a currency pair
// (e.g. "EUR/USD"), Currency (15) and SettlCurrency (120) for both
// legs of the trade, SettlDate (64), and SettlType (63 / 64) per spot
// vs. forward conventions.
//
// All randomness sources from the supplied *rand.Rand. Determinism-
// from-seed verified by `TestFXDeterminismFromSeed`.
package fx

import (
	"fmt"
	"math/rand"

	"github.com/observiq/blitz/generator/fix/catalog"
	"github.com/observiq/blitz/generator/fix/catalog/v44/app"
)

// FX-specific tag numbers.
const (
	TagCurrency      catalog.Tag = 15
	TagSettlCurrency catalog.Tag = 120
	TagSettlDate     catalog.Tag = 64
	TagSettlType     catalog.Tag = 63
	TagSettlDate2    catalog.Tag = 193
	TagSecurityID    catalog.Tag = 48
	TagSecurityIDSrc catalog.Tag = 22
)

// SettlType values (tag 63).
const (
	SettlTypeSpot    = "0" // Regular / T+2
	SettlTypeCash    = "1" // Same-day
	SettlTypeNextDay = "2" // T+1
	SettlTypeFwd     = "6" // Future / forward
)

// SecurityIDSource values relevant to FX (tag 22).
const (
	SecurityIDSourceBBGTICK = "A" // Bloomberg
	SecurityIDSourceISO4217 = "G" // ISO 4217 currency code pair
)

// pair is one row in the per-SecurityType FX pair table. CCY1 and CCY2
// are ISO 4217 currency codes; SettlT+N is the standard settlement
// horizon in business days; NDF flags pairs that emit as non-
// deliverable in the NDF SecurityType (note: a pair can be NDF-eligible
// even if not its default form — e.g., USDBRL trades NDF offshore).
type pair struct {
	CCY1        string
	CCY2        string
	SettlTN     int // typically 2 for major spot, 1 for USDCAD, 0 same-day for USDCNY
	NDFEligible bool
}

// fxPairs is the source-of-truth table mapping each FX SecurityType to
// representative currency pairs.
var fxPairs = map[catalog.SecurityType][]pair{
	catalog.SecFOR: {
		{"EUR", "USD", 2, false},
		{"GBP", "USD", 2, false},
		{"USD", "JPY", 2, false},
		{"USD", "CHF", 2, false},
		{"AUD", "USD", 2, false},
		{"USD", "CAD", 1, false},
		{"NZD", "USD", 2, false},
		{"EUR", "GBP", 2, false},
		{"EUR", "JPY", 2, false},
		{"USD", "MXN", 2, false},
	},
	catalog.SecFXFWD: {
		{"EUR", "USD", 30, false}, // 1-month forward
		{"GBP", "USD", 90, false}, // 3-month forward
		{"USD", "JPY", 30, false},
		{"AUD", "USD", 90, false},
		{"USD", "CAD", 30, false},
	},
	catalog.SecFXSWAP: {
		{"EUR", "USD", 7, false},  // 1-week swap
		{"GBP", "USD", 30, false}, // 1-month swap
		{"USD", "JPY", 7, false},
	},
	catalog.SecFXNDF: {
		// NDFs trade in pairs where one side is non-convertible offshore.
		{"USD", "BRL", 30, true},
		{"USD", "INR", 30, true},
		{"USD", "CNY", 30, true},
		{"USD", "KRW", 30, true},
		{"USD", "TWD", 30, true},
		{"USD", "RUB", 30, true},
	},
}

func init() {
	registerAll()
}

// Reregister wipes the catalog Registry and re-runs registrations.
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
			AssetCategory: catalog.AssetCategoryFX,
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
			[]catalog.FieldGenerator{
				side(), ordType(), tif(),
				orderQty(), price(), transactTime(),
				settlDate(), settlType(),
			},
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
				catalog.LiteralField(app.TagAvgPx, "0.00000"),
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

// instrumentBlock emits the FX Instrument component: Symbol (55),
// SecurityID (48), SecurityIDSource (22), SecurityType (167),
// Currency (15), SettlCurrency (120). v1 picks each independently
// (see fieldsFor comment for the wire-coherence note); PR #16 pins
// per-message coherence.
func instrumentBlock() []catalog.FieldGenerator {
	return []catalog.FieldGenerator{
		symbolField(),
		securityIDField(),
		securityIDSourceField(),
		securityTypeField(),
		currencyField(),
		settlCurrencyField(),
	}
}

func symbolField() catalog.FieldGenerator {
	return func(r *rand.Rand, _ *catalog.GenerateCtx) catalog.Field {
		p := pickPair(r, pickSecurityType(r))
		return catalog.Field{Tag: app.TagSymbol, Value: p.CCY1 + "/" + p.CCY2}
	}
}

func securityIDField() catalog.FieldGenerator {
	return func(r *rand.Rand, _ *catalog.GenerateCtx) catalog.Field {
		p := pickPair(r, pickSecurityType(r))
		return catalog.Field{Tag: TagSecurityID, Value: p.CCY1 + p.CCY2}
	}
}

func securityIDSourceField() catalog.FieldGenerator {
	return catalog.LiteralField(TagSecurityIDSrc, SecurityIDSourceISO4217)
}

func securityTypeField() catalog.FieldGenerator {
	return func(r *rand.Rand, _ *catalog.GenerateCtx) catalog.Field {
		st := pickSecurityType(r)
		return catalog.Field{Tag: app.TagSecurityType, Value: string(st)}
	}
}

func currencyField() catalog.FieldGenerator {
	return func(r *rand.Rand, _ *catalog.GenerateCtx) catalog.Field {
		p := pickPair(r, pickSecurityType(r))
		return catalog.Field{Tag: TagCurrency, Value: p.CCY1}
	}
}

func settlCurrencyField() catalog.FieldGenerator {
	return func(r *rand.Rand, _ *catalog.GenerateCtx) catalog.Field {
		p := pickPair(r, pickSecurityType(r))
		// For NDF, settlement is in USD by convention.
		settl := p.CCY2
		if p.NDFEligible {
			settl = "USD"
		}
		return catalog.Field{Tag: TagSettlCurrency, Value: settl}
	}
}

func pickSecurityType(r *rand.Rand) catalog.SecurityType {
	types := []catalog.SecurityType{
		catalog.SecFOR, catalog.SecFXFWD, catalog.SecFXSWAP, catalog.SecFXNDF,
	}
	return types[r.Intn(len(types))] // #nosec G404
}

func pickPair(r *rand.Rand, st catalog.SecurityType) pair {
	ps := fxPairs[st]
	if len(ps) == 0 {
		return pair{CCY1: "EUR", CCY2: "USD", SettlTN: 2}
	}
	return ps[r.Intn(len(ps))] // #nosec G404
}

// --- shared generators ---

func clOrdID() catalog.FieldGenerator {
	return func(r *rand.Rand, _ *catalog.GenerateCtx) catalog.Field {
		return catalog.Field{
			Tag:   app.TagClOrdID,
			Value: fmt.Sprintf("BLZ-%08d", r.Intn(100000000)), // #nosec G404
		}
	}
}

func origClOrdID() catalog.FieldGenerator {
	return func(r *rand.Rand, _ *catalog.GenerateCtx) catalog.Field {
		return catalog.Field{
			Tag:   app.TagOrigClOrdID,
			Value: fmt.Sprintf("BLZ-%08d", r.Intn(100000000)), // #nosec G404
		}
	}
}

func orderID() catalog.FieldGenerator {
	return func(r *rand.Rand, _ *catalog.GenerateCtx) catalog.Field {
		return catalog.Field{
			Tag:   app.TagOrderID,
			Value: fmt.Sprintf("ORD-%010d", r.Intn(1000000000)), // #nosec G404
		}
	}
}

func execID() catalog.FieldGenerator {
	return func(r *rand.Rand, _ *catalog.GenerateCtx) catalog.Field {
		return catalog.Field{
			Tag:   app.TagExecID,
			Value: fmt.Sprintf("EXE-%010d", r.Intn(1000000000)), // #nosec G404
		}
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
	choices := []string{app.TIFDay, app.TIFIOC, app.TIFFOK}
	return func(r *rand.Rand, _ *catalog.GenerateCtx) catalog.Field {
		return catalog.Field{Tag: app.TagTimeInForce, Value: choices[r.Intn(len(choices))]} // #nosec G404
	}
}

func orderQty() catalog.FieldGenerator {
	return func(r *rand.Rand, _ *catalog.GenerateCtx) catalog.Field {
		// Realistic FX order size: $1MM - $100MM notional in $1MM ticks.
		mm := 1 + r.Intn(100) // #nosec G404
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
		// FX prices use 5 decimal places (pip + half-pip precision).
		// Range: 0.50000 to 200.00000.
		// Use integer ten-thousandths to avoid float rounding.
		pips := 50000 + r.Intn(19950000) // 5_0000 to 199_9999_5 // #nosec G404
		whole := pips / 100000
		frac := pips % 100000
		return catalog.Field{Tag: app.TagPrice, Value: fmt.Sprintf("%d.%05d", whole, frac)}
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

func settlDate() catalog.FieldGenerator {
	// Deterministic placeholder; StateTracker will populate per-trade
	// settlement date based on T+N from the picked pair in PR #16.
	return catalog.LiteralField(TagSettlDate, "20260601")
}

func settlType() catalog.FieldGenerator {
	choices := []string{SettlTypeSpot, SettlTypeCash, SettlTypeNextDay, SettlTypeFwd}
	return func(r *rand.Rand, _ *catalog.GenerateCtx) catalog.Field {
		return catalog.Field{Tag: TagSettlType, Value: choices[r.Intn(len(choices))]} // #nosec G404
	}
}
