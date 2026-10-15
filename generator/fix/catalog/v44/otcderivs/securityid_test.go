package otcderivs

import (
	"math/rand"
	"testing"

	"github.com/observiq/blitz/generator/fix/catalog"
)

// TestSecurityIDsAreValid pins that every swap's SecurityID is a well-formed
// ISIN with the OTC-derivative "EZ" prefix, check digit included, declared
// as an ISIN; FIX 4.4 has no Markit RED SecurityIDSource. The empty-table
// fallback is held to the same rule.
func TestSecurityIDsAreValid(t *testing.T) {
	check := func(label string, sw swap) {
		if sw.IDSource != catalog.SecurityIDSourceISIN || sw.ID[:2] != "EZ" || !catalog.ValidSecurityID(sw.ID, sw.IDSource) {
			t.Errorf("%s %s: SecurityID %q (source %q) is not a valid EZ ISIN", label, sw.Symbol, sw.ID, sw.IDSource)
		}
	}
	for st, rows := range swapsByType {
		for _, sw := range rows {
			check(string(st), sw)
		}
	}
	check("fallback", pickSwap(rand.New(rand.NewSource(1)), catalog.SecurityType("NONE"))) // #nosec G404
}
