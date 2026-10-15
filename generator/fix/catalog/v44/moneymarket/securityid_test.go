package moneymarket

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
	for st, rows := range instrumentsByType {
		for _, in := range rows {
			if !catalog.ValidSecurityID(in.Cusip, catalog.SecurityIDSourceCUSIP) {
				t.Errorf("%s %s: invalid SecurityID %q", st, in.Symbol, in.Cusip)
			}
		}
	}
	in := pickInstrument(rand.New(rand.NewSource(1)), catalog.SecurityType("NONE")) // #nosec G404
	if !catalog.ValidSecurityID(in.Cusip, catalog.SecurityIDSourceCUSIP) {
		t.Errorf("fallback %s: invalid SecurityID %q", in.Symbol, in.Cusip)
	}
}

// TestCUSIPsArePrivateUse pins that the illustrative instruments carry an
// issue number CUSIP reserves for private use (90-99, 9A-9Y), so no
// SecurityID names a real security.
func TestCUSIPsArePrivateUse(t *testing.T) {
	for st, rows := range instrumentsByType {
		for _, in := range rows {
			if in.Cusip[6] != '9' || in.Cusip[7] == 'Z' {
				t.Errorf("%s %s: CUSIP %q has no private-use issue number", st, in.Symbol, in.Cusip)
			}
		}
	}
}
