package fix

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/observiq/blitz/embed"
	"github.com/observiq/blitz/generator/fix/catalog"
)

// testStart is a Tuesday, so T+1 and T+2 stay in the same week and a
// forward date crosses at least one weekend.
var testStart = time.Date(2026, 9, 29, 14, 30, 0, 0, time.UTC)

// fixFields parses one tag=value<SOH> FIX message into a tag map.
func fixFields(msg []byte) map[string]string {
	out := make(map[string]string)
	for _, kv := range bytes.Split(msg, []byte{0x01}) {
		if k, v, ok := bytes.Cut(kv, []byte("=")); ok {
			out[string(k)] = string(v)
		}
	}
	return out
}

// runFIX emits at least n messages for the categories and returns them parsed.
func runFIX(t *testing.T, n int, cats ...catalog.AssetCategory) []map[string]string {
	t.Helper()
	cons := &captureConsumer{}
	g, err := New(zap.NewNop(), Config{
		Workers: 1, Rate: time.Millisecond, Version: catalog.V44, Seed: 99,
		EnabledCategories: cats, Start: testStart,
	}, cons, embed.NopTelemetry())
	require.NoError(t, err)
	require.NoError(t, g.Start(context.Background()))
	require.Eventually(t, func() bool { return cons.Count() >= n }, 5*time.Second, 5*time.Millisecond)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	require.NoError(t, g.Stop(ctx))
	var out []map[string]string
	for _, m := range cons.Snapshot() {
		out = append(out, fixFields(m))
	}
	return out
}

func parseDate(t *testing.T, v string) time.Time {
	t.Helper()
	d, err := time.Parse("20060102", v)
	require.NoError(t, err, "LocalMktDate %q", v)
	return d
}

func businessDaysAfter(d time.Time, n int) time.Time {
	for n > 0 {
		d = d.AddDate(0, 0, 1)
		if d.Weekday() != time.Saturday && d.Weekday() != time.Sunday {
			n--
		}
	}
	return d
}

// TestSendingTimeAnchoredToStart: tag 52 (and 60 where present) is a FIX
// UTCTimestamp at the configured start, advancing with the session.
func TestSendingTimeAnchoredToStart(t *testing.T) {
	var prev time.Time
	for i, f := range runFIX(t, 20, catalog.AssetCategoryEquities) {
		st, err := time.Parse("20060102-15:04:05.000", f["52"])
		require.NoError(t, err, "msg %d SendingTime %q", i, f["52"])
		require.False(t, st.Before(testStart), "msg %d SendingTime %v before start", i, st)
		require.True(t, st.Before(testStart.Add(time.Hour)), "msg %d SendingTime %v far after start", i, st)
		require.False(t, st.Before(prev), "msg %d SendingTime went backwards", i)
		prev = st
		if tt, ok := f["60"]; ok {
			require.Equal(t, f["52"], tt, "msg %d TransactTime != SendingTime", i)
		}
	}
}

// TestFXSettlDateMatchesSettlType: SettlDate (64) is the business day
// implied by SettlType (63) from the trade date, never a weekend.
func TestFXSettlDateMatchesSettlType(t *testing.T) {
	trade := time.Date(2026, 9, 29, 0, 0, 0, 0, time.UTC)
	seen := 0
	for i, f := range runFIX(t, 60, catalog.AssetCategoryFX) {
		st, sd := f["63"], f["64"]
		if sd == "" {
			continue
		}
		seen++
		d := parseDate(t, sd)
		require.NotContains(t, []time.Weekday{time.Saturday, time.Sunday}, d.Weekday(), "msg %d SettlDate %s on a weekend", i, sd)
		switch st {
		case "1":
			require.Equal(t, trade, d, "msg %d cash settles T+0", i)
		case "2":
			require.Equal(t, businessDaysAfter(trade, 1), d, "msg %d next-day settles T+1", i)
		case "0":
			lag := 2
			if f["55"] == "USD/CAD" {
				lag = 1
			}
			require.Equal(t, businessDaysAfter(trade, lag), d, "msg %d %s spot settles T+%d", i, f["55"], lag)
		case "6":
			require.NotEqual(t, "FOR", f["167"], "msg %d spot FX with forward SettlType", i)
			require.True(t, d.After(businessDaysAfter(trade, 2)), "msg %d forward %s must settle after spot", i, sd)
		default:
			t.Fatalf("msg %d unexpected SettlType %q", i, st)
		}
	}
	require.Positive(t, seen, "no FX message carried SettlDate")
}

// TestRepoAndMoneyMarketDatesFollowTradeDate: repo legs and money-market
// issue/maturity dates sit on the right side of the trade date.
func TestRepoAndMoneyMarketDatesFollowTradeDate(t *testing.T) {
	trade := time.Date(2026, 9, 29, 0, 0, 0, 0, time.UTC)
	repo, mm := 0, 0
	for i, f := range runFIX(t, 80, catalog.AssetCategoryRepos, catalog.AssetCategoryMoneyMarket) {
		if near, ok := f["64"]; ok {
			repo++
			n := parseDate(t, near)
			require.False(t, n.Before(trade), "msg %d repo near leg %s before trade date", i, near)
			if far, ok := f["193"]; ok {
				require.True(t, parseDate(t, far).After(n), "msg %d repo far leg %s not after near leg %s", i, far, near)
			}
		}
		if iss, ok := f["225"]; ok {
			mm++
			require.False(t, parseDate(t, iss).After(trade), "msg %d IssueDate %s after trade date", i, iss)
			if mat, ok := f["541"]; ok {
				require.True(t, parseDate(t, mat).After(trade), "msg %d MaturityDate %s not after trade date", i, mat)
			}
		}
	}
	require.Positive(t, repo, "no repo SettlDate seen")
	require.Positive(t, mm, "no money-market IssueDate seen")
}
