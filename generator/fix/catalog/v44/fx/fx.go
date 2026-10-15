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
	"strconv"
	"time"

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
	SecurityIDSourceISO4217 = "6" // ISO Currency Code (the pair in 48)
)

// pair is one tradable currency pair as blitz emits it. CCY1/CCY2 are ISO
// 4217 codes. SpotLag is the spot settlement lag in business days (T+1 for
// USD/CAD, T+2 otherwise). TenorMonths/TenorDays is an outright, swap, or
// NDF tenor after spot (1M = one calendar month, 1W = seven days). NDFEligible
// marks a non-deliverable pair, which settles in USD.
type pair struct {
	CCY1, CCY2  string
	SpotLag     int
	TenorMonths int
	TenorDays   int
	NDFEligible bool
}

// fxPairs is the source-of-truth table mapping each FX SecurityType to
// representative currency pairs.
var fxPairs = map[catalog.SecurityType][]pair{
	catalog.SecFOR: {
		{CCY1: "EUR", CCY2: "USD", SpotLag: 2},
		{CCY1: "GBP", CCY2: "USD", SpotLag: 2},
		{CCY1: "USD", CCY2: "JPY", SpotLag: 2},
		{CCY1: "USD", CCY2: "CHF", SpotLag: 2},
		{CCY1: "AUD", CCY2: "USD", SpotLag: 2},
		{CCY1: "USD", CCY2: "CAD", SpotLag: 1},
		{CCY1: "NZD", CCY2: "USD", SpotLag: 2},
		{CCY1: "EUR", CCY2: "GBP", SpotLag: 2},
		{CCY1: "EUR", CCY2: "JPY", SpotLag: 2},
		{CCY1: "USD", CCY2: "MXN", SpotLag: 2},
	},
	catalog.SecFXFWD: {
		{CCY1: "EUR", CCY2: "USD", SpotLag: 2, TenorMonths: 1},
		{CCY1: "GBP", CCY2: "USD", SpotLag: 2, TenorMonths: 3},
		{CCY1: "USD", CCY2: "JPY", SpotLag: 2, TenorMonths: 1},
		{CCY1: "AUD", CCY2: "USD", SpotLag: 2, TenorMonths: 3},
		{CCY1: "USD", CCY2: "CAD", SpotLag: 1, TenorMonths: 1},
	},
	catalog.SecFXSWAP: {
		{CCY1: "EUR", CCY2: "USD", SpotLag: 2, TenorDays: 7},
		{CCY1: "GBP", CCY2: "USD", SpotLag: 2, TenorMonths: 1},
		{CCY1: "USD", CCY2: "JPY", SpotLag: 2, TenorDays: 7},
	},
	catalog.SecFXNDF: {
		// NDFs trade in pairs where one side is non-convertible offshore.
		{CCY1: "USD", CCY2: "BRL", SpotLag: 2, TenorMonths: 1, NDFEligible: true},
		{CCY1: "USD", CCY2: "INR", SpotLag: 2, TenorMonths: 1, NDFEligible: true},
		{CCY1: "USD", CCY2: "CNY", SpotLag: 2, TenorMonths: 1, NDFEligible: true},
		{CCY1: "USD", CCY2: "KRW", SpotLag: 2, TenorMonths: 1, NDFEligible: true},
		{CCY1: "USD", CCY2: "TWD", SpotLag: 2, TenorMonths: 1, NDFEligible: true},
		{CCY1: "USD", CCY2: "RUB", SpotLag: 2, TenorMonths: 1, NDFEligible: true},
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
// Currency (15), SettlCurrency (120). Every field reads the one
// instrument picked for the message (pickedInstrument), so the block is
// internally coherent and SettlDate can follow the pair's convention.
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
	return func(r *rand.Rand, ctx *catalog.GenerateCtx) catalog.Field {
		p := pickedInstrument(r, ctx).Pair
		return catalog.Field{Tag: app.TagSymbol, Value: p.CCY1 + "/" + p.CCY2}
	}
}

func securityIDField() catalog.FieldGenerator {
	return func(r *rand.Rand, ctx *catalog.GenerateCtx) catalog.Field {
		p := pickedInstrument(r, ctx).Pair
		return catalog.Field{Tag: TagSecurityID, Value: p.CCY1 + p.CCY2}
	}
}

func securityIDSourceField() catalog.FieldGenerator {
	return catalog.LiteralField(TagSecurityIDSrc, SecurityIDSourceISO4217)
}

func securityTypeField() catalog.FieldGenerator {
	return func(r *rand.Rand, ctx *catalog.GenerateCtx) catalog.Field {
		return catalog.Field{Tag: app.TagSecurityType, Value: string(pickedInstrument(r, ctx).SecType)}
	}
}

// instrumentKey is the GenerateCtx.Memo key for the message's instrument.
type instrumentKey struct{}

// instrument is the SecurityType and pair one message trades.
type instrument struct {
	SecType catalog.SecurityType
	Pair    pair
}

// pickedInstrument picks the message's instrument once and memoizes it,
// so every field of the message describes the same trade.
func pickedInstrument(r *rand.Rand, ctx *catalog.GenerateCtx) instrument {
	if ctx.Memo == nil {
		ctx.Memo = map[any]any{}
	}
	if v, ok := ctx.Memo[instrumentKey{}]; ok {
		return v.(instrument)
	}
	st := pickSecurityType(r)
	in := instrument{SecType: st, Pair: pickPair(r, st)}
	ctx.Memo[instrumentKey{}] = in
	return in
}

func currencyField() catalog.FieldGenerator {
	return func(r *rand.Rand, ctx *catalog.GenerateCtx) catalog.Field {
		p := pickedInstrument(r, ctx).Pair
		return catalog.Field{Tag: TagCurrency, Value: p.CCY1}
	}
}

func settlCurrencyField() catalog.FieldGenerator {
	return func(r *rand.Rand, ctx *catalog.GenerateCtx) catalog.Field {
		p := pickedInstrument(r, ctx).Pair
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
		return pair{CCY1: "EUR", CCY2: "USD", SpotLag: 2}
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
	return func(r *rand.Rand, ctx *catalog.GenerateCtx) catalog.Field {
		return catalog.Field{Tag: app.TagOrderQty, Value: catalog.OrderQty(r, ctx, drawQty)}
	}
}

func drawQty(r *rand.Rand) string {
	// Realistic FX order size: $1MM - $100MM notional in $1MM ticks.
	mm := 1 + r.Intn(100) // #nosec G404
	return fmt.Sprintf("%d000000", mm)
}

func leavesQty() catalog.FieldGenerator {
	return func(r *rand.Rand, ctx *catalog.GenerateCtx) catalog.Field {
		return catalog.Field{Tag: app.TagLeavesQty, Value: catalog.OrderQty(r, ctx, drawQty)}
	}
}

// quote is a pair's reference mid and its market quoting precision (JPY
// pairs trade to 3 decimals, KRW to 2, most majors to 5).
type quote struct {
	mid      float64
	decimals int
}

// Static reference mids, not live rates; each price jitters within
// +/-1% of its mid. Feed live or configured mids if absolute levels matter.
var quotes = map[string]quote{
	"EURUSD": {1.1200, 5}, "GBPUSD": {1.3100, 5}, "USDJPY": {150.00, 3},
	"USDCHF": {0.8500, 5}, "AUDUSD": {0.6600, 5}, "USDCAD": {1.3800, 5},
	"NZDUSD": {0.6000, 5}, "EURGBP": {0.8550, 5}, "EURJPY": {165.00, 3},
	"USDMXN": {18.800, 4}, "USDBRL": {5.500, 4}, "USDINR": {86.00, 4},
	"USDCNY": {7.150, 4}, "USDKRW": {1400.0, 2}, "USDTWD": {31.50, 3},
	"USDRUB": {88.00, 4},
}

func price() catalog.FieldGenerator {
	return func(r *rand.Rand, ctx *catalog.GenerateCtx) catalog.Field {
		p := pickedInstrument(r, ctx).Pair
		q, ok := quotes[p.CCY1+p.CCY2]
		if !ok {
			// Safety net: every pair has a quote; a pair added without one
			// still prices instead of emitting 0.
			q = quote{mid: 1, decimals: 5}
		}
		px := q.mid * (1 + (r.Float64()*2-1)*0.01) // #nosec G404
		return catalog.Field{Tag: app.TagPrice, Value: strconv.FormatFloat(px, 'f', q.decimals, 64)}
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

// settlementKey is the GenerateCtx.Memo key for the message's settlement.
type settlementKey struct{}

// settlement is a SettlType (63) with the SettlDate (64) it implies.
type settlement struct {
	Type string
	Date time.Time
}

// pickedSettlement picks SettlType and derives SettlDate from the trade
// date, memoized so tags 63 and 64 always agree. Spot (FOR) trades settle
// cash (T+0), next day (T+1), or regular (the pair's spot lag). Forwards,
// swaps, and NDFs settle as SettlType 6 at spot plus the pair's tenor (whole
// calendar months or days), rolled to the next business day.
func pickedSettlement(r *rand.Rand, ctx *catalog.GenerateCtx) settlement {
	if ctx.Memo == nil { // safety net: message fields pick the instrument first, which allocates Memo
		ctx.Memo = map[any]any{}
	}
	if v, ok := ctx.Memo[settlementKey{}]; ok {
		return v.(settlement)
	}
	in := pickedInstrument(r, ctx)
	trade := ctx.TradeDate()
	var st settlement
	if in.SecType == catalog.SecFOR {
		switch []string{SettlTypeSpot, SettlTypeCash, SettlTypeNextDay}[r.Intn(3)] { // #nosec G404
		case SettlTypeCash:
			st = settlement{Type: SettlTypeCash, Date: catalog.NextBusinessDay(trade)}
		case SettlTypeNextDay:
			st = settlement{Type: SettlTypeNextDay, Date: catalog.AddBusinessDays(trade, 1)}
		default:
			st = settlement{Type: SettlTypeSpot, Date: catalog.AddBusinessDays(trade, in.Pair.SpotLag)}
		}
	} else {
		spot := catalog.AddBusinessDays(trade, in.Pair.SpotLag)
		st = settlement{Type: SettlTypeFwd, Date: catalog.AddTenor(spot, in.Pair.TenorMonths, in.Pair.TenorDays)}
	}
	ctx.Memo[settlementKey{}] = st
	return st
}

func settlDate() catalog.FieldGenerator {
	return func(r *rand.Rand, ctx *catalog.GenerateCtx) catalog.Field {
		return catalog.Field{Tag: TagSettlDate, Value: pickedSettlement(r, ctx).Date.Format("20060102")}
	}
}

func settlType() catalog.FieldGenerator {
	return func(r *rand.Rand, ctx *catalog.GenerateCtx) catalog.Field {
		return catalog.Field{Tag: TagSettlType, Value: pickedSettlement(r, ctx).Type}
	}
}
