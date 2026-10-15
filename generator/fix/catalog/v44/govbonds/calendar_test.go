package govbonds

import (
	"math/rand"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/observiq/blitz/generator/fix/catalog"
	"github.com/observiq/blitz/generator/fix/catalog/v44/app"
	"github.com/stretchr/testify/require"
)

// cusipCheckDigitOK reports whether c's ninth character is its CUSIP check
// digit (modulus 10 "double-add-double").
func cusipCheckDigitOK(c string) bool {
	if len(c) != 9 {
		return false
	}
	sum := 0
	for i, ch := range c[:8] {
		v := int(ch - '0')
		if ch >= 'A' && ch <= 'Z' {
			v = int(ch-'A') + 10
		}
		if i%2 == 1 {
			v *= 2
		}
		sum += v/10 + v%10
	}
	return int(c[8]-'0') == (10-sum%10)%10
}

// messages builds n gov-bond NewOrderSingle tag maps on trade.
func messages(t *testing.T, trade time.Time, n int) []map[catalog.Tag]string {
	t.Helper()
	reregisterAll()
	def := catalog.Get(catalog.MessageKey{Version: catalog.V44, MsgType: app.MsgTypeNewOrderSingle, AssetCategory: catalog.AssetCategoryGovBonds})
	out := make([]map[catalog.Tag]string, n)
	for seed := range out {
		ctx := &catalog.GenerateCtx{Version: catalog.V44, AssetCategory: catalog.AssetCategoryGovBonds, Now: trade}
		r := rand.New(rand.NewSource(int64(seed)))
		tags := map[catalog.Tag]string{}
		for _, g := range def.Fields {
			f := g(r, ctx)
			tags[f.Tag] = f.Value
		}
		out[seed] = tags
	}
	return out
}

// TestCUSIPsAreValid pins that every emitted CUSIP carries a valid check
// digit, real or synthetic, today and years from now.
func TestCUSIPsAreValid(t *testing.T) {
	for _, trade := range []time.Time{time.Date(2026, 10, 6, 14, 0, 0, 0, time.UTC), time.Date(2031, 6, 18, 14, 0, 0, 0, time.UTC)} {
		for _, tags := range messages(t, trade, 500) {
			require.True(t, cusipCheckDigitOK(tags[TagSecurityID]), "%s CUSIP %s", tags[app.TagSymbol], tags[TagSecurityID])
		}
	}
}

// realIssues are the latest original issues per row on TreasuryDirect
// (2026-10-06), keyed by symbol: CUSIP, coupon, issue date, maturity date.
var realIssues = map[string][4]string{
	"T-NOTE-2Y":  {"91282CRP8", "4.750", "20260930", "20280930"},
	"T-NOTE-3Y":  {"91282CRL7", "4.375", "20260915", "20290915"},
	"T-NOTE-5Y":  {"91282CRN3", "5.000", "20260930", "20310930"},
	"T-NOTE-7Y":  {"91282CRM5", "5.000", "20260930", "20330930"},
	"T-NOTE-10Y": {"91282CRF0", "4.625", "20260817", "20360815"},
	"T-BOND-20Y": {"912810UX4", "5.125", "20260831", "20460815"},
	"T-BOND-30Y": {"912810UW6", "5.125", "20260817", "20560815"},
	"TIPS-5Y":    {"91282CQP9", "1.250", "20260430", "20310415"},
	"TIPS-10Y":   {"91282CRE3", "2.375", "20260731", "20360715"},
	"TIPS-30Y":   {"912810US5", "2.375", "20260227", "20560215"},
	// STRIPS interest components; the issue date is their parent's.
	"STRIP-INT-2033": {"912834K80", "0.000", "20260630", "20330630"},
	"STRIP-INT-2056": {"912834ZX9", "0.000", "20260515", "20560515"},
}

// TestOutstandingRealIssuesCarryTheirRealData pins that while a row's real
// Treasury issue is outstanding, messages use its CUSIP, coupon, and dates.
func TestOutstandingRealIssuesCarryTheirRealData(t *testing.T) {
	seen := map[string]bool{}
	for _, tags := range messages(t, time.Date(2026, 10, 6, 14, 0, 0, 0, time.UTC), 1000) {
		want, ok := realIssues[tags[app.TagSymbol]]
		if !ok {
			continue
		}
		seen[tags[app.TagSymbol]] = true
		require.Equal(t, want, [4]string{tags[TagSecurityID], tags[TagCouponRate], tags[TagIssueDate], tags[TagMaturityDate]}, tags[app.TagSymbol])
	}
	require.Len(t, seen, len(realIssues), "every real row was drawn")
}

