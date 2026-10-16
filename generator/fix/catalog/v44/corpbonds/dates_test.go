package corpbonds

import (
	"math/rand"
	"testing"
	"time"

	"github.com/observiq/blitz/generator/fix/catalog"
)

// TestDatesFollowTradeDate pins that maturity is after the trade date and issue before it
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
		iss, err := time.Parse("20060102", issueDateField()(r, ctx).Value)
		if err != nil {
			t.Fatal(err)
		}
		if !iss.Before(trade) {
			t.Fatalf("seed %d: issue date %s is not before trade date %s", seed, iss.Format("20060102"), trade.Format("20060102"))
		}
	}
}
