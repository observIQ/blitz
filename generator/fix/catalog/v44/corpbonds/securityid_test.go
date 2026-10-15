package corpbonds

import (
	"math/rand"
	"testing"

	"github.com/observiq/blitz/generator/fix/catalog"
)

// TestSecurityIDsAreValid pins that every row's SecurityID is well formed for
// its SecurityIDSource, check digit included, so a consumer that validates
// identifiers accepts every message. The empty-table fallback is held to the
// same rule.
func TestSecurityIDsAreValid(t *testing.T) {
	for st, rows := range corpBondsByType {
		for _, b := range rows {
			if !catalog.ValidSecurityID(b.Cusip, catalog.SecurityIDSourceCUSIP) {
				t.Errorf("%s %s: invalid SecurityID %q", st, b.Symbol, b.Cusip)
			}
		}
	}
	b := pickBond(rand.New(rand.NewSource(1)), catalog.SecurityType("NONE")) // #nosec G404
	if !catalog.ValidSecurityID(b.Cusip, catalog.SecurityIDSourceCUSIP) {
		t.Errorf("fallback %s: invalid SecurityID %q", b.Symbol, b.Cusip)
	}
}
