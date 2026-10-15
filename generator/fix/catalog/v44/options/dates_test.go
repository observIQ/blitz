package options

import (
	"math/rand"
	"slices"
	"testing"
	"time"

	"github.com/observiq/blitz/generator/fix/catalog"
)

var tradeDate = time.Date(2031, 6, 18, 14, 0, 0, 0, time.UTC)

// maturity draws MaturityDate for a message whose contract is u.
func maturity(t *testing.T, seed int64, u underlying) time.Time {
	t.Helper()
	return maturityOn(t, tradeDate, seed, u)
}

// maturityOn draws MaturityDate on the given trade date.
func maturityOn(t *testing.T, tradeDate time.Time, seed int64, u underlying) time.Time {
	t.Helper()
	ctx := &catalog.GenerateCtx{Now: tradeDate, Memo: map[any]any{contractKey{}: contract{underlying: u}}}
	mat, err := time.Parse("20060102", maturityDateField()(rand.New(rand.NewSource(seed)), ctx).Value)
	if err != nil {
		t.Fatal(err)
	}
	if !mat.After(tradeDate) {
		t.Fatalf("%s seed %d: expiration %s is not after trade date", u.Symbol, seed, mat.Format("20060102"))
	}
	return mat
}

func isThirdFriday(d time.Time) bool {
	return d.Weekday() == time.Friday && d.Day() >= 15 && d.Day() <= 21
}

// TestMaturityIsAFutureThirdFriday pins listed-option expiration: the third
// Friday of a month after the trade date, for every underlying but VIX.
func TestMaturityIsAFutureThirdFriday(t *testing.T) {
	for _, u := range optionUnderlyings {
		if u.Symbol == "VIX" {
			continue
		}
		for seed := int64(0); seed < 50; seed++ {
			if mat := maturity(t, seed, u); !isThirdFriday(mat) {
				t.Fatalf("%s seed %d: expiration %s is not a third Friday", u.Symbol, seed, mat.Format("20060102"))
			}
		}
	}
}

// TestVIXMaturityIsThirtyDaysBeforeNextThirdFriday pins Cboe's VIX option
// expiration: the Wednesday 30 days before the third Friday of the following
// month.
func TestVIXMaturityIsThirtyDaysBeforeNextThirdFriday(t *testing.T) {
	vix := underlying{Symbol: "VIX", Exchange: "XCBO", Index: true}
	for seed := int64(0); seed < 50; seed++ {
		mat := maturity(t, seed, vix)
		if mat.Weekday() != time.Wednesday || !isThirdFriday(mat.AddDate(0, 0, 30)) {
			t.Fatalf("seed %d: VIX expiration %s is not the Wednesday 30 days before a third Friday", seed, mat.Format("20060102"))
		}
	}
}

// drawnMaturities is every MaturityDate u's options draw on the trade date.
func drawnMaturities(t *testing.T, trade time.Time, u underlying) []string {
	t.Helper()
	var got []string
	for seed := int64(0); seed < 2000; seed++ {
		if d := maturityOn(t, trade, seed, u).Format("20060102"); !slices.Contains(got, d) {
			got = append(got, d)
		}
	}
	slices.Sort(got)
	return got
}

// TestMaturitiesAreListedSeries pins each class to the expirations always
// listed on the trade date, so every option names a real series:
//   - Stocks and ETFs (OIC): the two near-term months, plus January LEAPS.
//     The next LEAPS year lists on the Monday of September expiration week.
//   - SPX, NDX, and RUT (Cboe RUT spec): three near-term months plus three
//     from the March quarterly cycle.
//   - VIX: the three near-term expirations. On its expiration day a VIX
//     series has already stopped trading.
func TestMaturitiesAreListedSeries(t *testing.T) {
	june := time.Date(2031, 6, 18, 14, 0, 0, 0, time.UTC)
	cases := []struct {
		name  string
		trade time.Time
		u     underlying
		want  []string
	}{
		{"stock", june, underlying{Symbol: "AAPL", Exchange: "XCBO", Index: false}, []string{"20310620", "20310718", "20320116", "20330121"}},
		{"etf", june, underlying{Symbol: "SPY", Exchange: "XCBO", Index: false}, []string{"20310620", "20310718", "20320116", "20330121"}},
		{"stock before LEAPS listing", time.Date(2031, 9, 12, 14, 0, 0, 0, time.UTC), underlying{Symbol: "AAPL", Exchange: "XCBO", Index: false},
			[]string{"20310919", "20311017", "20320116", "20330121"}},
		{"stock in LEAPS listing week", time.Date(2031, 9, 15, 14, 0, 0, 0, time.UTC), underlying{Symbol: "AAPL", Exchange: "XCBO", Index: false},
			[]string{"20310919", "20311017", "20320116", "20330121", "20340120"}},
		{"index", june, underlying{Symbol: "SPX", Exchange: "XCBO", Index: true}, []string{"20310620", "20310718", "20310815", "20310919", "20311219", "20320319"}},
		{"index on a quarterly month", time.Date(2031, 7, 21, 14, 0, 0, 0, time.UTC), underlying{Symbol: "RUT", Exchange: "XCBO", Index: true},
			[]string{"20310815", "20310919", "20311017", "20311219", "20320319", "20320618"}},
		{"vix", june, underlying{Symbol: "VIX", Exchange: "XCBO", Index: true}, []string{"20310716", "20310820", "20310917"}},
	}
	for _, c := range cases {
		if got := drawnMaturities(t, c.trade, c.u); !slices.Equal(got, c.want) {
			t.Errorf("%s on %s: maturities %v, want %v", c.name, c.trade.Format("20060102"), got, c.want)
		}
	}
}
