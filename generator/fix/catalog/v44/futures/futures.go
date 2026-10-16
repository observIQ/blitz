// Package futures registers FIX 4.4 MessageDefinitions for the Futures
// asset category (SecurityType = FUT). Listed-futures Instrument
// component carries MaturityMonthYear (200), ContractMultiplier (231),
// SecurityExchange (207), and CFICode (461) following ISO 10962.
//
// All randomness sources from the supplied *rand.Rand.
package futures

import (
	"fmt"
	"math"
	"math/rand"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/observiq/blitz/generator/fix/catalog"
	"github.com/observiq/blitz/generator/fix/catalog/v44/app"
)

// Futures-specific tag numbers.
const (
	TagMaturityMonthYear  catalog.Tag = 200
	TagContractMultiplier catalog.Tag = 231
	TagSecurityExchange   catalog.Tag = 207
	TagCFICode            catalog.Tag = 461
)

// contract is one row in the futures table.
type contract struct {
	Symbol             string // exchange ticker (e.g. ES, NQ, CL)
	Exchange           string // MIC code (XCME, XCBT, XNYM, XCEC)
	ContractMultiplier int    // points or units per contract
	CFICode            string
	TickSize           string       // minimum price increment; Price (44) is quoted to it
	Months             []time.Month // listed contract months (the listing cycle)
	// LastTrade is the last trading day of the contract for month (its first
	// day), per the exchange's termination-of-trading rule.
	LastTrade func(month time.Time) time.Time
}

// Listing cycles: equity index, Treasury, and FX futures list the quarterly
// H/M/U/Z months; energy, metals, Bitcoin, and VIX futures list every month
// (metals for their first two years, past the four drawn here); grains follow
// their exchange cycles (per CME Group contract specs).
var (
	quarterly = []time.Month{time.March, time.June, time.September, time.December}
	hknuz     = []time.Month{time.March, time.May, time.July, time.September, time.December}
	monthly   = []time.Month{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12}
)

// Termination-of-trading rules (CME Group and Cboe contract specs), on a
// weekends-only calendar: exchange holidays are out of scope.
var (
	// equity index: the 3rd Friday of the contract month.
	thirdFridayRule = func(m time.Time) time.Time { return catalog.ThirdFriday(m, 0) }
	// CL: 3 business days before the 25th of the prior month, counted from
	// the business day before the 25th when the 25th is not one.
	crudeRule = func(m time.Time) time.Time {
		d := time.Date(m.Year(), m.Month()-1, 25, 0, 0, 0, 0, time.UTC)
		if !isBusinessDay(d) {
			d = businessDaysBefore(d, 1)
		}
		return businessDaysBefore(d, 3)
	}
	// NG: 3 business days before the 1st of the contract month.
	natGasRule = func(m time.Time) time.Time { return businessDaysBefore(m, 3) }
	// COMEX metals: the 3rd-last business day of the contract month.
	metalsRule = func(m time.Time) time.Time { return businessDaysBefore(lastBusinessDay(m), 2) }
	// ZN and ZB: 7 business days before the month's last business day.
	longTreasuryRule = func(m time.Time) time.Time { return businessDaysBefore(lastBusinessDay(m), 7) }
	// ZF: the last business day of the contract month.
	lastBusinessDayRule = lastBusinessDay
	// grains: the business day before the 15th of the contract month.
	grainsRule = func(m time.Time) time.Time {
		return businessDaysBefore(time.Date(m.Year(), m.Month(), 15, 0, 0, 0, 0, time.UTC), 1)
	}
	// FX: 2 business days before the 3rd Wednesday of the contract month.
	fxRule = func(m time.Time) time.Time {
		first := time.Date(m.Year(), m.Month(), 1, 0, 0, 0, 0, time.UTC)
		thirdWednesday := first.AddDate(0, 0, (int(time.Wednesday)-int(first.Weekday())+7)%7+14)
		return businessDaysBefore(thirdWednesday, 2)
	}
	// Bitcoin: the last Friday of the contract month.
	lastFridayRule = func(m time.Time) time.Time {
		d := time.Date(m.Year(), m.Month()+1, 0, 0, 0, 0, 0, time.UTC)
		return d.AddDate(0, 0, -((int(d.Weekday()) - int(time.Friday) + 7) % 7))
	}
	// VX: the Wednesday 30 days before the next month's 3rd Friday.
	vixRule = func(m time.Time) time.Time { return catalog.ThirdFriday(m, 1).AddDate(0, 0, -30) }
)

