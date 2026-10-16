package options

import (
	"math/rand"
	"strconv"
	"testing"
	"time"

	"github.com/observiq/blitz/generator/fix/catalog"
	"github.com/observiq/blitz/generator/fix/catalog/v44/app"
)

func reregisterAll() {
	app.Reregister()
	registerAll()
}

func TestUnderlyingsTable(t *testing.T) {
	if len(optionUnderlyings) < 5 {
		t.Errorf("underlyings table too sparse: %d entries", len(optionUnderlyings))
	}
	for i, u := range optionUnderlyings {
		if u.Symbol == "" {
			t.Errorf("row %d: empty Symbol", i)
		}
		if len(u.Exchange) != 4 {
			t.Errorf("row %d (%s): Exchange %q must be 4-char MIC", i, u.Symbol, u.Exchange)
		}
	}
}

// TestIndexOptionsListOnTheirExchange pins the index options to the exchange
// that lists them: NDX trades on Nasdaq PHLX (and its sister Nasdaq
// exchanges), not Cboe; SPX, VIX, and RUT are Cboe's.
func TestIndexOptionsListOnTheirExchange(t *testing.T) {
	want := map[string]string{"SPX": "XCBO", "NDX": "XPHL", "VIX": "XCBO", "RUT": "XCBO"}
	for _, u := range optionUnderlyings {
		if mic, ok := want[u.Symbol]; ok && u.Exchange != mic {
			t.Errorf("%s options list on %s, want %s", u.Symbol, u.Exchange, mic)
		}
	}
}

func TestAllOptionsMessagesRegistered(t *testing.T) {
	reregisterAll()
	for _, mt := range []string{
		app.MsgTypeNewOrderSingle, app.MsgTypeExecutionReport,
		app.MsgTypeOrderCancelRequest, app.MsgTypeOrderCancelReplaceRequest,
		app.MsgTypeOrderStatusRequest,
	} {
		if catalog.Get(catalog.MessageKey{Version: catalog.V44, MsgType: mt, AssetCategory: catalog.AssetCategoryOptions}) == nil {
			t.Errorf("Options %s not registered", mt)
		}
	}
}

func TestOptionsInstrumentBlock(t *testing.T) {
	reregisterAll()
	def := catalog.Get(catalog.MessageKey{Version: catalog.V44, MsgType: app.MsgTypeNewOrderSingle, AssetCategory: catalog.AssetCategoryOptions})
	if def == nil {
		t.Fatal("Options NewOrderSingle not registered")
	}
	tags := buildTagMap(def.Fields, rand.New(rand.NewSource(1)))
	required := []catalog.Tag{
		app.TagSymbol, app.TagSecurityType, TagStrikePrice, TagPutOrCall,
		TagMaturityDate, TagOptAttribute, TagContractMultiplier,
		TagSecurityExchange, TagCFICode,
	}
	for _, want := range required {
		if _, ok := tags[want]; !ok {
			t.Errorf("Options NewOrderSingle missing tag %d", want)
		}
	}
	if tags[app.TagSecurityType] != string(catalog.SecOPT) {
		t.Errorf("Options SecurityType = %q, want %q", tags[app.TagSecurityType], catalog.SecOPT)
	}
}

func TestOptionsPutOrCallIsValid(t *testing.T) {
	reregisterAll()
	def := catalog.Get(catalog.MessageKey{Version: catalog.V44, MsgType: app.MsgTypeNewOrderSingle, AssetCategory: catalog.AssetCategoryOptions})
	for seed := int64(1); seed <= 50; seed++ {
		tags := buildTagMap(def.Fields, rand.New(rand.NewSource(seed)))
		v := tags[TagPutOrCall]
		if v != PutOrCallPut && v != PutOrCallCall {
			t.Errorf("seed=%d: PutOrCall %q not in {%q, %q}", seed, v, PutOrCallPut, PutOrCallCall)
		}
	}
}

func TestOptionsMaturityDateFormat(t *testing.T) {
	reregisterAll()
	def := catalog.Get(catalog.MessageKey{Version: catalog.V44, MsgType: app.MsgTypeNewOrderSingle, AssetCategory: catalog.AssetCategoryOptions})
	for seed := int64(1); seed <= 50; seed++ {
		tags := buildTagMap(def.Fields, rand.New(rand.NewSource(seed)))
		md := tags[TagMaturityDate]
		if len(md) != 8 {
			t.Errorf("seed=%d: MaturityDate %q must be YYYYMMDD (8 chars)", seed, md)
		}
		yr, err := strconv.Atoi(md[:4])
		if err != nil || yr < 2000 || yr > 2100 {
			t.Errorf("seed=%d: invalid year in MaturityDate %q", seed, md)
		}
	}
}

