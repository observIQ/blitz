package equities

import (
	"math/rand"
	"strconv"
	"strings"
	"testing"

	"github.com/observiq/blitz/generator/fix/catalog"
	"github.com/observiq/blitz/generator/fix/catalog/v44/app"
)

// marketBands are plausible trading ranges per symbol, written independently
// of the generator's own table so a wrong level fails.
var marketBands = map[string]struct{ lo, hi float64 }{
	"AAPL": {150, 300}, "MSFT": {300, 550}, "GOOGL": {120, 260}, "AMZN": {150, 260},
	"JPM": {180, 330}, "JNJ": {140, 200}, "V": {250, 380}, "MA": {450, 620},
	"BAC.PB": {20, 28}, "JPM.PD": {20, 28}, "GS.PA": {17, 27}, "WFC.PL": {1100, 1400},
	"SPY": {450, 750}, "IVV": {450, 750}, "VOO": {420, 690}, "QQQ": {380, 650},
	"DIA": {380, 500}, "IWM": {180, 270},
	"VFINX": {450, 700}, "FXAIX": {160, 260}, "VTSAX": {110, 170}, "SWPPX": {75, 115},
	"BABA": {70, 200}, "TSM": {130, 330}, "NVS": {90, 130}, "BP": {25, 40},
	"ZVZZT.WS": {0.5, 5}, "ZWZZT.WS": {0.5, 5}, "ZXZZT.R": {0.5, 5}, "ZBZZT.RT": {0.5, 5},
}

// TestPriceIsRealisticForSymbol pins each symbol's price to its market range
// in whole cents.
func TestPriceIsRealisticForSymbol(t *testing.T) {
	reregisterAll()
	def := catalog.Get(catalog.MessageKey{Version: catalog.V44, MsgType: app.MsgTypeNewOrderSingle, AssetCategory: catalog.AssetCategoryEquities})
	seen := map[string]bool{}
	for seed := int64(0); seed < 1000; seed++ {
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
		if _, frac, _ := strings.Cut(px, "."); len(frac) != 2 {
			t.Fatalf("seed %d: %s price %s not in whole cents", seed, sym, px)
		}
	}
	if len(seen) != len(marketBands) {
		t.Fatalf("saw %d of %d symbols", len(seen), len(marketBands))
	}
}
