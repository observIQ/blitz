// Package otcderivs registers FIX 4.4 MessageDefinitions for the OTC
// Derivatives asset category — IRS (interest rate swap), CDS (credit
// default swap), BSWAP (basis), VARSWAP (variance), TRSWAP (total
// return), XCS (cross-currency swap).
//
// The full FIX 4.4 swap modeling involves repeating groups (NoLegs,
// NoUnderlyings) — v1 emits a flat representative subset so the wire
// shape is observability-platform compatible. Per-leg/per-underlying
// repeating groups are deferred to PR #16 (StateTracker).
package otcderivs

import (
	"fmt"
	"math/rand"

	"github.com/observiq/blitz/generator/fix/catalog"
	"github.com/observiq/blitz/generator/fix/catalog/v44/app"
)

const (
	TagCFICode              catalog.Tag = 461
	TagSecurityID           catalog.Tag = 48
	TagSecurityIDSrc        catalog.Tag = 22
	TagMaturityDate         catalog.Tag = 541
	TagIssueDate            catalog.Tag = 225
	TagCouponRate           catalog.Tag = 223 // Fixed leg rate for swaps
	TagUnderlyingSymbol     catalog.Tag = 311 // Reference entity / underlier
	TagUnderlyingSecurityID catalog.Tag = 309
	TagUnderlyingCFICode    catalog.Tag = 463
	TagDayCount             catalog.Tag = 478 // SettlInstSource (proxy for day-count convention)
	TagSettlSessSubID       catalog.Tag = 717 // Used as restructuring-clause proxy for CDS
)

const (
	SecurityIDSourceCUSIP = "1"
	SecurityIDSourceISIN  = "4"
	SecurityIDSourceRED   = "I" // Markit RED (Reference Entity Database) — common for CDS
)

type swap struct {
	Symbol       string
	ID           string
	IDSource     string
	FixedRate    string
	Underlying   string
	UnderlyingID string
	DayCount     string // ACT360, ACT365, 30/360 etc.
	Restructure  string // "MR", "MM", "CR", "XR" for CDS
	CFICode      string
}

var swapsByType = map[catalog.SecurityType][]swap{
	catalog.SecIRS: {
		{"USD-IRS-5Y", "IRS-USD-5Y-2026", SecurityIDSourceISIN, "4.500", "USD-SOFR", "USD-SOFR-COMPOUND", "ACT360", "", "SRACSP"},
		{"EUR-IRS-10Y", "IRS-EUR-10Y-2026", SecurityIDSourceISIN, "3.250", "EUR-ESTR", "EUR-ESTR-COMPOUND", "ACT360", "", "SRACSP"},
		{"GBP-IRS-2Y", "IRS-GBP-2Y-2026", SecurityIDSourceISIN, "4.875", "GBP-SONIA", "GBP-SONIA-COMPOUND", "ACT365", "", "SRACSP"},
	},
	catalog.SecCDS: {
		{"CDS-XYZ-5Y", "8I000000", SecurityIDSourceRED, "1.000", "XYZ Corp", "8I0000AAA", "ACT360", "MR", "SRRCSP"},
		{"CDS-CDX-IG", "2I000000", SecurityIDSourceRED, "1.250", "CDX IG Index", "2I0000AAA", "ACT360", "MR", "SRRCSP"},
		{"CDS-iTraxx", "4I000000", SecurityIDSourceRED, "1.000", "iTraxx Main", "4I0000AAA", "ACT360", "MM", "SRRCSP"},
	},
	catalog.SecBSWAP: {
		{"USD-BASIS-3M-1M", "BS-USD-3M1M-2026", SecurityIDSourceISIN, "0.150", "USD-LIBOR-3M / USD-LIBOR-1M", "USD-BASIS-SET", "ACT360", "", "SRBCSP"},
		{"EUR-BASIS-6M-3M", "BS-EUR-6M3M-2026", SecurityIDSourceISIN, "0.100", "EUR-EURIBOR-6M / EUR-EURIBOR-3M", "EUR-BASIS-SET", "ACT360", "", "SRBCSP"},
	},
	catalog.SecVARSWAP: {
		{"SPX-VAR-1Y", "VS-SPX-1Y-2026", SecurityIDSourceISIN, "20.000", "SPX", "SPX-INDEX", "ACT365", "", "SRVCSP"},
		{"VIX-VAR-3M", "VS-VIX-3M-2026", SecurityIDSourceISIN, "30.000", "VIX", "VIX-INDEX", "ACT365", "", "SRVCSP"},
	},
	catalog.SecTRSWAP: {
		{"TRS-SPX-1Y", "TR-SPX-1Y-2026", SecurityIDSourceISIN, "0.500", "SPX", "SPX-INDEX", "ACT365", "", "SRTCSP"},
		{"TRS-AAPL-1Y", "TR-AAPL-1Y-2026", SecurityIDSourceISIN, "0.300", "AAPL", "037833100", "ACT365", "", "SRTCSP"},
	},
	catalog.SecXCS: {
		{"XCS-USDEUR-5Y", "XCS-USDEUR-5Y-2026", SecurityIDSourceISIN, "0.250", "USD/EUR", "USDEUR-CROSS", "ACT360", "", "SRXCSP"},
		{"XCS-USDJPY-10Y", "XCS-USDJPY-10Y-2026", SecurityIDSourceISIN, "0.150", "USD/JPY", "USDJPY-CROSS", "ACT365", "", "SRXCSP"},
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
			Version: catalog.V44, MsgType: mt, AssetCategory: catalog.AssetCategoryOTCDerivs,
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
		symbolField(), securityTypeField(), securityIDField(), securityIDSrcField(),
		couponRateField(), maturityDateField(),
		underlyingSymbolField(), underlyingSecurityIDField(),
		dayCountField(), restructuringField(),
		cfiCodeField(),
	}
}

