// Package repos registers FIX 4.4 MessageDefinitions for the Repurchase
// Agreement asset category — REPO (standard repo), REVREPO (reverse
// repo), HREPO (hold-in-custody repo).
package repos

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/observiq/blitz/generator/fix/catalog"
	"github.com/observiq/blitz/generator/fix/catalog/v44/app"
)

const (
	TagSettlDate                  catalog.Tag = 64  // Near leg settlement
	TagSettlDate2                 catalog.Tag = 193 // Far leg settlement (repo close)
	TagPriceType                  catalog.Tag = 423
	TagCouponRate                 catalog.Tag = 223 // Repo rate
	TagRepoCollateralSecurityType catalog.Tag = 239
	TagTermType                   catalog.Tag = 829 // 1=Overnight, 2=Term, 3=Flexible
	TagMaturityDate               catalog.Tag = 541
	TagCFICode                    catalog.Tag = 461
	TagSecurityID                 catalog.Tag = 48
	TagSecurityIDSrc              catalog.Tag = 22
	TagMarginRatio                catalog.Tag = 898 // Haircut as ratio (e.g. 1.02 = 2% haircut)
)

const (
	SecurityIDSourceISIN = "4"
)

// TermType (829) values.
const (
	TermOvernight = "1"
	TermTerm      = "2"
	TermFlexible  = "3"
)

// PriceType (423) for repos: "1" = % of par.
const PriceTypePercentOfPar = "1"

type repo struct {
	Symbol        string
	ID            string
	CollatSecType string // The collateral SecurityType (e.g. TBOND, CORP)
	Term          string
	Rate          string
	Haircut       string // Margin ratio: "1.000" = no haircut, "1.020" = 2%
	CFICode       string
}

var reposByType = map[catalog.SecurityType][]repo{
	catalog.SecREPO: {
		// Tri-party repos on government collateral
		{"REPO-UST-ON", "REPO-UST-ON-001", string(catalog.SecTBOND), TermOvernight, "5.250", "1.020", "RPRSXX"},
		{"REPO-UST-1W", "REPO-UST-1W-001", string(catalog.SecTBOND), TermTerm, "5.300", "1.020", "RPRSXX"},
		{"REPO-AGY-ON", "REPO-AGY-ON-001", string(catalog.SecMBS), TermOvernight, "5.350", "1.030", "RPRSXX"},
		{"REPO-CORP-ON", "REPO-CORP-ON-001", string(catalog.SecCORP), TermOvernight, "5.500", "1.050", "RPRSXX"},
	},
	catalog.SecREVREPO: {
		{"REVREPO-UST-ON", "REVREPO-UST-ON-001", string(catalog.SecTBOND), TermOvernight, "5.200", "1.020", "RPRSXX"},
		{"REVREPO-UST-1W", "REVREPO-UST-1W-001", string(catalog.SecTBOND), TermTerm, "5.250", "1.020", "RPRSXX"},
	},
	catalog.SecHREPO: {
		// Hold-in-custody repos — collateral stays in original custodian.
		{"HREPO-UST-2W", "HREPO-UST-2W-001", string(catalog.SecTBOND), TermTerm, "5.275", "1.025", "RPHCXX"},
		{"HREPO-CORP-ON", "HREPO-CORP-ON-001", string(catalog.SecCORP), TermOvernight, "5.475", "1.055", "RPHCXX"},
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
			Version: catalog.V44, MsgType: mt, AssetCategory: catalog.AssetCategoryRepos,
			Fields: fieldsFor(mt),
		})
	}
}

func fieldsFor(msgType string) []catalog.FieldGenerator {
	switch msgType {
	case app.MsgTypeNewOrderSingle:
		return concat([]catalog.FieldGenerator{clOrdID()}, instrumentBlock(),
			[]catalog.FieldGenerator{side(), ordType(), tif(), orderQty(), price(),
				catalog.LiteralField(TagPriceType, PriceTypePercentOfPar), transactTime()})
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
		symbolField(), securityTypeField(), securityIDField(),
		catalog.LiteralField(TagSecurityIDSrc, SecurityIDSourceISIN),
		collateralTypeField(), termTypeField(),
		repoRateField(), settlDateField(), settlDate2Field(),
		marginRatioField(), cfiCodeField(),
	}
}

func pickSecurityType(r *rand.Rand) catalog.SecurityType {
	types := []catalog.SecurityType{catalog.SecREPO, catalog.SecREVREPO, catalog.SecHREPO}
	return types[r.Intn(len(types))] // #nosec G404
}

func pickRepo(r *rand.Rand, st catalog.SecurityType) repo {
	rs := reposByType[st]
	if len(rs) == 0 {
		return repo{Symbol: "REPO-FALLBACK", ID: "REPO-X", CollatSecType: string(catalog.SecTBOND), Term: TermOvernight, Rate: "5.000", Haircut: "1.020", CFICode: "RPRSXX"}
	}
	return rs[r.Intn(len(rs))] // #nosec G404
}

// pickKey is the Memo key for the picked (SecurityType, repo) pair so
// every instrument-block field in one message agrees on a single row.
type pickKey struct{}