func isBusinessDay(d time.Time) bool {
	return d.Weekday() != time.Saturday && d.Weekday() != time.Sunday
}

// businessDaysBefore steps d back n weekdays.
func businessDaysBefore(d time.Time, n int) time.Time {
	for n > 0 {
		d = d.AddDate(0, 0, -1)
		if isBusinessDay(d) {
			n--
		}
	}
	return d
}

// lastBusinessDay is the last weekday of m's month.
func lastBusinessDay(m time.Time) time.Time {
	d := time.Date(m.Year(), m.Month()+1, 0, 0, 0, 0, 0, time.UTC)
	if !isBusinessDay(d) {
		d = businessDaysBefore(d, 1)
	}
	return d
}

// futuresContracts lists representative listed-futures contracts
// across asset classes (equity index, energy, metals, rates, ags).
var futuresContracts = []contract{
	{"ES", "XCME", 50, "FFICSX", "0.25", quarterly, thirdFridayRule},                         // E-mini S&P 500
	{"NQ", "XCME", 20, "FFICSX", "0.25", quarterly, thirdFridayRule},                         // E-mini Nasdaq-100
	{"YM", "XCBT", 5, "FFICSX", "1", quarterly, thirdFridayRule},                             // E-mini Dow
	{"CL", "XNYM", 1000, "FFRCSX", "0.01", monthly, crudeRule},                               // WTI Crude Oil
	{"NG", "XNYM", 10000, "FFRCSX", "0.001", monthly, natGasRule},                            // Natural Gas
	{"GC", "XCEC", 100, "FFMCSX", "0.10", monthly, metalsRule},                               // Gold
	{"SI", "XCEC", 5000, "FFMCSX", "0.005", monthly, metalsRule},                             // Silver
	{"HG", "XCEC", 25000, "FFMCSX", "0.0005", monthly, metalsRule},                           // Copper
	{"ZN", "XCBT", 1000, "FFDCSX", "0.015625", quarterly, longTreasuryRule},                  // 10-Year T-Note
	{"ZB", "XCBT", 1000, "FFDCSX", "0.03125", quarterly, longTreasuryRule},                   // 30-Year T-Bond
	{"ZF", "XCBT", 1000, "FFDCSX", "0.0078125", quarterly, lastBusinessDayRule},              // 5-Year T-Note
	{"ZC", "XCBT", 5000, "FFACSX", "0.0025", hknuz, grainsRule},                              // Corn
	{"ZS", "XCBT", 5000, "FFACSX", "0.0025", []time.Month{1, 3, 5, 7, 8, 9, 11}, grainsRule}, // Soybeans
	{"ZW", "XCBT", 5000, "FFACSX", "0.0025", hknuz, grainsRule},                              // Wheat
	{"6E", "XCME", 125000, "FFFCSX", "0.00005", quarterly, fxRule},                           // Euro FX futures
	{"6J", "XCME", 12500000, "FFFCSX", "0.0000005", quarterly, fxRule},                       // Japanese Yen futures
	{"BTC", "XCME", 5, "FFCCSX", "5", monthly, lastFridayRule},                               // Bitcoin futures
	{"VX", "XCBF", 1000, "FFXCSX", "0.05", monthly, vixRule},                                 // VIX futures
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
			AssetCategory: catalog.AssetCategoryFutures,
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

// instrumentBlock emits the futures Instrument component: Symbol (55),
// SecurityType (167=FUT), MaturityMonthYear (200), ContractMultiplier
// (231), SecurityExchange (207), CFICode (461).
func instrumentBlock() []catalog.FieldGenerator {
	return []catalog.FieldGenerator{
		symbolField(),
		catalog.LiteralField(app.TagSecurityType, string(catalog.SecFUT)),
		maturityMonthYearField(),
		contractMultiplierField(),
		securityExchangeField(),
		cfiCodeField(),
	}
}

// contractKey is the GenerateCtx.Memo key for the message's contract.
type contractKey struct{}

// pickedContract picks the message's one contract, memoized so every field
// (symbol, multiplier, exchange, CFI code, contract month) describes it.
func pickedContract(r *rand.Rand, ctx *catalog.GenerateCtx) contract {
	if ctx.Memo == nil {
		ctx.Memo = map[any]any{}
	}
	if v, ok := ctx.Memo[contractKey{}]; ok {
		return v.(contract)
	}
	c := futuresContracts[r.Intn(len(futuresContracts))] // #nosec G404
	ctx.Memo[contractKey{}] = c
	return c
}

func symbolField() catalog.FieldGenerator {
	return func(r *rand.Rand, ctx *catalog.GenerateCtx) catalog.Field {
		c := pickedContract(r, ctx)
		return catalog.Field{Tag: app.TagSymbol, Value: c.Symbol}
	}
}

func maturityMonthYearField() catalog.FieldGenerator {
	return func(r *rand.Rand, ctx *catalog.GenerateCtx) catalog.Field {
		// One of the contract's next four listed months still trading on the
		// trade date: from the trade month on, past each month's last trade.
		c := pickedContract(r, ctx)
		trade := ctx.TradeDate()
		var listed []time.Time
		for k := 0; len(listed) < 4; k++ {
			m := time.Date(trade.Year(), trade.Month()+time.Month(k), 1, 0, 0, 0, 0, time.UTC)
			if slices.Contains(c.Months, m.Month()) && !c.LastTrade(m).Before(trade) {
				listed = append(listed, m)
			}
		}
		return catalog.Field{Tag: TagMaturityMonthYear, Value: listed[r.Intn(len(listed))].Format("200601")} // #nosec G404
	}
}

func contractMultiplierField() catalog.FieldGenerator {
	return func(r *rand.Rand, ctx *catalog.GenerateCtx) catalog.Field {
		c := pickedContract(r, ctx)
		return catalog.Field{Tag: TagContractMultiplier, Value: fmt.Sprintf("%d", c.ContractMultiplier)}
	}
}

func securityExchangeField() catalog.FieldGenerator {
	return func(r *rand.Rand, ctx *catalog.GenerateCtx) catalog.Field {
		c := pickedContract(r, ctx)
		return catalog.Field{Tag: TagSecurityExchange, Value: c.Exchange}
	}
}

func cfiCodeField() catalog.FieldGenerator {
	return func(r *rand.Rand, ctx *catalog.GenerateCtx) catalog.Field {
		c := pickedContract(r, ctx)
		return catalog.Field{Tag: TagCFICode, Value: c.CFICode}
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
	choices := []string{app.OrdTypeMarket, app.OrdTypeLimit, app.OrdTypeStop, app.OrdTypeStopLimit}
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
	qty := 1 + r.Intn(1000) // 1-1000 contracts // #nosec G404
	return fmt.Sprintf("%d", qty)
}

func leavesQty() catalog.FieldGenerator {
	return func(r *rand.Rand, ctx *catalog.GenerateCtx) catalog.Field {
		return catalog.Field{Tag: app.TagLeavesQty, Value: catalog.OrderQty(r, ctx, drawQty)}
	}
}

// mids are static reference prices per contract, in its tick size's units;
// each price jitters within +/-1% of its mid and lands on a whole tick. Feed
// live or configured mids if absolute levels matter.
var mids = map[string]float64{
	"ES": 6000, "NQ": 21000, "YM": 43000,
	"CL": 70, "NG": 3.2, "GC": 2700, "SI": 31, "HG": 4.4,
	"ZN": 111, "ZB": 118, "ZF": 107,
	"ZC": 4.3, "ZS": 10.4, "ZW": 5.6,
	"6E": 1.12, "6J": 0.006667, "BTC": 95000, "VX": 17,
}

func price() catalog.FieldGenerator {
	return func(r *rand.Rand, ctx *catalog.GenerateCtx) catalog.Field {
		c := pickedContract(r, ctx)
		mid, ok := mids[c.Symbol]
		if !ok {
			// Safety net: every contract has a mid; one added without it
			// still prices instead of emitting 0.
			mid = 100
		}
		tick, err := strconv.ParseFloat(c.TickSize, 64)
		if err != nil || tick <= 0 {
			tick = 0.01 // safety net: every table TickSize parses
		}
		_, frac, _ := strings.Cut(c.TickSize, ".")
		px := math.Round(mid*(1+(r.Float64()*2-1)*0.01)/tick) * tick // #nosec G404
		return catalog.Field{Tag: app.TagPrice, Value: strconv.FormatFloat(px, 'f', len(frac), 64)}
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
