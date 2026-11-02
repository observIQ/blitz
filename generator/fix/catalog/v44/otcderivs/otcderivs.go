// Package otcderivs registers FIX 4.4 MessageDefinitions for the OTC
// Derivatives asset category — IRS (interest rate swap), CDS (credit
// default swap), BSWAP (basis), VARSWAP (variance), TRSWAP (total
// return), XCS (cross-currency swap).
//
// The full FIX 4.4 swap modeling involves repeating groups (NoLegs,
// NoUnderlyings) — v1 emits a flat representative subset so the wire
// shape is observability-platform compatible. Per-leg/per-underlying
// repeating groups wait for repeating-group state tracking, which does
// not exist yet.
package otcderivs

import (
	"fmt"
	"math/rand"
	"time"

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
	TagUnderlyingSecIDSrc   catalog.Tag = 305
	TagUnderlyingCFICode    catalog.Tag = 463
)

const (
	SecurityIDSourceCUSIP = "1"
	SecurityIDSourceISIN  = "4"
)

type swap struct {
	Symbol     string
	ID         string
	IDSource   string
	FixedRate  string
	Underlying string
	// UnderlyingID is the underlier's identifier, empty when it has none
	// (a rate index, an FX pair, a CDS reference entity: FIX 4.4 has no RED
	// SecurityIDSource); UnderlyingIDSource is its tag 305 value.
	UnderlyingID       string
	UnderlyingIDSource string
	CFICode            string
	TenorMonths        int // the tenor the name carries (5Y = 60)
}

