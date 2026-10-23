// Package options registers FIX 4.4 MessageDefinitions for the Options
// asset category (SecurityType = OPT). Listed-options Instrument
// component carries StrikePrice (202), PutOrCall (201), MaturityDate
// (541), OptAttribute (206), ContractMultiplier (231),
// SecurityExchange (207), and CFICode (461).
//
// Multi-leg combos (NoLegs / LegSymbol etc.) need repeating-group state
// tracking, which does not exist yet, so v1 emits single-leg options only.
package options

import (
	"fmt"
	"math"
	"math/rand"
	"slices"
	"time"

	"github.com/observiq/blitz/generator/fix/catalog"
	"github.com/observiq/blitz/generator/fix/catalog/v44/app"
)

// Options-specific tag numbers.
const (
	TagStrikePrice        catalog.Tag = 202
	TagPutOrCall          catalog.Tag = 201
	TagMaturityDate       catalog.Tag = 541
	TagOptAttribute       catalog.Tag = 206
	TagContractMultiplier catalog.Tag = 231
	TagSecurityExchange   catalog.Tag = 207
	TagCFICode            catalog.Tag = 461
)

// PutOrCall (tag 201).
const (
	PutOrCallPut  = "0"
	PutOrCallCall = "1"
)

// OptAttribute values (tag 206) for exercise style.
const (
	OptAttributeAmerican = "A"
	OptAttributeEuropean = "E"
)

// underlying represents one underlier the option tracks.
type underlying struct {
	Symbol   string
	Exchange string // MIC for the listed option's exchange
	Index    bool   // index options: European, cash-settled
	// Level is the underlying's rough price level, and StrikeStep a strike
	// interval the exchange lists across 80-120% of it: $10 for stocks and
	// ETFs ($5 intervals up to $200, $10 above), 50 points for SPX/NDX/RUT,
	// and 5 for VIX (whose minimum interval is 2.50).
	// Fixed levels drift with the market; feed live levels from
	// the environment if strikes must track today's prices.
	Level      float64
	StrikeStep int
}

// optionUnderlyings table — common underlyings for listed US options.
// Exchanges: XCBO=Cboe, XPHL=Nasdaq PHLX.
var optionUnderlyings = []underlying{
	{"AAPL", "XCBO", false, 230, 10},
	{"MSFT", "XCBO", false, 450, 10},
	{"GOOGL", "XCBO", false, 180, 10},
	{"AMZN", "XCBO", false, 210, 10},
	{"TSLA", "XCBO", false, 300, 10},
	{"NVDA", "XCBO", false, 150, 10},
	{"META", "XCBO", false, 650, 10},
	{"SPY", "XCBO", false, 600, 10},  // SPDR S&P 500 ETF options
	{"QQQ", "XCBO", false, 520, 10},  // Nasdaq-100 ETF options
	{"IWM", "XCBO", false, 220, 10},  // Russell 2000 ETF options
	{"SPX", "XCBO", true, 6000, 50},  // S&P 500 index options
	{"NDX", "XPHL", true, 21000, 50}, // Nasdaq-100 index options
	{"VIX", "XCBO", true, 18, 5},     // VIX index options
	{"RUT", "XCBO", true, 2200, 50},  // Russell 2000 index options
}

func init() {
	registerAll()
}

func Reregister() {
	catalog.ResetForTest()
	registerAll()
}

func registerAll() {
	for _, mt := range []string{
		app.MsgTypeNewOrderSingle,
		app.MsgTypeExecutionReport,
		app.MsgTypeOrderCancelRequest,
		app.MsgTypeOrderCancelReplaceRequest,
		app.MsgTypeOrderStatusRequest,
	} {
		catalog.Register(catalog.MessageDefinition{
			Version:       catalog.V44,
			MsgType:       mt,
			AssetCategory: catalog.AssetCategoryOptions,
			Fields:        fieldsFor(mt),
		})
	}
}