func TestOptionsContractMultiplierIs100(t *testing.T) {
	reregisterAll()
	def := catalog.Get(catalog.MessageKey{Version: catalog.V44, MsgType: app.MsgTypeNewOrderSingle, AssetCategory: catalog.AssetCategoryOptions})
	tags := buildTagMap(def.Fields, rand.New(rand.NewSource(1)))
	if tags[TagContractMultiplier] != "100" {
		t.Errorf("ContractMultiplier = %q, want 100", tags[TagContractMultiplier])
	}
}

func TestOptionsDeterminismFromSeed(t *testing.T) {
	reregisterAll()
	def := catalog.Get(catalog.MessageKey{Version: catalog.V44, MsgType: app.MsgTypeNewOrderSingle, AssetCategory: catalog.AssetCategoryOptions})
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
	ctx := &catalog.GenerateCtx{Version: catalog.V44, AssetCategory: catalog.AssetCategoryOptions, Now: time.Date(2026, 9, 24, 14, 0, 0, 0, time.UTC)}
	for _, g := range gens {
		f := g(r, ctx)
		out[f.Tag] = f.Value
	}
	return out
}

// TestOptionsCFICodeMatchesContract pins that the ISO 10962 CFI code agrees
// with the message's contract: call/put matches PutOrCall, exercise style
// matches OptAttribute, and index options (European, cash-settled) are told
// apart from equity and ETF options (American, physically settled).
func TestOptionsCFICodeMatchesContract(t *testing.T) {
	reregisterAll()
	def := catalog.Get(catalog.MessageKey{Version: catalog.V44, MsgType: app.MsgTypeNewOrderSingle, AssetCategory: catalog.AssetCategoryOptions})
	putCall := map[string]string{PutOrCallPut: "P", PutOrCallCall: "C"}
	index := map[string]bool{"SPX": true, "NDX": true, "VIX": true, "RUT": true}
	for seed := int64(0); seed < 300; seed++ {
		tags := buildTagMap(def.Fields, rand.New(rand.NewSource(seed)))
		want := "O" + putCall[tags[TagPutOrCall]] + "ASPS"
		if index[tags[app.TagSymbol]] {
			want = "O" + putCall[tags[TagPutOrCall]] + "EICS"
		}
		if tags[TagCFICode] != want || tags[TagOptAttribute] != want[2:3] {
			t.Fatalf("seed %d: %s option has CFI %q, OptAttribute %s, PutOrCall %s; want CFI %q",
				seed, tags[app.TagSymbol], tags[TagCFICode], tags[TagOptAttribute], tags[TagPutOrCall], want)
		}
	}
}

// TestOptionsStrikeIsAListedStrike pins that each strike sits on its
// underlying's strike grid, within a band around its reference level, so the
// option names a series the exchange lists.
func TestOptionsStrikeIsAListedStrike(t *testing.T) {
	reregisterAll()
	def := catalog.Get(catalog.MessageKey{Version: catalog.V44, MsgType: app.MsgTypeNewOrderSingle, AssetCategory: catalog.AssetCategoryOptions})
	bySymbol := map[string]underlying{}
	for _, u := range optionUnderlyings {
		bySymbol[u.Symbol] = u
	}
	for seed := int64(1); seed <= 300; seed++ {
		tags := buildTagMap(def.Fields, rand.New(rand.NewSource(seed)))
		u := bySymbol[tags[app.TagSymbol]]
		strike, err := strconv.ParseFloat(tags[TagStrikePrice], 64)
		if err != nil {
			t.Fatalf("seed=%d: StrikePrice %q: %v", seed, tags[TagStrikePrice], err)
		}
		if int(strike)%u.StrikeStep != 0 || strike != float64(int(strike)) {
			t.Errorf("seed=%d: %s strike %v is off its %d grid", seed, u.Symbol, strike, u.StrikeStep)
		}
		if strike < 0.8*u.Level || strike > 1.2*u.Level {
			t.Errorf("seed=%d: %s strike %v is outside 80-120%% of %v", seed, u.Symbol, strike, u.Level)
		}
	}
}
