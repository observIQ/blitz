// Package equities registers FIX 4.4 MessageDefinitions for the
// Equities asset category — every SecurityType value mapped to
// AssetCategoryEquities (CS, PFD, ETF, MF, ADR, WAR, RGT).
//
// Each MessageDefinition produced here overrides the asset-agnostic
// skeleton registered by the app subpackage (AssetCategoryUnknown)
// with Equities-specific Instrument component fields: Symbol drawn
// from realistic ticker tables, SecurityType (167), SecurityID +
// SecurityIDSource (22+48) for CUSIP, and CFICode (461) per ISO
// 10962.
//
// All randomness sources from the supplied *rand.Rand; no
// time.Now() calls or package-global RNG. Tests lock in
// determinism-from-seed.
package equities

import (
	"fmt"
	"math/rand"

	"github.com/observiq/blitz/generator/fix/catalog"
	"github.com/observiq/blitz/generator/fix/catalog/v44/app"
)

// Equities-specific tag numbers (in addition to those defined in the
// app subpackage).
const (
	TagSecurityID       catalog.Tag = 48
	TagSecurityIDSource catalog.Tag = 22
	TagCFICode          catalog.Tag = 461
	TagAccount          catalog.Tag = 1
	TagAccountType      catalog.Tag = 581
)

// SecurityIDSource values (tag 22).
const (
	SecurityIDSourceCUSIP   = "1" // North American CUSIP
	SecurityIDSourceSEDOL   = "2" // UK SEDOL
	SecurityIDSourceISIN    = "4" // ISO ISIN
	SecurityIDSourceRIC     = "5" // Reuters RIC
	SecurityIDSourceBBGLOBL = "A" // Bloomberg GlobalID
)

// AccountType values (tag 581).
const (
	AccountTypeCash   = "1" // Account is carried on customer side of books, cash
	AccountTypeMargin = "2" // Account is carried on customer side of books, margin
)

// instrument is one row in the per-SecurityType instrument table.
// Symbol is the ticker (or vendor-equivalent); CFICode is the ISO 10962
// classification of financial instruments code; ID is a representative
// security identifier (CUSIP / ISIN / etc.) in its native format.
type instrument struct {
	Symbol   string
	CFICode  string
	ID       string
	IDSource string
}

// equityInstruments is the source-of-truth table mapping each
// AssetCategoryEquities SecurityType to a list of realistic
// instruments. Symbols and IDs are illustrative; CFICodes are
// ISO-10962-correct.
//
// Sized small (5-10 entries per type) for v1; can grow without
// affecting wire compatibility.
var equityInstruments = map[catalog.SecurityType][]instrument{
	catalog.SecCS: {
		{"AAPL", "ESVUFR", "037833100", SecurityIDSourceCUSIP},
		{"MSFT", "ESVUFR", "594918104", SecurityIDSourceCUSIP},
		{"GOOGL", "ESVUFN", "02079K305", SecurityIDSourceCUSIP},
		{"AMZN", "ESVUFR", "023135106", SecurityIDSourceCUSIP},
		{"JPM", "ESVUFR", "46625H100", SecurityIDSourceCUSIP},
		{"JNJ", "ESVUFR", "478160104", SecurityIDSourceCUSIP},
		{"V", "ESVUFR", "92826C839", SecurityIDSourceCUSIP},
		{"MA", "ESVUFR", "57636Q104", SecurityIDSourceCUSIP},
	},
	catalog.SecPFD: {
		// CFI: E (equity) P (preferred) ...
		{"BAC.PB", "EPVUFR", "060505EM4", SecurityIDSourceCUSIP},
		{"JPM.PD", "EPVUFR", "46625H803", SecurityIDSourceCUSIP},
		{"GS.PA", "EPVUFR", "38141GFE7", SecurityIDSourceCUSIP},
		{"WFC.PL", "EPVUFR", "94986RAH4", SecurityIDSourceCUSIP},
	},
	catalog.SecETF: {
		// CFI: E (equity) U (unit) ...
		{"SPY", "EUOPNR", "78462F103", SecurityIDSourceCUSIP},
		{"IVV", "EUOPNR", "464287200", SecurityIDSourceCUSIP},
		{"VOO", "EUOPNR", "922908363", SecurityIDSourceCUSIP},
		{"QQQ", "EUOPNR", "46090E103", SecurityIDSourceCUSIP},
		{"DIA", "EUOPNR", "73935A104", SecurityIDSourceCUSIP},
		{"IWM", "EUOPNR", "464287655", SecurityIDSourceCUSIP},
	},
	catalog.SecMF: {
		// CFI: E (equity) U (unit) — mutual funds reuse equity-unit CFI
		{"VFINX", "EUOPNR", "922908793", SecurityIDSourceCUSIP},
		{"FXAIX", "EUOPNR", "315911750", SecurityIDSourceCUSIP},
		{"VTSAX", "EUOPNR", "922908769", SecurityIDSourceCUSIP},
		{"SWPPX", "EUOPNR", "808509400", SecurityIDSourceCUSIP},
	},
	catalog.SecADR: {
		// CFI: E (equity) D (depositary receipt)
		{"BABA", "EDVUFR", "01609W102", SecurityIDSourceCUSIP},
		{"TSM", "EDVUFR", "874039100", SecurityIDSourceCUSIP},
		{"NVS", "EDVUFR", "66987V109", SecurityIDSourceCUSIP},
		{"BP", "EDVUFR", "055622104", SecurityIDSourceCUSIP},
	},
	catalog.SecWAR: {
		// CFI: R (right/warrant) W (warrant)
		{"SOFI.WS", "RWXXXX", "83406F309", SecurityIDSourceCUSIP},
		{"BBAI.WS", "RWXXXX", "08862E207", SecurityIDSourceCUSIP},
	},
	catalog.SecRGT: {
		// CFI: R (right/warrant) S (subscription right)
		{"XYZ.R", "RSXXXX", "98432N109", SecurityIDSourceCUSIP},
		{"ABC.RT", "RSXXXX", "01234R107", SecurityIDSourceCUSIP},
	},
}

