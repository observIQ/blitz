// Package app registers FIX 4.4 application-layer MessageDefinitions
// with the catalog Registry at AssetCategory=Unknown — the asset-
// agnostic skeleton variant. Per-asset-category packages register
// their own versions of these MsgTypes with category-specific Instrument
// component fields; the asset-agnostic versions here serve as the
// fallback and the test reference.
//
// Messages registered (FIX 4.4):
//
//	NewOrderSingle (D), ExecutionReport (8),
//	OrderCancelRequest (F), OrderCancelReplaceRequest (G),
//	OrderStatusRequest (H), BusinessMessageReject (j)
//
// All field generators here are deterministic-from-seed: every source
// of randomness reads from the supplied *rand.Rand, never from a
// package-global RNG or time.Now().
package app

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/observiq/blitz/generator/fix/catalog"
)

// MsgType codes for FIX 4.4 application-layer messages (tag 35 values).
const (
	MsgTypeNewOrderSingle            = "D"
	MsgTypeExecutionReport           = "8"
	MsgTypeOrderCancelRequest        = "F"
	MsgTypeOrderCancelReplaceRequest = "G"
	MsgTypeOrderStatusRequest        = "H"
	MsgTypeBusinessMessageReject     = "j"
)

// Application-layer tag constants used across all asset categories.
const (
	TagSymbol               catalog.Tag = 55
	TagSide                 catalog.Tag = 54
	TagTransactTime         catalog.Tag = 60
	TagOrderQty             catalog.Tag = 38
	TagPrice                catalog.Tag = 44
	TagOrdType              catalog.Tag = 40
	TagTimeInForce          catalog.Tag = 59
	TagClOrdID              catalog.Tag = 11
	TagOrigClOrdID          catalog.Tag = 41
	TagOrderID              catalog.Tag = 37
	TagExecID               catalog.Tag = 17
	TagExecType             catalog.Tag = 150
	TagOrdStatus            catalog.Tag = 39
	TagCumQty               catalog.Tag = 14
	TagAvgPx                catalog.Tag = 6
	TagLeavesQty            catalog.Tag = 151
	TagLastQty              catalog.Tag = 32
	TagLastPx               catalog.Tag = 31
	TagBusinessRejectReason catalog.Tag = 380
	TagRefMsgType           catalog.Tag = 372
	TagRefSeqNum            catalog.Tag = 45
	TagText                 catalog.Tag = 58
	TagSecurityType         catalog.Tag = 167
)

// Side enum values (tag 54).
const (
	SideBuy             = "1"
	SideSell            = "2"
	SideSellShort       = "5"
	SideSellShortExempt = "6"
)

// OrdType enum values (tag 40).
const (
	OrdTypeMarket    = "1"
	OrdTypeLimit     = "2"
	OrdTypeStop      = "3"
	OrdTypeStopLimit = "4"
)

// TimeInForce enum values (tag 59).
const (
	TIFDay = "0"
	TIFGTC = "1"
	TIFIOC = "3"
	TIFFOK = "4"
)

// ExecType enum values (tag 150).
const (
	ExecTypeNew           = "0"
	ExecTypePartialFill   = "F" // 4.4: trade is "F" (Trade); legacy 4.2 used "1"
	ExecTypeFill          = "F"
	ExecTypeDoneForDay    = "3"
	ExecTypeCanceled      = "4"
	ExecTypeReplaced      = "5"
	ExecTypePendingCancel = "6"
	ExecTypeRejected      = "8"
	ExecTypePendingNew    = "A"
	ExecTypeExpired       = "C"
)

// OrdStatus enum values (tag 39).
const (
	OrdStatusNew             = "0"
	OrdStatusPartiallyFilled = "1"
	OrdStatusFilled          = "2"
	OrdStatusDoneForDay      = "3"
	OrdStatusCanceled        = "4"
	OrdStatusReplaced        = "5"
	OrdStatusPendingCancel   = "6"
	OrdStatusRejected        = "8"
	OrdStatusPendingNew      = "A"
	OrdStatusExpired         = "C"
)

func init() {
	registerAll()
}

// Reregister wipes the catalog Registry and re-runs this package's
// registrations. Intended ONLY for tests.
func Reregister() {
	catalog.ResetForTest()
	registerAll()
}