func pickSecurityType(r *rand.Rand) catalog.SecurityType {
	types := []catalog.SecurityType{
		catalog.SecIRS, catalog.SecCDS, catalog.SecBSWAP,
		catalog.SecVARSWAP, catalog.SecTRSWAP, catalog.SecXCS,
	}
	return types[r.Intn(len(types))] // #nosec G404
}

func pickSwap(r *rand.Rand, st catalog.SecurityType) swap {
	ss := swapsByType[st]
	if len(ss) == 0 {
		return swap{Symbol: "OTC-FALLBACK", ID: "OTC-X", IDSource: SecurityIDSourceISIN, FixedRate: "0.000", Underlying: "X", UnderlyingID: "X", DayCount: "ACT360", CFICode: "SRACSP"}
	}
	return ss[r.Intn(len(ss))] // #nosec G404
}

// pickKey is the Memo key for the picked (SecurityType, swap) pair.
// Using a private type guarantees no collisions with other packages
// sharing the same GenerateCtx.Memo map.
type pickKey struct{}

type pickedPair struct {
	SecType catalog.SecurityType
	Swap    swap
}

// pickedSwap returns the (SecurityType, swap) pair for the current
// message, picking once and memoizing the result on ctx.Memo so every
// instrument-block field generator in the same message agrees on a
// single instrument.
func pickedSwap(r *rand.Rand, ctx *catalog.GenerateCtx) pickedPair {
	if ctx.Memo == nil {
		ctx.Memo = map[any]any{}
	}
	if v, ok := ctx.Memo[pickKey{}]; ok {
		return v.(pickedPair)
	}
	st := pickSecurityType(r)
	p := pickedPair{SecType: st, Swap: pickSwap(r, st)}
	ctx.Memo[pickKey{}] = p
	return p
}

