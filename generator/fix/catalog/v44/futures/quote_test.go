package futures

import (
	"math"
	"math/rand"
	"strconv"
	"strings"
	"testing"

	"github.com/observiq/blitz/generator/fix/catalog"
	"github.com/observiq/blitz/generator/fix/catalog/v44/app"
)

// marketBands are plausible trading ranges per contract, in the units of the
// contract's tick size, written independently of the generator's own table
// so a wrong level fails.
var marketBands = map[string]struct{ lo, hi float64 }{
	"ES": {4000, 8000}, "NQ": {15000, 30000}, "YM": {33000, 52000},
	"CL": {40, 110}, "NG": {1.5, 8}, "GC": {1800, 5000}, "SI": {20, 60}, "HG": {3, 6.5},
	"ZN": {100, 125}, "ZB": {105, 135}, "ZF": {100, 115},
	"ZC": {3, 6.5}, "ZS": {8.5, 15}, "ZW": {4, 8},
	"6E": {1.0, 1.25}, "6J": {0.0055, 0.008}, "BTC": {30000, 150000}, "VX": {11, 40},
}

// TestPriceIsRealisticForContract pins each contract's price to its market
// range and to a whole number of its ticks.
func TestPriceIsRealisticForContract(t *testing.T) {
	reregisterAll()
	def := catalog.Get(catalog.MessageKey{Version: catalog.V44, MsgType: app.MsgTypeNewOrderSingle, AssetCategory: catalog.AssetCategoryFutures})
	ticks := map[string]string{}
	for _, c := range futuresContracts {
		ticks[c.Symbol] = c.TickSize
	}
	seen := map[string]bool{}
	for seed := int64(0); seed < 600; seed++ {
		tags := buildTagMap(def.Fields, rand.New(rand.NewSource(seed)))
		sym, px := tags[app.TagSymbol], tags[app.TagPrice]
		seen[sym] = true
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
		tick, err := strconv.ParseFloat(ticks[sym], 64)
		if err != nil {
			t.Fatal(err)
		}
		if n := v / tick; math.Abs(n-math.Round(n)) > 1e-6 {
			t.Fatalf("seed %d: %s price %s is not a multiple of tick %s", seed, sym, px, ticks[sym])
		}
		_, tickFrac, _ := strings.Cut(ticks[sym], ".")
		if _, frac, _ := strings.Cut(px, "."); len(frac) != len(tickFrac) {
			t.Fatalf("seed %d: %s price %s not quoted to tick %s precision", seed, sym, px, ticks[sym])
		}
	}
	if len(seen) != len(futuresContracts) {
		t.Fatalf("saw %d of %d contracts", len(seen), len(futuresContracts))
	}
}