// registerAll registers the asset-agnostic skeleton MessageDefinitions
// at AssetCategory=Unknown. Per-asset packages override with their own
// category-specific definitions.
func registerAll() {
	defs := []catalog.MessageDefinition{
		newOrderSingleSkeleton(),
		executionReportSkeleton(),
		orderCancelRequestSkeleton(),
		orderCancelReplaceRequestSkeleton(),
		orderStatusRequestSkeleton(),
		businessMessageRejectSkeleton(),
	}
	for _, d := range defs {
		catalog.Register(d)
	}
}

func newOrderSingleSkeleton() catalog.MessageDefinition {
	return catalog.MessageDefinition{
		Version:       catalog.V44,
		MsgType:       MsgTypeNewOrderSingle,
		AssetCategory: catalog.AssetCategoryUnknown,
		Fields: []catalog.FieldGenerator{
			clOrdIDField(),
			symbolField("XYZ"),
			sideField(),
			ordTypeField(),
			tifField(),
			orderQtyField(),
			priceField(),
			transactTimeField(),
		},
	}
}

func executionReportSkeleton() catalog.MessageDefinition {
	return catalog.MessageDefinition{
		Version:       catalog.V44,
		MsgType:       MsgTypeExecutionReport,
		AssetCategory: catalog.AssetCategoryUnknown,
		Fields: []catalog.FieldGenerator{
			orderIDField(),
			clOrdIDField(),
			execIDField(),
			catalog.LiteralField(TagExecType, ExecTypeNew),
			catalog.LiteralField(TagOrdStatus, OrdStatusNew),
			symbolField("XYZ"),
			sideField(),
			orderQtyField(),
			cumQtyField(0),
			leavesQtyField(),
			avgPxField(),
		},
	}
}

func orderCancelRequestSkeleton() catalog.MessageDefinition {
	return catalog.MessageDefinition{
		Version:       catalog.V44,
		MsgType:       MsgTypeOrderCancelRequest,
		AssetCategory: catalog.AssetCategoryUnknown,
		Fields: []catalog.FieldGenerator{
			origClOrdIDField(),
			clOrdIDField(),
			symbolField("XYZ"),
			sideField(),
			transactTimeField(),
		},
	}
}

func orderCancelReplaceRequestSkeleton() catalog.MessageDefinition {
	return catalog.MessageDefinition{
		Version:       catalog.V44,
		MsgType:       MsgTypeOrderCancelReplaceRequest,
		AssetCategory: catalog.AssetCategoryUnknown,
		Fields: []catalog.FieldGenerator{
			origClOrdIDField(),
			clOrdIDField(),
			symbolField("XYZ"),
			sideField(),
			transactTimeField(),
			orderQtyField(),
			priceField(),
			ordTypeField(),
		},
	}
}

func orderStatusRequestSkeleton() catalog.MessageDefinition {
	return catalog.MessageDefinition{
		Version:       catalog.V44,
		MsgType:       MsgTypeOrderStatusRequest,
		AssetCategory: catalog.AssetCategoryUnknown,
		Fields: []catalog.FieldGenerator{
			clOrdIDField(),
			origClOrdIDField(),
			symbolField("XYZ"),
			sideField(),
		},
	}
}

func businessMessageRejectSkeleton() catalog.MessageDefinition {
	return catalog.MessageDefinition{
		Version:       catalog.V44,
		MsgType:       MsgTypeBusinessMessageReject,
		AssetCategory: catalog.AssetCategoryUnknown,
		Fields: []catalog.FieldGenerator{
			catalog.IntField(TagRefSeqNum, 0),
			catalog.LiteralField(TagRefMsgType, "D"),
			catalog.IntField(TagBusinessRejectReason, 0),
			catalog.LiteralField(TagText, "Generic business-layer reject"),
		},
	}
}

// ---------- field generators ----------

// clOrdIDField emits a unique-per-call ClOrdID derived from the RNG.
// Format: "BLZ-NNNNNNNN" — 8-digit zero-padded random. Deterministic
// because the RNG is supplied by the caller.
func clOrdIDField() catalog.FieldGenerator {
	return func(r *rand.Rand, _ *catalog.GenerateCtx) catalog.Field {
		return catalog.Field{
			Tag:   TagClOrdID,
			Value: fmt.Sprintf("BLZ-%08d", r.Intn(100000000)), // #nosec G404 -- seeded *rand.Rand
		}
	}
}

func origClOrdIDField() catalog.FieldGenerator {
	return func(r *rand.Rand, _ *catalog.GenerateCtx) catalog.Field {
		return catalog.Field{
			Tag:   TagOrigClOrdID,
			Value: fmt.Sprintf("BLZ-%08d", r.Intn(100000000)), // #nosec G404 -- seeded *rand.Rand
		}
	}
}

