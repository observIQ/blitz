package catalog

import (
	"math/rand"
	"testing"
	"time"
)

func TestLiteralField(t *testing.T) {
	g := LiteralField(35, "D")
	got := g(rand.New(rand.NewSource(1)), &GenerateCtx{})
	if got.Tag != 35 || got.Value != "D" {
		t.Errorf("LiteralField result = %+v, want Tag=35 Value=D", got)
	}
}

func TestLiteralFieldIgnoresRNG(t *testing.T) {
	g := LiteralField(35, "D")
	// Calling with a different RNG must give the same result — that's
	// the whole point of a literal.
	a := g(rand.New(rand.NewSource(1)), &GenerateCtx{})
	b := g(rand.New(rand.NewSource(99)), &GenerateCtx{})
	if a != b {
		t.Errorf("LiteralField varies with RNG: %v vs %v", a, b)
	}
}

func TestIntField(t *testing.T) {
	g := IntField(34, 42)
	got := g(rand.New(rand.NewSource(1)), &GenerateCtx{})
	if got.Tag != 34 || got.Value != "42" {
		t.Errorf("IntField(34, 42) = %+v, want Tag=34 Value=42", got)
	}
}

// TestAddTenor pins the money-market tenor conventions (ISDA): a month tenor
// never overflows into the next month, rolls modified-following, and keeps
// the end-of-month rule; a day tenor rolls following.
func TestAddTenor(t *testing.T) {
	d := func(s string) time.Time {
		v, err := time.Parse("2006-01-02", s)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	cases := []struct {
		name         string
		spot         string
		months, days int
		want         string
	}{
		{"end of month stays end of month", "2026-01-30", 1, 0, "2026-02-27"}, // Jan 30 is January's last business day
		{"no overflow into next month", "2026-01-29", 1, 0, "2026-02-27"},     // Feb 28 is a Saturday; following would cross into March
		{"weekend rolls forward in month", "2026-01-15", 1, 0, "2026-02-16"},  // Feb 15 is a Sunday
		{"end of month to longer month", "2026-02-27", 1, 0, "2026-03-31"},    // Feb 27 is February's last business day
		{"plain tenor", "2026-03-10", 3, 0, "2026-06-10"},
		{"one week rolls following", "2026-01-30", 0, 7, "2026-02-06"},
		{"one week across a weekend", "2026-02-27", 0, 7, "2026-03-06"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := AddTenor(d(tc.spot), tc.months, tc.days)
			if got.Format("2006-01-02") != tc.want {
				t.Fatalf("AddTenor(%s, %dM, %dD) = %s, want %s", tc.spot, tc.months, tc.days, got.Format("2006-01-02"), tc.want)
			}
		})
	}
}

// TestBondDatesSpanTheTerm pins that a bond's issue and maturity dates are
// its term apart, on the 15th, with the trade date strictly between them, so
// the bond is outstanding on any trade date.
func TestBondDatesSpanTheTerm(t *testing.T) {
	trade := time.Date(2031, 6, 18, 14, 0, 0, 0, time.UTC)
	for _, months := range []int{24, 120, 360} {
		for seed := int64(0); seed < 200; seed++ {
			issue, mat := BondDates(rand.New(rand.NewSource(seed)), trade, months)
			if !issue.Before(trade) || !mat.After(trade) {
				t.Fatalf("%d-month bond %s..%s does not span trade date", months, issue.Format("20060102"), mat.Format("20060102"))
			}
			if !issue.AddDate(0, months, 0).Equal(mat) || mat.Day() != 15 {
				t.Fatalf("%d-month bond %s..%s is not its term apart on the 15th", months, issue.Format("20060102"), mat.Format("20060102"))
			}
		}
	}
}

// TestBillDatesSpanTheTerm pins a Treasury bill's dates: issued and maturing
// on its auction weekday, its term in weeks apart, spanning the trade date.
func TestBillDatesSpanTheTerm(t *testing.T) {
	// A week of trade dates, so each auction weekday is also a trade day:
	// a bill issued that day is not yet outstanding.
	for d := range 7 {
		trade := time.Date(2031, 6, 16+d, 14, 0, 0, 0, time.UTC)
		tradeDay := time.Date(2031, 6, 16+d, 0, 0, 0, 0, time.UTC)
		testBillDates(t, trade, tradeDay)
	}
}

func testBillDates(t *testing.T, trade, tradeDay time.Time) {
	t.Helper()
	for _, tc := range []struct {
		weeks int
		day   time.Weekday
	}{{4, time.Tuesday}, {13, time.Thursday}, {52, time.Thursday}} {
		for seed := int64(0); seed < 200; seed++ {
			issue, mat := BillDates(rand.New(rand.NewSource(seed)), trade, tc.weeks, tc.day)
			if !issue.Before(tradeDay) || !mat.After(tradeDay) {
				t.Fatalf("%dW bill %s..%s does not span trade date %s", tc.weeks, issue.Format("20060102"), mat.Format("20060102"), tradeDay.Format("20060102"))
			}
			if !issue.AddDate(0, 0, 7*tc.weeks).Equal(mat) || mat.Weekday() != tc.day {
				t.Fatalf("%dW bill %s..%s is not %d weeks apart on %s", tc.weeks, issue.Format("20060102"), mat.Format("20060102"), tc.weeks, tc.day)
			}
		}
	}
}