func init() {
	registerAll()
}

// Reregister wipes the catalog Registry and re-runs this package's
// registrations. Intended ONLY for tests.
func Reregister() {
	catalog.ResetForTest()
	registerAll()
}

// registerAll registers the FIX 4.4 application MessageDefinitions for
// the Equities category. There is one (Version, MsgType,
// AssetCategoryEquities) entry per supported MsgType.
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
			AssetCategory: catalog.AssetCategoryEquities,
			Fields:        fieldsFor(mt),
		})
	}
}

// fieldsFor returns the FieldGenerator chain for the given MsgType,
// specialized to the Equities asset category.
//
// The Instrument component block (Symbol, SecurityID,
// SecurityIDSource, SecurityType, CFICode) is emitted as five
// independent generators that each pick from the equityInstruments
// table. v1 accepts that the picks are independent, so a single
// message's tag-55 may not correspond to the same instrument as its
// tag-48 / tag-167 / tag-461 — wire-WELL-FORMED but cross-tag
// inconsistent. Tightening to per-message coherence is PR #16's
// (StateTracker) job.
func fieldsFor(msgType string) []catalog.FieldGenerator {
	switch msgType {
	case app.MsgTypeNewOrderSingle:
		return concat(
			[]catalog.FieldGenerator{clOrdID(), account(), accountType()},
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

// concat flattens multiple FieldGenerator slices into one. Used to
// inline the Instrument component block into each MessageDefinition.
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

// instrumentBlock returns the five Instrument-component generators in
// FIX wire order: Symbol (55), SecurityID (48), SecurityIDSource (22),
// SecurityType (167), CFICode (461). Each picks independently from
// equityInstruments — see fieldsFor comment for the v1 cross-tag-
// inconsistency note.
func instrumentBlock() []catalog.FieldGenerator {
	return []catalog.FieldGenerator{
		symbolField(),
		securityIDField(),
		securityIDSourceField(),
		securityTypeField(),
		cfiCodeField(),
	}
}

func symbolField() catalog.FieldGenerator {
	return func(r *rand.Rand, _ *catalog.GenerateCtx) catalog.Field {
		inst := pickInstrument(r, pickSecurityType(r))
		return catalog.Field{Tag: app.TagSymbol, Value: inst.Symbol}
	}
}

func securityIDField() catalog.FieldGenerator {
	return func(r *rand.Rand, _ *catalog.GenerateCtx) catalog.Field {
		inst := pickInstrument(r, pickSecurityType(r))
		return catalog.Field{Tag: TagSecurityID, Value: inst.ID}
	}
}

func securityIDSourceField() catalog.FieldGenerator {
	return func(r *rand.Rand, _ *catalog.GenerateCtx) catalog.Field {
		inst := pickInstrument(r, pickSecurityType(r))
		return catalog.Field{Tag: TagSecurityIDSource, Value: inst.IDSource}
	}
}

func securityTypeField() catalog.FieldGenerator {
	return func(r *rand.Rand, _ *catalog.GenerateCtx) catalog.Field {
		st := pickSecurityType(r)
		return catalog.Field{Tag: app.TagSecurityType, Value: string(st)}
	}
}

func cfiCodeField() catalog.FieldGenerator {
	return func(r *rand.Rand, _ *catalog.GenerateCtx) catalog.Field {
		inst := pickInstrument(r, pickSecurityType(r))
		return catalog.Field{Tag: TagCFICode, Value: inst.CFICode}
	}
}

func pickSecurityType(r *rand.Rand) catalog.SecurityType {
	types := []catalog.SecurityType{
		catalog.SecCS, catalog.SecPFD, catalog.SecETF, catalog.SecMF,
		catalog.SecADR, catalog.SecWAR, catalog.SecRGT,
	}
	return types[r.Intn(len(types))] // #nosec G404 -- seeded *rand.Rand
}

func pickInstrument(r *rand.Rand, st catalog.SecurityType) instrument {
	insts := equityInstruments[st]
	if len(insts) == 0 {
		return instrument{Symbol: "UNKNOWN", CFICode: "ESVUFR", ID: "000000000", IDSource: SecurityIDSourceCUSIP}
	}
	return insts[r.Intn(len(insts))] // #nosec G404 -- seeded *rand.Rand
}

// --- per-field generators ---

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

func account() catalog.FieldGenerator {
	accounts := []string{"ACCT-RETAIL-001", "ACCT-INST-001", "ACCT-PROP-001"}
	return func(r *rand.Rand, _ *catalog.GenerateCtx) catalog.Field {
		return catalog.Field{
			Tag:   TagAccount,
			Value: accounts[r.Intn(len(accounts))], // #nosec G404
		}
	}
}

func accountType() catalog.FieldGenerator {
	types := []string{AccountTypeCash, AccountTypeMargin}
	return func(r *rand.Rand, _ *catalog.GenerateCtx) catalog.Field {
		return catalog.Field{
			Tag:   TagAccountType,
			Value: types[r.Intn(len(types))], // #nosec G404
		}
	}
}

func side() catalog.FieldGenerator {
	choices := []string{app.SideBuy, app.SideSell, app.SideSellShort}
	return func(r *rand.Rand, _ *catalog.GenerateCtx) catalog.Field {
		return catalog.Field{Tag: app.TagSide, Value: choices[r.Intn(len(choices))]} // #nosec G404
	}
}

func ordType() catalog.FieldGenerator {
	choices := []string{
		app.OrdTypeMarket, app.OrdTypeLimit, app.OrdTypeStop, app.OrdTypeStopLimit,
	}
	return func(r *rand.Rand, _ *catalog.GenerateCtx) catalog.Field {
		return catalog.Field{Tag: app.TagOrdType, Value: choices[r.Intn(len(choices))]} // #nosec G404
	}
}

func tif() catalog.FieldGenerator {
	choices := []string{app.TIFDay, app.TIFGTC, app.TIFIOC, app.TIFFOK}
	return func(r *rand.Rand, _ *catalog.GenerateCtx) catalog.Field {
		return catalog.Field{Tag: app.TagTimeInForce, Value: choices[r.Intn(len(choices))]} // #nosec G404
	}
}

func orderQty() catalog.FieldGenerator {
	return func(r *rand.Rand, _ *catalog.GenerateCtx) catalog.Field {
		// Realistic equity order sizing: 100-10,000 shares in round lots.
		qty := 100 * (1 + r.Intn(100)) // #nosec G404
		return catalog.Field{Tag: app.TagOrderQty, Value: fmt.Sprintf("%d", qty)}
	}
}

func leavesQty() catalog.FieldGenerator {
	return func(r *rand.Rand, _ *catalog.GenerateCtx) catalog.Field {
		qty := 100 * (1 + r.Intn(100)) // #nosec G404
		return catalog.Field{Tag: app.TagLeavesQty, Value: fmt.Sprintf("%d", qty)}
	}
}

func price() catalog.FieldGenerator {
	return func(r *rand.Rand, _ *catalog.GenerateCtx) catalog.Field {
		// Realistic equity price: $1.00 - $1,000.00, 2dp.
		cents := 100 + r.Intn(99900) // #nosec G404
		return catalog.Field{
			Tag:   app.TagPrice,
			Value: fmt.Sprintf("%d.%02d", cents/100, cents%100),
		}
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
