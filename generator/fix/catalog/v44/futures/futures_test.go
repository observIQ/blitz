package futures

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

func TestFuturesContractTable(t *testing.T) {
	if len(futuresContracts) < 5 {
		t.Errorf("futures contract table too sparse: %d entries", len(futuresContracts))
	}
	for i, c := range futuresContracts {
		if c.Symbol == "" {
			t.Errorf("row %d: empty Symbol", i)
		}
		if len(c.Exchange) != 4 {
			t.Errorf("row %d (%s): Exchange %q must be 4-char MIC", i, c.Symbol, c.Exchange)
		}
		if len(c.CFICode) != 6 {
			t.Errorf("row %d (%s): CFICode %q must be 6 chars", i, c.Symbol, c.CFICode)
		}
		if c.ContractMultiplier <= 0 {
			t.Errorf("row %d (%s): non-positive ContractMultiplier %d", i, c.Symbol, c.ContractMultiplier)
		}
	}
}

func TestAllFuturesMessagesRegistered(t *testing.T) {
	reregisterAll()
	for _, mt := range []string{
		app.MsgTypeNewOrderSingle, app.MsgTypeExecutionReport,
		app.MsgTypeOrderCancelRequest, app.MsgTypeOrderCancelReplaceRequest,
		app.MsgTypeOrderStatusRequest,
	} {
		if catalog.Get(catalog.MessageKey{Version: catalog.V44, MsgType: mt, AssetCategory: catalog.AssetCategoryFutures}) == nil {
			t.Errorf("Futures %s not registered", mt)
		}
	}
}

func TestFuturesInstrumentBlock(t *testing.T) {
	reregisterAll()
	def := catalog.Get(catalog.MessageKey{Version: catalog.V44, MsgType: app.MsgTypeNewOrderSingle, AssetCategory: catalog.AssetCategoryFutures})
	if def == nil {
		t.Fatal("Futures NewOrderSingle not registered")
	}
	tags := buildTagMap(def.Fields, rand.New(rand.NewSource(1)))
	required := []catalog.Tag{
		app.TagSymbol, app.TagSecurityType, TagMaturityMonthYear,
		TagContractMultiplier, TagSecurityExchange, TagCFICode,
	}
	for _, want := range required {
		if _, ok := tags[want]; !ok {
			t.Errorf("Futures NewOrderSingle missing tag %d", want)
		}
	}
	if tags[app.TagSecurityType] != string(catalog.SecFUT) {
		t.Errorf("Futures SecurityType = %q, want %q", tags[app.TagSecurityType], catalog.SecFUT)
	}
}

func TestFuturesMaturityMonthYearFormat(t *testing.T) {
	reregisterAll()
	def := catalog.Get(catalog.MessageKey{Version: catalog.V44, MsgType: app.MsgTypeNewOrderSingle, AssetCategory: catalog.AssetCategoryFutures})
	for seed := int64(1); seed <= 50; seed++ {
		tags := buildTagMap(def.Fields, rand.New(rand.NewSource(seed)))
		mmy := tags[TagMaturityMonthYear]
		if len(mmy) != 6 {
			t.Errorf("seed=%d: MaturityMonthYear %q must be YYYYMM (6 chars)", seed, mmy)
		}
		yr, err := strconv.Atoi(mmy[:4])
		if err != nil || yr < 2000 || yr > 2100 {
			t.Errorf("seed=%d: invalid year in %q", seed, mmy)
		}
		mo, err := strconv.Atoi(mmy[4:])
		if err != nil || mo < 1 || mo > 12 {
			t.Errorf("seed=%d: invalid month in %q", seed, mmy)
		}
	}
}

func TestFuturesDeterminismFromSeed(t *testing.T) {
	reregisterAll()
	def := catalog.Get(catalog.MessageKey{Version: catalog.V44, MsgType: app.MsgTypeNewOrderSingle, AssetCategory: catalog.AssetCategoryFutures})
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
	ctx := &catalog.GenerateCtx{Version: catalog.V44, AssetCategory: catalog.AssetCategoryFutures}
	for _, g := range gens {
		f := g(r, ctx)
		out[f.Tag] = f.Value
	}
	return out
}
