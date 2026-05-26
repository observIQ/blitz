// Package v50sp2 registers FIX 5.0 SP2 application MessageDefinitions
// for all 10 asset categories using the FIXT.1.1 session layer.
//
// 5.0 SP2 differences from 4.4:
//   - BeginString is "FIXT.1.1" (session transport), not "FIX.5.0"
//   - ApplVerID (1128) = "9" must accompany application messages to
//     identify the 5.0 SP2 application layer
//   - DefaultApplVerID (1137) appears in Logon
//   - A few new tags exist (1300, 1301 etc.) but for v1 we focus on
//     the wire-correct ApplVerID injection — most existing tags carry
//     identical semantics
//
// Implementation: mirror every (V44, MsgType, AssetCategory) entry to
// (V50SP2, ..., ...) and prepend an ApplVerID generator on application
// messages (not on session messages, which are owned by the FIXT layer
// and don't carry ApplVerID).
package v50sp2

import (
	"github.com/observiq/blitz/generator/fix/catalog"
	"github.com/observiq/blitz/generator/fix/catalog/v44/app"

	_ "github.com/observiq/blitz/generator/fix/catalog/v44/corpbonds"
	_ "github.com/observiq/blitz/generator/fix/catalog/v44/equities"
	_ "github.com/observiq/blitz/generator/fix/catalog/v44/futures"
	_ "github.com/observiq/blitz/generator/fix/catalog/v44/fx"
	_ "github.com/observiq/blitz/generator/fix/catalog/v44/govbonds"
	_ "github.com/observiq/blitz/generator/fix/catalog/v44/moneymarket"
	_ "github.com/observiq/blitz/generator/fix/catalog/v44/options"
	_ "github.com/observiq/blitz/generator/fix/catalog/v44/otcderivs"
	_ "github.com/observiq/blitz/generator/fix/catalog/v44/repos"
	_ "github.com/observiq/blitz/generator/fix/catalog/v44/session"
	_ "github.com/observiq/blitz/generator/fix/catalog/v44/structured"
)

// FIXT / 5.0 SP2 tag numbers.
const (
	TagApplVerID        catalog.Tag = 1128
	TagDefaultApplVerID catalog.Tag = 1137
	TagCstmApplVerID    catalog.Tag = 1129
)

// ApplVerID values per the FIX spec — "9" identifies FIX 5.0 SP2.
const ApplVerIDFIX50SP2 = "9"

// applicationMsgTypes — the set of MsgTypes that carry ApplVerID under
// FIXT.1.1. Session-layer messages (Logon, Heartbeat, Logout,
// ResendRequest, SequenceReset, TestRequest, Reject) belong to the
// session transport and do NOT include ApplVerID — only application
// messages do.
var applicationMsgTypes = map[string]bool{
	app.MsgTypeNewOrderSingle:            true,
	app.MsgTypeExecutionReport:           true,
	app.MsgTypeOrderCancelRequest:        true,
	app.MsgTypeOrderCancelReplaceRequest: true,
	app.MsgTypeOrderStatusRequest:        true,
	app.MsgTypeBusinessMessageReject:     true,
}

// sessionMsgTypes — session-layer MsgTypes. For 5.0 SP2 these get a
// special Logon override (DefaultApplVerID=9).
var sessionMsgTypes = map[string]bool{
	"0": true, "1": true, "2": true, "3": true, "4": true, "5": true, "A": true,
}

func init() {
	registerAll()
}

func registerAll() {
	for _, def := range catalog.AllDefinitions() {
		if def.Version != catalog.V44 {
			continue
		}
		key := catalog.MessageKey{
			Version:       catalog.V50SP2,
			MsgType:       def.MsgType,
			AssetCategory: def.AssetCategory,
		}
		if catalog.Get(key) != nil {
			continue
		}
		catalog.Register(catalog.MessageDefinition{
			Version:       catalog.V50SP2,
			MsgType:       def.MsgType,
			AssetCategory: def.AssetCategory,
			Fields:        adjustForV50SP2(def.MsgType, def.Fields),
		})
	}
}

// adjustForV50SP2 prepends ApplVerID (1128=9) to application messages
// and inserts DefaultApplVerID (1137=9) into Logon. Other messages
// pass through unchanged.
func adjustForV50SP2(msgType string, src []catalog.FieldGenerator) []catalog.FieldGenerator {
	if applicationMsgTypes[msgType] {
		// Prepend ApplVerID as the first body field.
		out := make([]catalog.FieldGenerator, 0, len(src)+1)
		out = append(out, catalog.LiteralField(TagApplVerID, ApplVerIDFIX50SP2))
		out = append(out, src...)
		return out
	}
	if msgType == "A" { // Logon
		// Append DefaultApplVerID at the end of Logon body.
		out := make([]catalog.FieldGenerator, 0, len(src)+1)
		out = append(out, src...)
		out = append(out, catalog.LiteralField(TagDefaultApplVerID, ApplVerIDFIX50SP2))
		return out
	}
	return src
}