func fieldsFor(msgType string) []catalog.FieldGenerator {
	switch msgType {
	case app.MsgTypeNewOrderSingle:
		return concat(
			[]catalog.FieldGenerator{clOrdID()},
			instrumentBlock(),
			[]catalog.FieldGenerator{side(), ordType(), tif(), orderQty(), price(), transactTime()},
		)
	case app.MsgTypeExecutionReport:
		return concat(
			[]catalog.FieldGenerator{
				orderID(), clOrdID(), execID(),
				catalog.LiteralField(app.TagExecType, app.ExecTypeNew),
				catalog.LiteralField(app.TagOrdStatus, app.OrdStatusNew),
			},
			instrumentBlock(),
			[]catalog.FieldGenerator{
				side(), orderQty(),
				catalog.IntField(app.TagCumQty, 0),
				leavesQty(),
				catalog.LiteralField(app.TagAvgPx, "0.00"),
			},
		)
	case app.MsgTypeOrderCancelRequest:
		return concat(
			[]catalog.FieldGenerator{origClOrdID(), clOrdID()},
			instrumentBlock(),
			[]catalog.FieldGenerator{side(), transactTime()},
		)
	case app.MsgTypeOrderCancelReplaceRequest:
		return concat(
			[]catalog.FieldGenerator{origClOrdID(), clOrdID()},
			instrumentBlock(),
			[]catalog.FieldGenerator{side(), transactTime(), orderQty(), price(), ordType()},
		)
	case app.MsgTypeOrderStatusRequest:
		return concat(
			[]catalog.FieldGenerator{origClOrdID()},
			instrumentBlock(),
			[]catalog.FieldGenerator{side()},
		)
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

// instrumentBlock emits the options Instrument component: Symbol (55),
// SecurityType (167=OPT), StrikePrice (202), PutOrCall (201),
// MaturityDate (541), OptAttribute (206), ContractMultiplier (231),
// SecurityExchange (207), CFICode (461).
func instrumentBlock() []catalog.FieldGenerator {
	return []catalog.FieldGenerator{
		symbolField(),
		catalog.LiteralField(app.TagSecurityType, string(catalog.SecOPT)),
		strikePriceField(),
		putOrCallField(),
		maturityDateField(),
		optAttributeField(),
		contractMultiplierField(),
		securityExchangeField(),
		cfiCodeField(),
	}
}

// contract is the message's one option: its underlying and put/call.
type contract struct {
	underlying
	PutOrCall string
}

// OptAttribute is the exercise style: US listed index options are
// European, equity and ETF options American.
func (c contract) OptAttribute() string {
	if c.Index {
		return OptAttributeEuropean
	}
	return OptAttributeAmerican
}

// CFICode is the ISO 10962 code: O, call/put, exercise style, underlying
// (I index, S equity), delivery (C cash, P physical), S standardized.
func (c contract) CFICode() string {
	pc := "C"
	if c.PutOrCall == PutOrCallPut {
		pc = "P"
	}
	if c.Index {
		return "O" + pc + c.OptAttribute() + "ICS"
	}
	return "O" + pc + c.OptAttribute() + "SPS"
}

// contractKey is the GenerateCtx.Memo key for the message's contract.
type contractKey struct{}

// pickedContract picks the message's one contract, memoized so every field
// describes it.
func pickedContract(r *rand.Rand, ctx *catalog.GenerateCtx) contract {
	if ctx.Memo == nil {
		ctx.Memo = map[any]any{}
	}
	if v, ok := ctx.Memo[contractKey{}]; ok {
		return v.(contract)
	}
	choices := []string{PutOrCallPut, PutOrCallCall}
	c := contract{
		underlying: optionUnderlyings[r.Intn(len(optionUnderlyings))], // #nosec G404
		PutOrCall:  choices[r.Intn(len(choices))],                     // #nosec G404
	}
	ctx.Memo[contractKey{}] = c
	return c
}

func symbolField() catalog.FieldGenerator {
	return func(r *rand.Rand, ctx *catalog.GenerateCtx) catalog.Field {
		return catalog.Field{Tag: app.TagSymbol, Value: pickedContract(r, ctx).Symbol}
	}
}

func strikePriceField() catalog.FieldGenerator {
	return func(r *rand.Rand, ctx *catalog.GenerateCtx) catalog.Field {
		u := pickedContract(r, ctx).underlying
		lo := int(math.Ceil(0.8*u.Level/float64(u.StrikeStep))) * u.StrikeStep
		hi := int(math.Floor(1.2*u.Level/float64(u.StrikeStep))) * u.StrikeStep
		strike := lo + u.StrikeStep*r.Intn((hi-lo)/u.StrikeStep+1) // #nosec G404
		return catalog.Field{Tag: TagStrikePrice, Value: fmt.Sprintf("%d.00", strike)}
	}
}

func putOrCallField() catalog.FieldGenerator {
	return func(r *rand.Rand, ctx *catalog.GenerateCtx) catalog.Field {
		return catalog.Field{Tag: TagPutOrCall, Value: pickedContract(r, ctx).PutOrCall}
	}
}

func maturityDateField() catalog.FieldGenerator {
	return func(r *rand.Rand, ctx *catalog.GenerateCtx) catalog.Field {
		listed := listedMaturities(ctx.TradeDate(), pickedContract(r, ctx).underlying)
		exp := listed[r.Intn(len(listed))] // #nosec G404
		return catalog.Field{Tag: TagMaturityDate, Value: exp.Format("20060102")}
	}
}

// listedMaturities is the expirations an underlying's options always list
// after the trade date, so every drawn maturity names a real series:
//   - Stocks and ETFs (OIC): the two near-term months, plus January LEAPS two
//     years out, or three from the Monday of September expiration week, when
//     the next LEAPS year lists.
//   - SPX, NDX, and RUT (Cboe RUT spec): three near-term months plus three
//     from the March quarterly cycle.
//   - VIX (Cboe): the three near-term expirations, each on the Wednesday 30
//     days before the following month's third Friday.
//
// Busier classes list more months; these are the ones every class carries.
func listedMaturities(trade time.Time, u underlying) []time.Time {
	// next is the first n expirations after the trade date that keep accepts,
	// exp(k) being the one k months out.
	next := func(n int, exp func(k int) time.Time, keep func(time.Time) bool) []time.Time {
		var out []time.Time
		for k := 0; len(out) < n; k++ {
			if d := exp(k); d.After(trade) && keep(d) {
				out = append(out, d)
			}
		}
		return out
	}
	monthly := func(k int) time.Time { return catalog.ThirdFriday(trade, k) }
	all := func(time.Time) bool { return true }
	switch {
	case u.Symbol == "VIX":
		return next(3, func(k int) time.Time { return catalog.ThirdFriday(trade, k+1).AddDate(0, 0, -30) }, all)
	case u.Index:
		near := next(3, monthly, all)
		last := near[len(near)-1]
		return append(near, next(3, monthly, func(d time.Time) bool { return d.After(last) && d.Month()%3 == 0 })...)
	}
	listed := next(2, monthly, all)
	years := 2
	september := catalog.ThirdFriday(time.Date(trade.Year(), time.September, 1, 0, 0, 0, 0, time.UTC), 0)
	if !trade.Before(september.AddDate(0, 0, -4)) {
		years = 3
	}
	for y := trade.Year() + 1; y <= trade.Year()+years; y++ {
		if jan := catalog.ThirdFriday(time.Date(y, time.January, 1, 0, 0, 0, 0, time.UTC), 0); !slices.Contains(listed, jan) {
			listed = append(listed, jan)
		}
	}
	return listed
}

func optAttributeField() catalog.FieldGenerator {
	return func(r *rand.Rand, ctx *catalog.GenerateCtx) catalog.Field {
		return catalog.Field{Tag: TagOptAttribute, Value: pickedContract(r, ctx).OptAttribute()}
	}
}

func contractMultiplierField() catalog.FieldGenerator {
	// Standard listed-options contract is 100 shares of the underlier.
	return catalog.LiteralField(TagContractMultiplier, "100")
}

func securityExchangeField() catalog.FieldGenerator {
	return func(r *rand.Rand, ctx *catalog.GenerateCtx) catalog.Field {
		return catalog.Field{Tag: TagSecurityExchange, Value: pickedContract(r, ctx).Exchange}
	}
}

func cfiCodeField() catalog.FieldGenerator {
	return func(r *rand.Rand, ctx *catalog.GenerateCtx) catalog.Field {
		return catalog.Field{Tag: TagCFICode, Value: pickedContract(r, ctx).CFICode()}
	}
}

// --- shared generators ---

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
	choices := []string{app.OrdTypeMarket, app.OrdTypeLimit}
	return func(r *rand.Rand, _ *catalog.GenerateCtx) catalog.Field {
		return catalog.Field{Tag: app.TagOrdType, Value: choices[r.Intn(len(choices))]} // #nosec G404
	}
}

func tif() catalog.FieldGenerator {
	choices := []string{app.TIFDay, app.TIFGTC, app.TIFIOC}
	return func(r *rand.Rand, _ *catalog.GenerateCtx) catalog.Field {
		return catalog.Field{Tag: app.TagTimeInForce, Value: choices[r.Intn(len(choices))]} // #nosec G404
	}
}

func orderQty() catalog.FieldGenerator {
	return func(r *rand.Rand, ctx *catalog.GenerateCtx) catalog.Field {
		return catalog.Field{Tag: app.TagOrderQty, Value: catalog.OrderQty(r, ctx, drawQty)}
	}
}

func drawQty(r *rand.Rand) string {
	// Realistic options: 1-100 contracts.
	qty := 1 + r.Intn(100) // #nosec G404
	return fmt.Sprintf("%d", qty)
}

func leavesQty() catalog.FieldGenerator {
	return func(r *rand.Rand, ctx *catalog.GenerateCtx) catalog.Field {
		return catalog.Field{Tag: app.TagLeavesQty, Value: catalog.OrderQty(r, ctx, drawQty)}
	}
}

func price() catalog.FieldGenerator {
	return func(r *rand.Rand, _ *catalog.GenerateCtx) catalog.Field {
		// Option premium: $0.05 - $50.00.
		cents := 5 + r.Intn(4995) // #nosec G404
		return catalog.Field{Tag: app.TagPrice, Value: fmt.Sprintf("%d.%02d", cents/100, cents%100)}
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
