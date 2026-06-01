// Package session registers FIX 4.4 session-layer message definitions
// with the catalog Registry. Session messages are asset-agnostic — they
// carry no instrument-specific fields and apply identically across all
// asset categories.
//
// Messages defined here:
//
//	Logon (A), Heartbeat (0), Logout (5), ResendRequest (2),
//	SequenceReset (4), TestRequest (1), Reject (3)
//
// Registration happens in init() at package import time.
package session

import (
	"fmt"
	"math/rand"

	"github.com/observiq/blitz/generator/fix/catalog"
)

// MsgType codes for FIX 4.4 session messages (tag 35 values).
const (
	MsgTypeHeartbeat     = "0"
	MsgTypeTestRequest   = "1"
	MsgTypeResendRequest = "2"
	MsgTypeReject        = "3"
	MsgTypeSequenceReset = "4"
	MsgTypeLogout        = "5"
	MsgTypeLogon         = "A"
)

// Session-layer tags. These live here (not in the catalog package)
// because they are specific to session messages and are not used by
// application-layer messages.
const (
	TagEncryptMethod       catalog.Tag = 98
	TagHeartBtInt          catalog.Tag = 108
	TagResetSeqNumFlag     catalog.Tag = 141
	TagDefaultApplVerID    catalog.Tag = 1137
	TagTestReqID           catalog.Tag = 112
	TagText                catalog.Tag = 58
	TagBeginSeqNo          catalog.Tag = 7
	TagEndSeqNo            catalog.Tag = 16
	TagGapFillFlag         catalog.Tag = 123
	TagNewSeqNo            catalog.Tag = 36
	TagRefSeqNum           catalog.Tag = 45
	TagRefTagID            catalog.Tag = 371
	TagRefMsgType          catalog.Tag = 372
	TagSessionRejectReason catalog.Tag = 373
)

// defaultHeartBtInt is the value emitted in Logon's HeartBtInt (108).
// The session layer uses this to schedule outgoing Heartbeats and
// detect stale links.
const defaultHeartBtInt = 30

func init() {
	registerAll()
}

// Reregister wipes the catalog Registry and re-runs this package's
// registrations. Intended ONLY for tests that need a clean Registry
// state without losing this package's contributions.
func Reregister() {
	catalog.ResetForTest()
	registerAll()
}

// registerAll registers every session-layer message definition this
// package owns. Called from both init() (at import time) and from
// Reregister (in tests).
func registerAll() {
	defs := []catalog.MessageDefinition{
		{
			Version:       catalog.V44,
			MsgType:       MsgTypeLogon,
			AssetCategory: catalog.AssetCategoryUnknown,
			Fields: []catalog.FieldGenerator{
				catalog.LiteralField(TagEncryptMethod, "0"),
				catalog.IntField(TagHeartBtInt, defaultHeartBtInt),
			},
		},
		{
			Version:       catalog.V44,
			MsgType:       MsgTypeHeartbeat,
			AssetCategory: catalog.AssetCategoryUnknown,
			// Heartbeats with no TestReqID context have an empty body.
			// Heartbeats that respond to a TestRequest carry TestReqID
			// 112; the StateTracker handles that case at emit time, not
			// the static definition.
			Fields: nil,
		},
		{
			Version:       catalog.V44,
			MsgType:       MsgTypeLogout,
			AssetCategory: catalog.AssetCategoryUnknown,
			Fields: []catalog.FieldGenerator{
				catalog.LiteralField(TagText, "Normal logout"),
			},
		},
		{
			Version:       catalog.V44,
			MsgType:       MsgTypeResendRequest,
			AssetCategory: catalog.AssetCategoryUnknown,
			// BeginSeqNo and EndSeqNo are session-state-driven; the
			// definition carries placeholder generators that the
			// StateTracker overrides at emit time.
			Fields: []catalog.FieldGenerator{
				catalog.IntField(TagBeginSeqNo, 1),
				catalog.IntField(TagEndSeqNo, 0),
			},
		},
		{
			Version:       catalog.V44,
			MsgType:       MsgTypeSequenceReset,
			AssetCategory: catalog.AssetCategoryUnknown,
			Fields: []catalog.FieldGenerator{
				catalog.LiteralField(TagGapFillFlag, "N"),
				catalog.IntField(TagNewSeqNo, 1),
			},
		},
		{
			Version:       catalog.V44,
			MsgType:       MsgTypeTestRequest,
			AssetCategory: catalog.AssetCategoryUnknown,
			Fields: []catalog.FieldGenerator{
				testReqIDField(),
			},
		},
		{
			Version:       catalog.V44,
			MsgType:       MsgTypeReject,
			AssetCategory: catalog.AssetCategoryUnknown,
			// RefTagID (371) and RefMsgType (372) are conditionally required
			// per FIX 4.4: RefTagID only when SessionRejectReason refers to a
			// specific tag (0=Invalid Tag, 1=Required Tag Missing, 2=Tag Not
			// Defined, 3, 4, 5, 6, 13, 14, 16, 17, 18 — tag-scoped reasons).
			// We emit SessionRejectReason=99 (Other) → neither tag is required.
			Fields: []catalog.FieldGenerator{
				catalog.IntField(TagRefSeqNum, 0),
				catalog.IntField(TagSessionRejectReason, 99),
				catalog.LiteralField(TagText, "Generic session-layer reject"),
			},
		},
	}
	for _, def := range defs {
		catalog.Register(def)
	}
}

// testReqIDField returns a FieldGenerator that emits a deterministic
// TestReqID derived from the supplied *rand.Rand. Format is "TR-NNNNN"
// for human-readable wire output without collisions inside a session.
func testReqIDField() catalog.FieldGenerator {
	return func(r *rand.Rand, _ *catalog.GenerateCtx) catalog.Field {
		n := r.Intn(100000) // #nosec G404 -- seeded *rand.Rand is intentional
		return catalog.Field{
			Tag:   TagTestReqID,
			Value: "TR-" + zeroPad5(n),
		}
	}
}

// zeroPad5 formats n as a 5-digit zero-padded decimal string.
func zeroPad5(n int) string {
	if n < 0 {
		n = -n
	}
	return fmt.Sprintf("%05d", n)
}
