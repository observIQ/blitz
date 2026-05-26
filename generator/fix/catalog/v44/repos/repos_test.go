package repos

import (
	"math/rand"
	"testing"

	"github.com/observiq/blitz/generator/fix/catalog"
	"github.com/observiq/blitz/generator/fix/catalog/v44/app"
)

func reregisterAll() {
	app.Reregister()
	registerAll()
}

func TestEveryRepoSecurityTypeHasRepos(t *testing.T) {
	for _, st := range catalog.SecurityTypesByCategory(catalog.AssetCategoryRepos) {
		if len(reposByType[st]) == 0 {
			t.Errorf("Repos %q has no entries", st)
		}
	}
}

func TestAllReposMessagesRegistered(t *testing.T) {
	reregisterAll()
	for _, mt := range []string{
		app.MsgTypeNewOrderSingle, app.MsgTypeExecutionReport,
		app.MsgTypeOrderCancelRequest, app.MsgTypeOrderCancelReplaceRequest,
		app.MsgTypeOrderStatusRequest,
	} {
		if catalog.Get(catalog.MessageKey{Version: catalog.V44, MsgType: mt, AssetCategory: catalog.AssetCategoryRepos}) == nil {
			t.Errorf("Repos %s not registered", mt)
		}
	}
}

func TestReposInstrumentBlock(t *testing.T) {
	reregisterAll()
	def := catalog.Get(catalog.MessageKey{Version: catalog.V44, MsgType: app.MsgTypeNewOrderSingle, AssetCategory: catalog.AssetCategoryRepos})
	tags := buildTagMap(def.Fields, rand.New(rand.NewSource(1)))
	required := []catalog.Tag{
		app.TagSymbol, app.TagSecurityType, TagSecurityID, TagSecurityIDSrc,
		TagRepoCollateralSecurityType, TagTermType, TagCouponRate,
		TagSettlDate, TagSettlDate2, TagMarginRatio, TagCFICode,
	}
	for _, want := range required {
		if _, ok := tags[want]; !ok {
			t.Errorf("Repos NewOrderSingle missing tag %d", want)
		}
	}
}

func TestReposSecurityTypeStaysInCategory(t *testing.T) {
	reregisterAll()
	def := catalog.Get(catalog.MessageKey{Version: catalog.V44, MsgType: app.MsgTypeNewOrderSingle, AssetCategory: catalog.AssetCategoryRepos})
	for seed := int64(1); seed <= 100; seed++ {
		tags := buildTagMap(def.Fields, rand.New(rand.NewSource(seed)))
		st := catalog.SecurityType(tags[app.TagSecurityType])
		if st.Category() != catalog.AssetCategoryRepos {
			t.Errorf("seed=%d: SecurityType %q has category %s, want Repos", seed, st, st.Category())
		}
	}
}

func TestReposTermTypeIsValid(t *testing.T) {
	reregisterAll()
	def := catalog.Get(catalog.MessageKey{Version: catalog.V44, MsgType: app.MsgTypeNewOrderSingle, AssetCategory: catalog.AssetCategoryRepos})
	for seed := int64(1); seed <= 50; seed++ {
		tags := buildTagMap(def.Fields, rand.New(rand.NewSource(seed)))
		tt := tags[TagTermType]
		if tt != TermOvernight && tt != TermTerm && tt != TermFlexible {
			t.Errorf("seed=%d: TermType %q invalid", seed, tt)
		}
	}
}