type pickedPair struct {
	SecType catalog.SecurityType
	Repo    repo
}

func pickedRepo(r *rand.Rand, ctx *catalog.GenerateCtx) pickedPair {
	if ctx.Memo == nil {
		ctx.Memo = map[any]any{}
	}
	if v, ok := ctx.Memo[pickKey{}]; ok {
		return v.(pickedPair)
	}
	st := pickSecurityType(r)
	p := pickedPair{SecType: st, Repo: pickRepo(r, st)}
	ctx.Memo[pickKey{}] = p
	return p
}

func symbolField() catalog.FieldGenerator {
	return func(r *rand.Rand, ctx *catalog.GenerateCtx) catalog.Field {
		return catalog.Field{Tag: app.TagSymbol, Value: pickedRepo(r, ctx).Repo.Symbol}
	}
}
func securityTypeField() catalog.FieldGenerator {
	return func(r *rand.Rand, ctx *catalog.GenerateCtx) catalog.Field {
		return catalog.Field{Tag: app.TagSecurityType, Value: string(pickedRepo(r, ctx).SecType)}
	}
}
func securityIDField() catalog.FieldGenerator {
	return func(r *rand.Rand, ctx *catalog.GenerateCtx) catalog.Field {
		return catalog.Field{Tag: TagSecurityID, Value: pickedRepo(r, ctx).Repo.ID}
	}
}
func collateralTypeField() catalog.FieldGenerator {
	return func(r *rand.Rand, ctx *catalog.GenerateCtx) catalog.Field {
		return catalog.Field{Tag: TagRepoCollateralSecurityType, Value: pickedRepo(r, ctx).Repo.CollatSecType}
	}
}
func termTypeField() catalog.FieldGenerator {
	return func(r *rand.Rand, ctx *catalog.GenerateCtx) catalog.Field {
		return catalog.Field{Tag: TagTermType, Value: pickedRepo(r, ctx).Repo.Term}
	}
}
func repoRateField() catalog.FieldGenerator {
	return func(r *rand.Rand, ctx *catalog.GenerateCtx) catalog.Field {
		return catalog.Field{Tag: TagCouponRate, Value: pickedRepo(r, ctx).Repo.Rate}
	}
}

// settlBaseDate is the fixed near-leg settlement reference used by the
// emitter — deterministic for reproducible output. SettlDate2 is
// computed relative to this.
var settlBaseDate = time.Date(2026, 5, 27, 0, 0, 0, 0, time.UTC)

func settlDateField() catalog.FieldGenerator {
	return catalog.LiteralField(TagSettlDate, settlBaseDate.Format("20060102"))
}
func settlDate2Field() catalog.FieldGenerator {
	// Far-leg settlement: depends on TermType per FIX 4.4 repo
	// conventions. Overnight closes T+1; standard term ~1 month;
	// flexible ~1 month as a working default.
	return func(r *rand.Rand, ctx *catalog.GenerateCtx) catalog.Field {
		var d time.Time
		switch pickedRepo(r, ctx).Repo.Term {
		case TermOvernight:
			d = settlBaseDate.AddDate(0, 0, 1)
		case TermTerm:
			d = settlBaseDate.AddDate(0, 1, 0)
		default:
			d = settlBaseDate.AddDate(0, 1, 0)
		}
		return catalog.Field{Tag: TagSettlDate2, Value: d.Format("20060102")}
	}
}
func marginRatioField() catalog.FieldGenerator {
	return func(r *rand.Rand, ctx *catalog.GenerateCtx) catalog.Field {
		return catalog.Field{Tag: TagMarginRatio, Value: pickedRepo(r, ctx).Repo.Haircut}
	}
}
func cfiCodeField() catalog.FieldGenerator {
	return func(r *rand.Rand, ctx *catalog.GenerateCtx) catalog.Field {
		return catalog.Field{Tag: TagCFICode, Value: pickedRepo(r, ctx).Repo.CFICode}
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
	return catalog.LiteralField(app.TagTimeInForce, app.TIFDay)
}
func orderQty() catalog.FieldGenerator {
	return func(r *rand.Rand, _ *catalog.GenerateCtx) catalog.Field {
		mm := 1 + r.Intn(1000) // $1MM - $1B // #nosec G404
		return catalog.Field{Tag: app.TagOrderQty, Value: fmt.Sprintf("%d000000", mm)}
	}
}
func leavesQty() catalog.FieldGenerator {
	return func(r *rand.Rand, _ *catalog.GenerateCtx) catalog.Field {
		mm := 1 + r.Intn(1000) // #nosec G404
		return catalog.Field{Tag: app.TagLeavesQty, Value: fmt.Sprintf("%d000000", mm)}
	}
}
func price() catalog.FieldGenerator {
	// Repo "price" is % of par of collateral.
	return func(r *rand.Rand, _ *catalog.GenerateCtx) catalog.Field {
		hundredths := 95000 + r.Intn(10000) // #nosec G404
		return catalog.Field{Tag: app.TagPrice, Value: fmt.Sprintf("%d.%03d", hundredths/1000, hundredths%1000)}
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
