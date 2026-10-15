package futures

import (
	"maps"
	"math/rand"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/observiq/blitz/generator/fix/catalog"
	"github.com/observiq/blitz/generator/fix/catalog/v44/app"
	"github.com/stretchr/testify/require"
)

// TestMaturityMonthFollowsTradeDate pins that the contract month is the trade
// month or one of the next twelve; TestContractMonthIsStillTrading pins that
// it has not stopped trading.
func TestMaturityMonthFollowsTradeDate(t *testing.T) {
	trade := time.Date(2031, 6, 18, 14, 0, 0, 0, time.UTC)
	for seed := int64(0); seed < 300; seed++ {
		r := rand.New(rand.NewSource(seed))
		got, err := time.Parse("200601", maturityMonthYearField()(r, &catalog.GenerateCtx{Now: trade}).Value)
		if err != nil {
			t.Fatal(err)
		}
		months := (got.Year()-trade.Year())*12 + int(got.Month()-trade.Month())
		if months < 0 || months > 12 {
			t.Fatalf("seed %d: contract month %s is %d months from trade date %s", seed, got.Format("200601"), months, trade.Format("20060102"))
		}
	}
}

// listingCycles are the CME/CBOT/NYMEX/COMEX/CFE listed months per contract,
// written independently of the generator's table.
var listingCycles = map[string][]time.Month{
	"ES": {3, 6, 9, 12}, "NQ": {3, 6, 9, 12}, "YM": {3, 6, 9, 12},
	"ZN": {3, 6, 9, 12}, "ZB": {3, 6, 9, 12}, "ZF": {3, 6, 9, 12},
	"6E": {3, 6, 9, 12}, "6J": {3, 6, 9, 12},
	"GC": {1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12}, "SI": {1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12},
	"HG": {1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12},
	"ZC": {3, 5, 7, 9, 12}, "ZS": {1, 3, 5, 7, 8, 9, 11}, "ZW": {3, 5, 7, 9, 12},
	"CL": {1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12}, "NG": {1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12},
	"BTC": {1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12}, "VX": {1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12},
}

// TestMessageNamesOneListedContract pins that every field of a futures
// message describes one contract (Symbol, multiplier, exchange, CFI code) and
// that its contract month is one the contract actually lists.
func TestMessageNamesOneListedContract(t *testing.T) {
	reregisterAll()
	def := catalog.Get(catalog.MessageKey{Version: catalog.V44, MsgType: app.MsgTypeNewOrderSingle, AssetCategory: catalog.AssetCategoryFutures})
	byMult := map[string]contract{}
	for _, c := range futuresContracts {
		byMult[c.Symbol] = c
	}
	for seed := int64(0); seed < 300; seed++ {
		tags := buildTagMap(def.Fields, rand.New(rand.NewSource(seed)))
		c, ok := byMult[tags[app.TagSymbol]]
		if !ok {
			t.Fatalf("seed %d: unknown symbol %q", seed, tags[app.TagSymbol])
		}
		if tags[TagContractMultiplier] != strconv.Itoa(c.ContractMultiplier) || tags[TagSecurityExchange] != c.Exchange || tags[TagCFICode] != c.CFICode {
			t.Fatalf("seed %d: %s message mixes contracts: multiplier %s, exchange %s, CFI %s",
				seed, c.Symbol, tags[TagContractMultiplier], tags[TagSecurityExchange], tags[TagCFICode])
		}
		m, err := time.Parse("200601", tags[TagMaturityMonthYear])
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Contains(listingCycles[c.Symbol], m.Month()) {
			t.Fatalf("seed %d: %s does not list %s", seed, c.Symbol, m.Format("Jan 2006"))
		}
	}
}

// drawnMonths returns every contract month symbol's MaturityMonthYear draws
// across many seeds on trade.
func drawnMonths(t *testing.T, symbol string, trade time.Time) []string {
	t.Helper()
	var c contract
	for _, fc := range futuresContracts {
		if fc.Symbol == symbol {
			c = fc
		}
	}
	require.Equal(t, symbol, c.Symbol)
	seen := map[string]bool{}
	for seed := int64(0); seed < 200; seed++ {
		ctx := &catalog.GenerateCtx{Now: trade, Memo: map[any]any{contractKey{}: c}}
		seen[maturityMonthYearField()(rand.New(rand.NewSource(seed)), ctx).Value] = true
	}
	out := slices.Sorted(maps.Keys(seen))
	return out
}

// TestContractMonthIsStillTrading pins each exchange's last-trade rule at its
// edge: on a contract's last trading day it is the front month offered, and
// the day after it is gone. Dates are worked by hand from the CME Group and
// Cboe contract specs (weekends-only calendar).
func TestContractMonthIsStillTrading(t *testing.T) {
	for _, c := range []struct {
		symbol, month, lastTrade string
	}{
		{"CL", "202607", "2026-06-22"}, // 3 business days before the 25th of the prior month
		{"NG", "202608", "2026-07-29"}, // 3rd-last business day of the prior month
		{"ES", "202703", "2027-03-19"}, // 3rd Friday of the contract month
		{"YM", "202703", "2027-03-19"},
		{"GC", "202608", "2026-08-27"},  // 3rd-last business day of the contract month
		{"ZN", "202609", "2026-09-21"},  // 7 business days before the month's last business day
		{"ZF", "202609", "2026-09-30"},  // last business day of the contract month
		{"ZC", "202609", "2026-09-14"},  // business day before the 15th
		{"6E", "202609", "2026-09-14"},  // 2 business days before the 3rd Wednesday
		{"BTC", "202607", "2026-07-31"}, // last Friday of the contract month
		{"VX", "202607", "2026-07-22"},  // Wednesday 30 days before the next month's 3rd Friday
	} {
		last, err := time.Parse(time.DateOnly, c.lastTrade)
		require.NoError(t, err)
		require.Equal(t, c.month, drawnMonths(t, c.symbol, last.Add(14*time.Hour))[0], "%s on its last trading day %s", c.symbol, c.lastTrade)
		require.NotContains(t, drawnMonths(t, c.symbol, last.AddDate(0, 0, 1).Add(14*time.Hour)), c.month, "%s after %s", c.symbol, c.lastTrade)
	}
	// Early in its own month, a still-trading contract is the front month.
	require.Equal(t, "202703", drawnMonths(t, "ES", time.Date(2027, 3, 2, 14, 0, 0, 0, time.UTC))[0])
}

// TestMetalsListConsecutiveNearMonths pins the COMEX listing: gold, silver,
// and copper list every month for two years out, so the next four still
// trading on 13 October 2026 are October through January.
func TestMetalsListConsecutiveNearMonths(t *testing.T) {
	trade := time.Date(2026, 10, 13, 14, 0, 0, 0, time.UTC)
	for _, symbol := range []string{"GC", "SI", "HG"} {
		require.Equal(t, []string{"202610", "202611", "202612", "202701"}, drawnMonths(t, symbol, trade), symbol)
	}
}

// TestFXRuleUsesTheThirdWednesday pins the FX rule in a month that starts on
// a Thursday, where the 3rd Wednesday (the 21st) falls after the 3rd Friday
// (the 16th): October 2026 trades through Monday the 19th.
func TestFXRuleUsesTheThirdWednesday(t *testing.T) {
	require.Equal(t, "2026-10-19", fxRule(time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)).Format(time.DateOnly))
}