// TestSyntheticMaturitiesFollowTheTreasuryCalendar pins each term's real
// maturity calendar (TreasuryDirect, 2023-2026 original issues) once its real
// issue has matured: 2-, 5-, and 7-year notes at month-end, 3-year notes on
// the 15th, 10-year notes and 20- and 30-year bonds on Feb/May/Aug/Nov 15,
// 5-year TIPS on Apr/Oct 15, 10-year TIPS on Jan/Jul 15, 30-year TIPS on
// Feb 15, and STRIPS on a bond coupon date (Feb/May/Aug/Nov 15).
func TestSyntheticMaturitiesFollowTheTreasuryCalendar(t *testing.T) {
	quarterly := []time.Month{time.February, time.May, time.August, time.November}
	rules := map[string]struct {
		eom    bool
		months []time.Month
	}{
		"T-NOTE-2Y": {eom: true}, "T-NOTE-5Y": {eom: true}, "T-NOTE-7Y": {eom: true},
		"T-NOTE-3Y":  {},
		"T-NOTE-10Y": {months: quarterly}, "T-BOND-20Y": {months: quarterly}, "T-BOND-30Y": {months: quarterly},
		"TIPS-5Y":  {months: []time.Month{time.April, time.October}},
		"TIPS-10Y": {months: []time.Month{time.January, time.July}},
		"TIPS-30Y": {months: []time.Month{time.February}},
	}
	trade := time.Date(2061, 6, 18, 14, 0, 0, 0, time.UTC) // every real issue has matured
	for _, tags := range messages(t, trade, 1000) {
		sym := tags[app.TagSymbol]
		if strings.HasPrefix(sym, "T-BILL-") {
			continue
		}
		if strings.HasPrefix(sym, "STRIP-INT-") {
			// A 7-year note's coupon strips mature at month-end; a 30-year
			// bond's on the quarterly 15th.
			sym = "T-NOTE-7Y"
			if d, _ := time.Parse("20060102", tags[TagMaturityDate]); d.Day() == 15 {
				sym = "T-BOND-30Y"
			}
		}
		rule, ok := rules[sym]
		require.True(t, ok, "unexpected symbol %s", sym)
		mat, err := time.Parse("20060102", tags[TagMaturityDate])
		require.NoError(t, err)
		if rule.eom {
			require.Equal(t, 1, mat.AddDate(0, 0, 1).Day(), "%s matures at month-end, got %s", sym, mat.Format("20060102"))
		} else {
			require.Equal(t, 15, mat.Day(), "%s matures on the 15th, got %s", sym, mat.Format("20060102"))
		}
		if rule.months != nil {
			require.True(t, slices.Contains(rule.months, mat.Month()), "%s maturity month %s", sym, mat.Month())
		}
	}
}

// TestSyntheticCUSIPsArePrivateUse pins that a synthetic issue's CUSIP uses
// an issue number CUSIP reserves for private use (90-99, 9A-9Y), so it can
// never name a real Treasury, and that every message's CUSIP is either a
// row's real issue or such a private one.
func TestSyntheticCUSIPsArePrivateUse(t *testing.T) {
	private := regexp.MustCompile(`^.{6}9[0-9A-HJ-NP-Y]\d$`)
	r := rand.New(rand.NewSource(1)) // #nosec G404
	for range 10000 {
		require.Regexp(t, private, syntheticCUSIP(r, "91282C"))
	}
	real := map[string]bool{}
	for _, v := range realIssues {
		real[v[0]] = true
	}
	trade := time.Date(2031, 6, 18, 14, 0, 0, 0, time.UTC)
	for _, tags := range messages(t, trade, 500) {
		c := tags[TagSecurityID]
		require.True(t, real[c] || private.MatchString(c), "%s CUSIP %s", tags[app.TagSymbol], c)
	}
}

// TestIssueDatesFollowTheRealIssues pins each row's issue convention to its
// real TreasuryDirect issue: the convention reproduces the real issue date
// from the real maturity (a weekend dated date settles the next weekday, a
// 20-year bond at the dated month's end, TIPS on the month's last weekday),
// the real maturity sits on the row's calendar, and every synthetic issue
// settles on a weekday.
func TestIssueDatesFollowTheRealIssues(t *testing.T) {
	trade := time.Date(2061, 6, 18, 14, 0, 0, 0, time.UTC) // every real issue has matured
	for _, rows := range govBondsByType {
		for _, b := range rows {
			if b.Real == nil || b.TermWeeks > 0 {
				continue
			}
			mat := b.Real.Maturity
			require.Equal(t, b.Real.Issue, issueDate(b, mat), "%s %s issue", b.Symbol, b.Real.Cusip)
			if b.EOM {
				require.Equal(t, 1, mat.AddDate(0, 0, 1).Day(), "%s %s matures at month-end", b.Symbol, b.Real.Cusip)
			} else {
				require.Equal(t, 15, mat.Day(), "%s %s matures on the 15th", b.Symbol, b.Real.Cusip)
			}
			if b.Months != nil {
				require.Contains(t, b.Months, mat.Month(), "%s %s maturity month", b.Symbol, b.Real.Cusip)
			}
			r := rand.New(rand.NewSource(1))
			for range 200 {
				issue, _ := calendarDates(r, trade, b)
				require.NotContains(t, []time.Weekday{time.Saturday, time.Sunday}, issue.Weekday(), "%s issued %s", b.Symbol, issue.Format("2006-01-02"))
			}
		}
	}
}

// TestSyntheticIssuesSettleBeforeTheTrade pins that a synthetic issue is
// outstanding on the trade date by its settlement date, not only its dated
// date: TIPS, 20-year bonds, and weekend-dated issues settle after the dated
// date, so a trade between the two must not get that issue.
func TestSyntheticIssuesSettleBeforeTheTrade(t *testing.T) {
	start := time.Date(2061, 1, 1, 14, 0, 0, 0, time.UTC)
	for _, rows := range govBondsByType {
		for _, b := range rows {
			if b.TermWeeks > 0 {
				continue
			}
			r := rand.New(rand.NewSource(1))
			for d := 0; d < 3*366; d++ {
				trade := start.AddDate(0, 0, d)
				issue, maturity := calendarDates(r, trade, b)
				require.True(t, issue.Before(trade) && maturity.After(trade), "%s traded %s: issued %s, matures %s",
					b.Symbol, trade.Format("2006-01-02"), issue.Format("2006-01-02"), maturity.Format("2006-01-02"))
			}
		}
	}
}
