package app

import (
	"math/rand"
	"testing"

	"github.com/observiq/blitz/generator/fix/catalog"
)

func TestAllAppMessagesRegistered(t *testing.T) {
	Reregister()
	want := []string{
		MsgTypeNewOrderSingle,
		MsgTypeExecutionReport,
		MsgTypeOrderCancelRequest,
		MsgTypeOrderCancelReplaceRequest,
		MsgTypeOrderStatusRequest,
		MsgTypeBusinessMessageReject,
	}
	for _, mt := range want {
		def := catalog.Get(catalog.MessageKey{
			Version:       catalog.V44,
			MsgType:       mt,
			AssetCategory: catalog.AssetCategoryUnknown,
		})
		if def == nil {
			t.Errorf("MsgType %q not registered", mt)
		}
	}
}

func TestNewOrderSingleHasRequiredFields(t *testing.T) {
	Reregister()
	def := catalog.Get(catalog.MessageKey{
		Version:       catalog.V44,
		MsgType:       MsgTypeNewOrderSingle,
		AssetCategory: catalog.AssetCategoryUnknown,
	})
	if def == nil {
		t.Fatal("NewOrderSingle not registered")
	}

	r := rand.New(rand.NewSource(1))
	tags := buildTagMap(def.Fields, r)

	required := []catalog.Tag{
		TagClOrdID, TagSymbol, TagSide, TagOrdType, TagTimeInForce,
		TagOrderQty, TagPrice, TagTransactTime,
	}
	for _, want := range required {
		if _, ok := tags[want]; !ok {
			t.Errorf("NewOrderSingle missing tag %d", want)
		}
	}
}

func TestExecutionReportHasRequiredFields(t *testing.T) {
	Reregister()
	def := catalog.Get(catalog.MessageKey{
		Version:       catalog.V44,
		MsgType:       MsgTypeExecutionReport,
		AssetCategory: catalog.AssetCategoryUnknown,
	})
	if def == nil {
		t.Fatal("ExecutionReport not registered")
	}

	r := rand.New(rand.NewSource(1))
	tags := buildTagMap(def.Fields, r)

	required := []catalog.Tag{
		TagOrderID, TagClOrdID, TagExecID, TagExecType,
		TagOrdStatus, TagSymbol, TagSide, TagOrderQty,
		TagCumQty, TagLeavesQty, TagAvgPx,
	}
	for _, want := range required {
		if _, ok := tags[want]; !ok {
			t.Errorf("ExecutionReport missing tag %d", want)
		}
	}
}

func TestClOrdIDFormatAndDeterminism(t *testing.T) {
	Reregister()
	def := catalog.Get(catalog.MessageKey{
		Version:       catalog.V44,
		MsgType:       MsgTypeNewOrderSingle,
		AssetCategory: catalog.AssetCategoryUnknown,
	})

	// Same seed → identical ClOrdID.
	a := def.Fields[0](rand.New(rand.NewSource(42)), &catalog.GenerateCtx{})
	b := def.Fields[0](rand.New(rand.NewSource(42)), &catalog.GenerateCtx{})
	if a != b {
		t.Errorf("ClOrdID not deterministic: %+v vs %+v", a, b)
	}
	if a.Tag != TagClOrdID {
		t.Errorf("first generator tag = %d, want %d", a.Tag, TagClOrdID)
	}
	if len(a.Value) != 12 || a.Value[:4] != "BLZ-" {
		t.Errorf("ClOrdID value = %q, want BLZ-NNNNNNNN", a.Value)
	}
}

func TestPriceTwoDecimalPlaces(t *testing.T) {
	Reregister()
	def := catalog.Get(catalog.MessageKey{
		Version:       catalog.V44,
		MsgType:       MsgTypeNewOrderSingle,
		AssetCategory: catalog.AssetCategoryUnknown,
	})

	// Iterate enough seeds to surface any malformed price strings.
	for seed := int64(1); seed <= 50; seed++ {
		tags := buildTagMap(def.Fields, rand.New(rand.NewSource(seed)))
		price, ok := tags[TagPrice]
		if !ok {
			t.Fatalf("seed=%d: NewOrderSingle missing Price", seed)
		}
		// Format is "N.NN" with exactly two trailing digits.
		dotIdx := -1
		for i, c := range price {
			if c == '.' {
				dotIdx = i
				break
			}
		}
		if dotIdx < 0 || len(price)-dotIdx != 3 {
			t.Errorf("seed=%d: Price %q not in N.NN format", seed, price)
		}
	}
}

func TestTransactTimeUsesContextSendingTime(t *testing.T) {
	Reregister()
	def := catalog.Get(catalog.MessageKey{
		Version:       catalog.V44,
		MsgType:       MsgTypeNewOrderSingle,
		AssetCategory: catalog.AssetCategoryUnknown,
	})

	want := "20260526-12:00:00.000"
	ctx := &catalog.GenerateCtx{SendingTime: want}

	// TransactTime is the last field in NewOrderSingle skeleton.
	r := rand.New(rand.NewSource(1))
	var got catalog.Field
	for _, g := range def.Fields {
		got = g(r, ctx)
	}
	if got.Tag != TagTransactTime {
		t.Fatalf("last field tag = %d, want %d", got.Tag, TagTransactTime)
	}
	if got.Value != want {
		t.Errorf("TransactTime = %q, want %q (context SendingTime)", got.Value, want)
	}
}

func TestOrderStatusRequestHasRequiredFields(t *testing.T) {
	Reregister()
	def := catalog.Get(catalog.MessageKey{
		Version:       catalog.V44,
		MsgType:       MsgTypeOrderStatusRequest,
		AssetCategory: catalog.AssetCategoryUnknown,
	})
	if def == nil {
		t.Fatal("OrderStatusRequest not registered")
	}
	r := rand.New(rand.NewSource(1))
	tags := buildTagMap(def.Fields, r)
	for _, want := range []catalog.Tag{TagClOrdID, TagSymbol, TagSide} {
		if _, ok := tags[want]; !ok {
			t.Errorf("OrderStatusRequest missing required tag %d", want)
		}
	}
}

func TestBusinessMessageRejectShape(t *testing.T) {
	Reregister()
	def := catalog.Get(catalog.MessageKey{
		Version:       catalog.V44,
		MsgType:       MsgTypeBusinessMessageReject,
		AssetCategory: catalog.AssetCategoryUnknown,
	})
	if def == nil {
		t.Fatal("BusinessMessageReject not registered")
	}
	r := rand.New(rand.NewSource(1))
	tags := buildTagMap(def.Fields, r)
	for _, want := range []catalog.Tag{TagRefSeqNum, TagRefMsgType, TagBusinessRejectReason, TagText} {
		if _, ok := tags[want]; !ok {
			t.Errorf("BusinessMessageReject missing tag %d", want)
		}
	}
}

// buildTagMap evaluates a slice of FieldGenerators and returns the
// resulting tag→value map. Helper for tests that assert presence.
func buildTagMap(gens []catalog.FieldGenerator, r *rand.Rand) map[catalog.Tag]string {
	out := make(map[catalog.Tag]string, len(gens))
	ctx := &catalog.GenerateCtx{Version: catalog.V44}
	for _, g := range gens {
		f := g(r, ctx)
		out[f.Tag] = f.Value
	}
	return out
}