func orderIDField() catalog.FieldGenerator {
	return func(r *rand.Rand, _ *catalog.GenerateCtx) catalog.Field {
		return catalog.Field{
			Tag:   TagOrderID,
			Value: fmt.Sprintf("ORD-%010d", r.Intn(1000000000)), // #nosec G404 -- seeded *rand.Rand
		}
	}
}

func execIDField() catalog.FieldGenerator {
	return func(r *rand.Rand, _ *catalog.GenerateCtx) catalog.Field {
		return catalog.Field{
			Tag:   TagExecID,
			Value: fmt.Sprintf("EXE-%010d", r.Intn(1000000000)), // #nosec G404 -- seeded *rand.Rand
		}
	}
}

func symbolField(literal string) catalog.FieldGenerator {
	return catalog.LiteralField(TagSymbol, literal)
}

func sideField() catalog.FieldGenerator {
	choices := []string{SideBuy, SideSell, SideSellShort}
	return func(r *rand.Rand, _ *catalog.GenerateCtx) catalog.Field {
		return catalog.Field{
			Tag:   TagSide,
			Value: choices[r.Intn(len(choices))], // #nosec G404 -- seeded *rand.Rand
		}
	}
}

func ordTypeField() catalog.FieldGenerator {
	choices := []string{OrdTypeMarket, OrdTypeLimit, OrdTypeStop, OrdTypeStopLimit}
	return func(r *rand.Rand, _ *catalog.GenerateCtx) catalog.Field {
		return catalog.Field{
			Tag:   TagOrdType,
			Value: choices[r.Intn(len(choices))], // #nosec G404 -- seeded *rand.Rand
		}
	}
}

func tifField() catalog.FieldGenerator {
	choices := []string{TIFDay, TIFGTC, TIFIOC, TIFFOK}
	return func(r *rand.Rand, _ *catalog.GenerateCtx) catalog.Field {
		return catalog.Field{
			Tag:   TagTimeInForce,
			Value: choices[r.Intn(len(choices))], // #nosec G404 -- seeded *rand.Rand
		}
	}
}

func orderQtyField() catalog.FieldGenerator {
	return func(r *rand.Rand, _ *catalog.GenerateCtx) catalog.Field {
		// Realistic order quantity: 100 to 10,000 share lots.
		qty := 100 * (1 + r.Intn(100)) // #nosec G404 -- seeded *rand.Rand
		return catalog.Field{Tag: TagOrderQty, Value: fmt.Sprintf("%d", qty)}
	}
}

func priceField() catalog.FieldGenerator {
	return func(r *rand.Rand, _ *catalog.GenerateCtx) catalog.Field {
		// Realistic price: $1.00 to $1000.00 with two decimal places.
		cents := 100 + r.Intn(99900) // #nosec G404 -- seeded *rand.Rand
		dollars := cents / 100
		fraction := cents % 100
		return catalog.Field{Tag: TagPrice, Value: fmt.Sprintf("%d.%02d", dollars, fraction)}
	}
}

func cumQtyField(value int) catalog.FieldGenerator {
	return catalog.IntField(TagCumQty, value)
}

func leavesQtyField() catalog.FieldGenerator {
	return func(r *rand.Rand, _ *catalog.GenerateCtx) catalog.Field {
		qty := 100 * (1 + r.Intn(100)) // #nosec G404 -- seeded *rand.Rand
		return catalog.Field{Tag: TagLeavesQty, Value: fmt.Sprintf("%d", qty)}
	}
}

func avgPxField() catalog.FieldGenerator {
	return func(_ *rand.Rand, _ *catalog.GenerateCtx) catalog.Field {
		// New orders have no average price yet.
		return catalog.Field{Tag: TagAvgPx, Value: "0.00"}
	}
}

func transactTimeField() catalog.FieldGenerator {
	return func(_ *rand.Rand, ctx *catalog.GenerateCtx) catalog.Field {
		// Use the SendingTime from the context if set (so emit-time
		// consistency is preserved across fields in the same message),
		// else fall back to a deterministic zero-time literal. Never
		// call time.Now() inside a field generator — that would break
		// determinism.
		v := ctx.SendingTime
		if v == "" {
			v = time.Unix(0, 0).UTC().Format("20060102-15:04:05.000")
		}
		return catalog.Field{Tag: TagTransactTime, Value: v}
	}
}
