package otcderivs

import (
	"math/rand"
	"testing"
	"time"

	"github.com/observiq/blitz/generator/fix/catalog"
)

// TestDatesFollowTradeDate pins that maturity is after the trade date
// for any trade date, so no instrument is emitted already matured.
func TestDatesFollowTradeDate(t *testing.T) {
	trade := time.Date(2031, 6, 18, 14, 0, 0, 0, time.UTC)
	for seed := int64(0); seed < 300; seed++ {
		r := rand.New(rand.NewSource(seed))
		ctx := &catalog.GenerateCtx{Now: trade}
		mat, err := time.Parse("20060102", maturityDateField()(r, ctx).Value)
		if err != nil {
			t.Fatal(err)
		}
		if !mat.After(trade) {
			t.Fatalf("seed %d: maturity %s is not after trade date %s", seed, mat.Format("20060102"), trade.Format("20060102"))
		}
	}
}

// TestMaturityMatchesTenor pins that a swap's maturity is its named tenor
// after spot (CDS: its semi-annual roll date), not an unrelated random date.
func TestMaturityMatchesTenor(t *testing.T) {
	trade := time.Date(2031, 6, 18, 14, 0, 0, 0, time.UTC)
	for seed := int64(0); seed < 300; seed++ {
		r := rand.New(rand.NewSource(seed))
		ctx := &catalog.GenerateCtx{Now: trade}
		p := pickedSwap(r, ctx)
		mat, err := time.Parse("20060102", maturityDateField()(r, ctx).Value)
		if err != nil {
			t.Fatal(err)
		}
		// Trade Wed 18 Jun 2031, spot Fri 20 Jun. Worked by hand: a CDS
		// traded between Mar 20 and Sep 19 is on the June roll, so a 5Y
		// matures 20 Jun 2036 (ISDA semi-annual roll, Dec 2015); a swap
		// matures its tenor after spot, modified following.
		want := map[string]string{
			"USD-IRS-5Y": "20360620", "EUR-IRS-10Y": "20410620", "GBP-IRS-2Y": "20330620",
			"CDS-XYZ-5Y": "20360620", "CDS-CDX-IG": "20360620", "CDS-iTraxx": "20360620",
			"USD-BASIS-SOFR-FF": "20360620", "EUR-BASIS-6M-3M": "20360620",
			"SPX-VAR-1Y": "20320621", "VIX-VAR-3M": "20310922", "TRS-SPX-1Y": "20320621",
			"TRS-AAPL-1Y": "20320621", "XCS-USDEUR-5Y": "20360620", "XCS-USDJPY-10Y": "20410620",
		}[p.Swap.Symbol]
		if got := mat.Format("20060102"); got != want {
			t.Fatalf("seed %d: %s maturity %s, want %s", seed, p.Swap.Symbol, got, want)
		}
	}
}

// TestCDSMaturityFollowsSemiAnnualRoll pins the ISDA semi-annual roll: trades
// from Mar 20 to Sep 19 roll to June, from Sep 20 to Mar 19 to December.
func TestCDSMaturityFollowsSemiAnnualRoll(t *testing.T) {
	for trade, want := range map[string]string{
		"2031-07-01": "2036-06-20", "2031-03-20": "2036-06-20", "2031-09-19": "2036-06-20",
		"2031-09-20": "2036-12-20", "2031-10-01": "2036-12-20", "2032-02-01": "2036-12-20", "2032-03-19": "2036-12-20",
	} {
		d, _ := time.Parse("2006-01-02", trade)
		if got := cdsMaturity(d, 60).Format("2006-01-02"); got != want {
			t.Errorf("5Y CDS traded %s matures %s, want %s", trade, got, want)
		}
	}
}

// TestSONIASwapStartsOnTheTradeDate pins GBP SONIA OIS's same-day start: a
// 2Y traded Mon 12 Oct 2026 matures Thu 12 Oct 2028, not 2Y after a T+2 spot.
func TestSONIASwapStartsOnTheTradeDate(t *testing.T) {
	var gbp swap
	for _, s := range swapsByType[catalog.SecIRS] {
		if s.Underlying == "GBP-SONIA" {
			gbp = s
		}
	}
	ctx := &catalog.GenerateCtx{Now: time.Date(2026, 10, 12, 14, 0, 0, 0, time.UTC),
		Memo: map[any]any{pickKey{}: pickedPair{SecType: catalog.SecIRS, Swap: gbp}}}
	if got := maturityDateField()(rand.New(rand.NewSource(1)), ctx).Value; got != "20281012" {
		t.Fatalf("GBP-IRS-2Y traded 2026-10-12 matures %s, want 20281012", got)
	}
}