func symbolField() catalog.FieldGenerator {
	return func(r *rand.Rand, ctx *catalog.GenerateCtx) catalog.Field {
		return catalog.Field{Tag: app.TagSymbol, Value: pickedSwap(r, ctx).Swap.Symbol}
	}
}
func securityTypeField() catalog.FieldGenerator {
	return func(r *rand.Rand, ctx *catalog.GenerateCtx) catalog.Field {
		return catalog.Field{Tag: app.TagSecurityType, Value: string(pickedSwap(r, ctx).SecType)}
	}
}
func securityIDField() catalog.FieldGenerator {
	return func(r *rand.Rand, ctx *catalog.GenerateCtx) catalog.Field {
		return catalog.Field{Tag: TagSecurityID, Value: pickedSwap(r, ctx).Swap.ID}
	}
}
func securityIDSrcField() catalog.FieldGenerator {
	return func(r *rand.Rand, ctx *catalog.GenerateCtx) catalog.Field {
		return catalog.Field{Tag: TagSecurityIDSrc, Value: pickedSwap(r, ctx).Swap.IDSource}
	}
}
func couponRateField() catalog.FieldGenerator {
	return func(r *rand.Rand, ctx *catalog.GenerateCtx) catalog.Field {
		return catalog.Field{Tag: TagCouponRate, Value: pickedSwap(r, ctx).Swap.FixedRate}
	}
}
func maturityDateField() catalog.FieldGenerator {
	return func(r *rand.Rand, _ *catalog.GenerateCtx) catalog.Field {
		yrs := 1 + r.Intn(30)                                                                              // #nosec G404
		return catalog.Field{Tag: TagMaturityDate, Value: fmt.Sprintf("%d%02d15", 2026+yrs, 1+r.Intn(12))} // #nosec G404
	}
}
func underlyingSymbolField() catalog.FieldGenerator {
	return func(r *rand.Rand, ctx *catalog.GenerateCtx) catalog.Field {
		return catalog.Field{Tag: TagUnderlyingSymbol, Value: pickedSwap(r, ctx).Swap.Underlying}
	}
}
func underlyingSecurityIDField() catalog.FieldGenerator {
	return func(r *rand.Rand, ctx *catalog.GenerateCtx) catalog.Field {
		return catalog.Field{Tag: TagUnderlyingSecurityID, Value: pickedSwap(r, ctx).Swap.UnderlyingID}
	}
}
func dayCountField() catalog.FieldGenerator {
	return func(r *rand.Rand, ctx *catalog.GenerateCtx) catalog.Field {
		return catalog.Field{Tag: TagDayCount, Value: pickedSwap(r, ctx).Swap.DayCount}
	}
}
func restructuringField() catalog.FieldGenerator {
	// Restructuring clauses (tag 717: MR/MM/CR/XR) only apply to CDS.
	// For non-CDS swaps, emit a zero-value Field so EncodeFields skips
	// it — non-CDS swap messages must not carry this tag at all.
	return func(r *rand.Rand, ctx *catalog.GenerateCtx) catalog.Field {
		p := pickedSwap(r, ctx)
		if p.SecType != catalog.SecCDS {
			return catalog.Field{}
		}
		return catalog.Field{Tag: TagSettlSessSubID, Value: p.Swap.Restructure}
	}
}
func cfiCodeField() catalog.FieldGenerator {
	return func(r *rand.Rand, ctx *catalog.GenerateCtx) catalog.Field {
		return catalog.Field{Tag: TagCFICode, Value: pickedSwap(r, ctx).Swap.CFICode}
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
	return catalog.LiteralField(app.TagTimeInForce, app.TIFGTC)
}
func orderQty() catalog.FieldGenerator {
	return func(r *rand.Rand, _ *catalog.GenerateCtx) catalog.Field {
		// Notional in $1MM increments, $1MM - $500MM.
		mm := 1 + r.Intn(500) // #nosec G404
		return catalog.Field{Tag: app.TagOrderQty, Value: fmt.Sprintf("%d000000", mm)}
	}
}
func leavesQty() catalog.FieldGenerator {
	return func(r *rand.Rand, _ *catalog.GenerateCtx) catalog.Field {
		mm := 1 + r.Intn(500) // #nosec G404
		return catalog.Field{Tag: app.TagLeavesQty, Value: fmt.Sprintf("%d000000", mm)}
	}
}
func price() catalog.FieldGenerator {
	return func(r *rand.Rand, ctx *catalog.GenerateCtx) catalog.Field {
		// CDS quotes in basis-point spread (integer bps, typically
		// 50-1000 for IG single-name and indices). Other swaps quote
		// NPV/par-spread as a decimal 0.0000 - 5.0000.
		if pickedSwap(r, ctx).SecType == catalog.SecCDS {
			bps := 50 + r.Intn(950) // 50-999 bps // #nosec G404
			return catalog.Field{Tag: app.TagPrice, Value: fmt.Sprintf("%d", bps)}
		}
		tenThousandths := r.Intn(50000) // #nosec G404
		return catalog.Field{Tag: app.TagPrice, Value: fmt.Sprintf("%d.%04d", tenThousandths/10000, tenThousandths%10000)}
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
