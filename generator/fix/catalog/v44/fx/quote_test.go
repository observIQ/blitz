package fx

import (
	"math/rand"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/observiq/blitz/generator/fix/catalog"
	"github.com/observiq/blitz/generator/fix/catalog/v44/app"
)

// marketBands are plausible trading ranges and quoting precision per pair,
// written independently of the generator's own table so a wrong level fails.
var marketBands = map[string]struct {
	lo, hi   float64
	decimals int
}{
	"EUR/USD": {1.00, 1.25, 5}, "GBP/USD": {1.15, 1.45, 5}, "USD/JPY": {130, 170, 3},
	"USD/CHF": {0.75, 0.98, 5}, "AUD/USD": {0.58, 0.75, 5}, "USD/CAD": {1.30, 1.48, 5},
	"NZD/USD": {0.53, 0.68, 5}, "EUR/GBP": {0.80, 0.92, 5}, "EUR/JPY": {145, 185, 3},
	"USD/MXN": {16, 22, 4}, "USD/BRL": {4.8, 6.5, 4}, "USD/INR": {80, 92, 4},
	"USD/CNY": {6.8, 7.5, 4}, "USD/KRW": {1250, 1550, 2}, "USD/TWD": {28, 35, 3},
	"USD/RUB": {65, 110, 4},
}

// TestPriceIsRealisticForPair pins each pair's price to its market range and
// quoting precision (JPY pairs 3 decimals, KRW 2, and so on).
func TestPriceIsRealisticForPair(t *testing.T) {
	reregisterAll()
	def := catalog.Get(catalog.MessageKey{Version: catalog.V44, MsgType: app.MsgTypeNewOrderSingle, AssetCategory: catalog.AssetCategoryFX})
	for seed := int64(0); seed < 400; seed++ {
		tags := buildTagMap(def.Fields, rand.New(rand.NewSource(seed)))
		sym, px := tags[app.TagSymbol], tags[app.TagPrice]
		band, ok := marketBands[sym]
		if !ok {
			t.Fatalf("seed %d: no market band for %s", seed, sym)
		}
		v, err := strconv.ParseFloat(px, 64)
		if err != nil {
			t.Fatal(err)
		}
		if v < band.lo || v > band.hi {
			t.Fatalf("seed %d: %s price %s outside %v-%v", seed, sym, px, band.lo, band.hi)
		}
		if dot := strings.Index(px, "."); dot < 0 || len(px)-dot-1 != band.decimals {
			t.Fatalf("seed %d: %s price %s must have %d decimals", seed, sym, px, band.decimals)
		}
	}
}

// TestOutrightSettlesAtSpotPlusTenor pins market tenor convention: a 1M or
// 3M outright settles that many calendar months after spot and a 1W one week
// after, rolled to a business day.
func TestOutrightSettlesAtSpotPlusTenor(t *testing.T) {
	trade := time.Date(2026, 1, 28, 14, 0, 0, 0, time.UTC) // spot 30 Jan; +1M crosses a short month
	for seed := int64(0); seed < 400; seed++ {
		r := rand.New(rand.NewSource(seed))
		ctx := &catalog.GenerateCtx{Now: trade}
		in := pickedInstrument(r, ctx)
		if in.SecType == catalog.SecFOR {
			continue
		}
		st := pickedSettlement(r, ctx)
		lag := 2
		if in.Pair.CCY1+in.Pair.CCY2 == "USDCAD" {
			lag = 1
		}
		if in.Pair.TenorMonths == 0 && in.Pair.TenorDays == 0 {
			t.Fatalf("seed %d: %s %s has no tenor", seed, in.SecType, in.Pair.CCY1+in.Pair.CCY2)
		}
		// Spot is Fri 30 Jan (Thu 29 Jan for USD/CAD). Worked by hand from
		// the ISDA rules: month tenors from Jan 30 (January's last business
		// day) keep end of month; from Jan 29 they clamp to Feb 28 (a
		// Saturday) and roll back to Feb 27 rather than into March.
		want := map[[2]int]string{{1, 0}: "20260227", {3, 0}: "20260430", {0, 7}: "20260206"}
		if lag == 1 {
			want = map[[2]int]string{{1, 0}: "20260227", {3, 0}: "20260429", {0, 7}: "20260205"}
		}
		w, ok := want[[2]int{in.Pair.TenorMonths, in.Pair.TenorDays}]
		if !ok {
			t.Fatalf("seed %d: no worked date for tenor %dM%dD", seed, in.Pair.TenorMonths, in.Pair.TenorDays)
		}
		if got := st.Date.Format("20060102"); got != w {
			t.Fatalf("seed %d: %s %s settles %s, want %s", seed, in.SecType, in.Pair.CCY1+in.Pair.CCY2, got, w)
		}
	}
}