// TestReposInstrumentBlockCoherent asserts that every instrument-block
// field in a single message references the same row in reposByType —
// Symbol/SecurityID/CollatSecType/TermType/Rate/Haircut/CFICode all
// come from one repo, not desynchronized independent picks.
func TestReposInstrumentBlockCoherent(t *testing.T) {
	reregisterAll()
	def := catalog.Get(catalog.MessageKey{Version: catalog.V44, MsgType: app.MsgTypeNewOrderSingle, AssetCategory: catalog.AssetCategoryRepos})
	for seed := int64(1); seed <= 50; seed++ {
		tags := buildTagMap(def.Fields, rand.New(rand.NewSource(seed)))
		st := catalog.SecurityType(tags[app.TagSecurityType])
		rows := reposByType[st]
		var match *repo
		for i := range rows {
			if rows[i].Symbol == tags[app.TagSymbol] {
				match = &rows[i]
				break
			}
		}
		if match == nil {
			t.Errorf("seed=%d: Symbol %q does not belong to SecurityType %q", seed, tags[app.TagSymbol], st)
			continue
		}
		if got := tags[TagSecurityID]; got != match.ID {
			t.Errorf("seed=%d st=%s: SecurityID=%q desyncs (want %q)", seed, st, got, match.ID)
		}
		if got := tags[TagRepoCollateralSecurityType]; got != match.CollatSecType {
			t.Errorf("seed=%d st=%s: CollateralType=%q desyncs (want %q)", seed, st, got, match.CollatSecType)
		}
		if got := tags[TagTermType]; got != match.Term {
			t.Errorf("seed=%d st=%s: TermType=%q desyncs (want %q)", seed, st, got, match.Term)
		}
		if got := tags[TagCouponRate]; got != match.Rate {
			t.Errorf("seed=%d st=%s: Rate=%q desyncs (want %q)", seed, st, got, match.Rate)
		}
		if got := tags[TagMarginRatio]; got != match.Haircut {
			t.Errorf("seed=%d st=%s: Haircut=%q desyncs (want %q)", seed, st, got, match.Haircut)
		}
		if got := tags[TagCFICode]; got != match.CFICode {
			t.Errorf("seed=%d st=%s: CFICode=%q desyncs (want %q)", seed, st, got, match.CFICode)
		}
	}
}

// TestReposSettlDate2DerivedFromTermType pins that SettlDate2 reflects
// TermType: Overnight closes T+1, Term/Flexible close ~1 month out.
func TestReposSettlDate2DerivedFromTermType(t *testing.T) {
	reregisterAll()
	def := catalog.Get(catalog.MessageKey{Version: catalog.V44, MsgType: app.MsgTypeNewOrderSingle, AssetCategory: catalog.AssetCategoryRepos})
	sawOvernight, sawTerm := false, false
	for seed := int64(1); seed <= 200; seed++ {
		tags := buildTagMap(def.Fields, rand.New(rand.NewSource(seed)))
		sd1, sd2 := tags[TagSettlDate], tags[TagSettlDate2]
		if sd1 == "" || sd2 == "" {
			t.Fatalf("seed=%d: empty SettlDate (sd1=%q sd2=%q)", seed, sd1, sd2)
		}
		switch tags[TagTermType] {
		case TermOvernight:
			sawOvernight = true
			if sd1 != "20260527" || sd2 != "20260528" {
				t.Errorf("seed=%d Overnight: SettlDate=%q SettlDate2=%q, want 20260527 / 20260528", seed, sd1, sd2)
			}
		case TermTerm, TermFlexible:
			sawTerm = true
			if sd1 != "20260527" || sd2 != "20260627" {
				t.Errorf("seed=%d %s: SettlDate=%q SettlDate2=%q, want 20260527 / 20260627", seed, tags[TagTermType], sd1, sd2)
			}
		}
	}
	if !sawOvernight || !sawTerm {
		t.Fatalf("test did not exercise both Overnight and Term/Flexible paths (overnight=%v, term=%v)", sawOvernight, sawTerm)
	}
}

func TestReposDeterminismFromSeed(t *testing.T) {
	reregisterAll()
	def := catalog.Get(catalog.MessageKey{Version: catalog.V44, MsgType: app.MsgTypeNewOrderSingle, AssetCategory: catalog.AssetCategoryRepos})
	a := buildTagMap(def.Fields, rand.New(rand.NewSource(42)))
	b := buildTagMap(def.Fields, rand.New(rand.NewSource(42)))
	for k, va := range a {
		if vb, ok := b[k]; !ok || va != vb {
			t.Errorf("seed-42 disagreement at tag %d: %q vs %q", k, va, vb)
		}
	}
}

func buildTagMap(gens []catalog.FieldGenerator, r *rand.Rand) map[catalog.Tag]string {
	out := make(map[catalog.Tag]string, len(gens))
	ctx := &catalog.GenerateCtx{Version: catalog.V44, AssetCategory: catalog.AssetCategoryRepos}
	for _, g := range gens {
		f := g(r, ctx)
		out[f.Tag] = f.Value
	}
	return out
}