var swapsByType = map[catalog.SecurityType][]swap{
	catalog.SecIRS: {
		{"USD-IRS-5Y", "EZXE13YG4T34", SecurityIDSourceISIN, "4.500", "USD-SOFR", "", "", "SRACSP", 60},
		{"EUR-IRS-10Y", "EZ79WZYA7KJ1", SecurityIDSourceISIN, "3.250", "EUR-ESTR", "", "", "SRACSP", 120},
		{"GBP-IRS-2Y", "EZAXYJ8T0ME0", SecurityIDSourceISIN, "4.875", "GBP-SONIA", "", "", "SRACSP", 24},
	},
	catalog.SecCDS: {
		{"CDS-XYZ-5Y", "EZ2LEFZHOGF8", SecurityIDSourceISIN, "1.000", "XYZ Corp", "", "", "SRRCSP", 60},
		{"CDS-CDX-IG", "EZI1XEIKK050", SecurityIDSourceISIN, "1.250", "CDX IG Index", "", "", "SRRCSP", 60},
		{"CDS-iTraxx", "EZEANY6QX6G6", SecurityIDSourceISIN, "1.000", "iTraxx Main", "", "", "SRRCSP", 60},
	},
	catalog.SecBSWAP: {
		{"USD-BASIS-SOFR-FF", "EZPDKBZO9OL0", SecurityIDSourceISIN, "0.150", "USD-SOFR / USD-FEDFUNDS", "", "", "SRBCSP", 60},
		{"EUR-BASIS-6M-3M", "EZXMF12MV350", SecurityIDSourceISIN, "0.100", "EUR-EURIBOR-6M / EUR-EURIBOR-3M", "", "", "SRBCSP", 60},
	},
	catalog.SecVARSWAP: {
		{"SPX-VAR-1Y", "EZIJH8K63J87", SecurityIDSourceISIN, "20.000", "SPX", "", "", "SRVCSP", 12},
		{"VIX-VAR-3M", "EZKBGD048AN3", SecurityIDSourceISIN, "30.000", "VIX", "", "", "SRVCSP", 3},
	},
	catalog.SecTRSWAP: {
		{"TRS-SPX-1Y", "EZ2GSTG22XW9", SecurityIDSourceISIN, "0.500", "SPX", "", "", "SRTCSP", 12},
		{"TRS-AAPL-1Y", "EZDPQPUQOLJ7", SecurityIDSourceISIN, "0.300", "AAPL", "037833100", SecurityIDSourceCUSIP, "SRTCSP", 12},
	},
	catalog.SecXCS: {
		{"XCS-USDEUR-5Y", "EZ6T2BW59UL3", SecurityIDSourceISIN, "0.250", "USD/EUR", "", "", "SRXCSP", 60},
		{"XCS-USDJPY-10Y", "EZ8D1CJM8WL9", SecurityIDSourceISIN, "0.150", "USD/JPY", "", "", "SRXCSP", 120},
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
		underlyingSymbolField(), underlyingSecurityIDField(), underlyingSecurityIDSrcField(),
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
		return swap{Symbol: "OTC-FALLBACK", ID: "EZ0KTK2MYTO4", IDSource: SecurityIDSourceISIN, FixedRate: "0.000", Underlying: "X", CFICode: "SRACSP"}
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
	return func(r *rand.Rand, ctx *catalog.GenerateCtx) catalog.Field {
		// A swap starts at spot (T+2; T+0 for GBP SONIA, which starts the
		// day it trades) and matures its named tenor later. Standard CDS
		// instead follow the ISDA semi-annual roll to 20 Jun or 20 Dec.
		p := pickedSwap(r, ctx)
		if p.SecType == catalog.SecCDS {
			return catalog.Field{Tag: TagMaturityDate, Value: cdsMaturity(ctx.TradeDate(), p.Swap.TenorMonths).Format("20060102")}
		}
		spot := catalog.AddBusinessDays(ctx.TradeDate(), 2)
		if p.Swap.Underlying == "GBP-SONIA" {
			spot = catalog.NextBusinessDay(ctx.TradeDate())
		}
		return catalog.Field{Tag: TagMaturityDate, Value: catalog.AddTenor(spot, p.Swap.TenorMonths, 0).Format("20060102")}
	}
}

func underlyingSymbolField() catalog.FieldGenerator {
	return func(r *rand.Rand, ctx *catalog.GenerateCtx) catalog.Field {
		return catalog.Field{Tag: TagUnderlyingSymbol, Value: pickedSwap(r, ctx).Swap.Underlying}
	}
}
func underlyingSecurityIDField() catalog.FieldGenerator {
	return func(r *rand.Rand, ctx *catalog.GenerateCtx) catalog.Field {
		if id := pickedSwap(r, ctx).Swap.UnderlyingID; id != "" {
			return catalog.Field{Tag: TagUnderlyingSecurityID, Value: id}
		}
		return catalog.Field{} // no identifier: EncodeFields skips the tag
	}
}
func underlyingSecurityIDSrcField() catalog.FieldGenerator {
	return func(r *rand.Rand, ctx *catalog.GenerateCtx) catalog.Field {
		if src := pickedSwap(r, ctx).Swap.UnderlyingIDSource; src != "" {
			return catalog.Field{Tag: TagUnderlyingSecIDSrc, Value: src}
		}
		return catalog.Field{}
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
	return func(r *rand.Rand, ctx *catalog.GenerateCtx) catalog.Field {
		return catalog.Field{Tag: app.TagOrderQty, Value: catalog.OrderQty(r, ctx, drawQty)}
	}
}

func drawQty(r *rand.Rand) string {
	// Notional in $1MM increments, $1MM - $500MM.
	mm := 1 + r.Intn(500) // #nosec G404
	return fmt.Sprintf("%d000000", mm)
}
func leavesQty() catalog.FieldGenerator {
	return func(r *rand.Rand, ctx *catalog.GenerateCtx) catalog.Field {
		return catalog.Field{Tag: app.TagLeavesQty, Value: catalog.OrderQty(r, ctx, drawQty)}
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

// cdsMaturity is a standard CDS contract's scheduled termination date under
// the ISDA semi-annual roll (effective December 2015): contracts traded from
// Mar 20 to Sep 19 mature on Jun 20, and from Sep 20 to Mar 19 on Dec 20, of
// the roll year plus the tenor. The roll year of a January-to-March-19 trade
// is the previous year, whose December roll it belongs to.
func cdsMaturity(trade time.Time, tenorMonths int) time.Time {
	y := trade.Year()
	mar20 := time.Date(y, time.March, 20, 0, 0, 0, 0, time.UTC)
	sep20 := time.Date(y, time.September, 20, 0, 0, 0, 0, time.UTC)
	years := tenorMonths / 12
	switch {
	case trade.Before(mar20):
		return time.Date(y-1+years, time.December, 20, 0, 0, 0, 0, time.UTC)
	case trade.Before(sep20):
		return time.Date(y+years, time.June, 20, 0, 0, 0, 0, time.UTC)
	default:
		return time.Date(y+years, time.December, 20, 0, 0, 0, 0, time.UTC)
	}
}
