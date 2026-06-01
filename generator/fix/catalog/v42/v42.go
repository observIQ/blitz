// Package v42 registers FIX 4.2 application MessageDefinitions for all
// 10 asset categories by mirroring the 4.4 registrations and applying
// version-specific deltas.
//
// 4.2-specific application-layer deltas captured here:
//   - ExecType (150): 4.2 uses numeric "1" Partial / "2" Fill — 4.4
//     introduced "F" Trade covering both. Wrapped on emit.
//
// Session-layer differences (BeginString = "FIX.4.2", no FIXT.1.1, no
// ApplVerID) are handled at framing time by the Version's BeginString /
// ApplVerID accessors — no per-message adjustment needed here.
//
// Importing this package side-effects the registration of every
// (V42, MsgType, AssetCategory) triple that exists at (V44, *, *).
package v42

import (
	"math/rand"

	"github.com/observiq/blitz/generator/fix/catalog"
	"github.com/observiq/blitz/generator/fix/catalog/v44/app"

	// Bring in every 4.4 per-category and session package so their
	// init() registers the V44 baseline before we mirror it to V42.
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

// FIX 4.2 legacy ExecType values (tag 150). 4.4 collapsed these into
// "F" (Trade); v42 restores the legacy split.
const (
	ExecType42PartialFill = "1"
	ExecType42Fill        = "2"
)

func init() {
	registerAll()
}

// registerAll mirrors every (V44, MsgType, AssetCategory) entry into
// a corresponding V42 entry, applying the ExecType wrapping for
// ExecutionReport.
func registerAll() {
	for _, def := range catalog.AllDefinitions() {
		if def.Version != catalog.V44 {
			continue
		}
		key := catalog.MessageKey{
			Version:       catalog.V42,
			MsgType:       def.MsgType,
			AssetCategory: def.AssetCategory,
		}
		if catalog.Get(key) != nil {
			continue
		}
		catalog.Register(catalog.MessageDefinition{
			Version:       catalog.V42,
			MsgType:       def.MsgType,
			AssetCategory: def.AssetCategory,
			Fields:        adjustForV42(def.MsgType, def.Fields),
		})
	}
}

// adjustForV42 wraps the V44 ExecType generator in ExecutionReports to
// emit the 4.2 numeric value instead of "F". Other generators are
// passed through unchanged.
func adjustForV42(msgType string, src []catalog.FieldGenerator) []catalog.FieldGenerator {
	if msgType != app.MsgTypeExecutionReport {
		return src
	}
	out := make([]catalog.FieldGenerator, len(src))
	for i, g := range src {
		out[i] = wrapExecTypeV42(g)
	}
	return out
}

// wrapExecTypeV42 wraps a FieldGenerator: if the generated Field is
// tag 150 (ExecType) with V44 value "F", rewrite to V42 "2" (Fill).
// All other generators pass through.
func wrapExecTypeV42(g catalog.FieldGenerator) catalog.FieldGenerator {
	return func(r *rand.Rand, ctx *catalog.GenerateCtx) catalog.Field {
		f := g(r, ctx)
		if f.Tag == app.TagExecType && f.Value == app.ExecTypeFill {
			f.Value = ExecType42Fill
		}
		return f
	}
}
