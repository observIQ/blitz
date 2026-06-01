package options

import (
	"math/rand"
	"strconv"
	"testing"

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
	ctx := &catalog.GenerateCtx{Version: catalog.V44, AssetCategory: catalog.AssetCategoryOptions}
	for _, g := range gens {
		f := g(r, ctx)
		out[f.Tag] = f.Value
	}
	return out
}
