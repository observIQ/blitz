package equities

import (
	"math/rand"
	"slices"
	"strings"
	"testing"

	"github.com/observiq/blitz/generator/fix/catalog"
)

// TestSecurityIDsAreValid pins that every row's SecurityID is well formed for
// its SecurityIDSource, check digit included, so a consumer that validates
// identifiers accepts every message. The empty-table fallback is held to the
// same rule.
func TestSecurityIDsAreValid(t *testing.T) {
	for st, rows := range equityInstruments {
		for _, in := range rows {
			if !catalog.ValidSecurityID(in.ID, in.IDSource) {
				t.Errorf("%s %s: invalid SecurityID %q", st, in.Symbol, in.ID)
			}
		}
	}
	in := pickInstrument(rand.New(rand.NewSource(1)), catalog.SecurityType("NONE")) // #nosec G404
	if !catalog.ValidSecurityID(in.ID, in.IDSource) {
		t.Errorf("fallback %s: invalid SecurityID %q", in.Symbol, in.ID)
	}
}

// realCUSIPs are the listed securities' CUSIPs, checked against the issuers'
// filings and fund pages (QuantumOnline for the preferreds) on 2026-10-12.
var realCUSIPs = map[string]string{
	"AAPL": "037833100", "MSFT": "594918104", "GOOGL": "02079K305", "AMZN": "023135106",
	"JPM": "46625H100", "JNJ": "478160104", "V": "92826C839", "MA": "57636Q104",
	"BAC.PB": "060505229", "JPM.PD": "48128B655", "GS.PA": "38143Y665", "WFC.PL": "949746804",
	"SPY": "78462F103", "IVV": "464287200", "VOO": "922908363", "QQQ": "46090E103",
	"DIA": "78467X109", "IWM": "464287655",
	"VFINX": "922908108", "FXAIX": "315911750", "VTSAX": "922908728", "SWPPX": "808509855",
	"BABA": "01609W102", "TSM": "874039100", "NVS": "66987V109", "BP": "055622104",
}

// TestSymbolsMatchTheirSecurityID pins that a listed symbol carries its real
// CUSIP, and that any other row is an unlisted Nasdaq test symbol with a
// private-use issue number (90-99, 9A-9Y), so no row pairs a real symbol with
// another security's identifier.
func TestSymbolsMatchTheirSecurityID(t *testing.T) {
	for st, rows := range equityInstruments {
		for _, in := range rows {
			if want, ok := realCUSIPs[in.Symbol]; ok {
				if in.ID != want {
					t.Errorf("%s %s: CUSIP %q, want %q", st, in.Symbol, in.ID, want)
				}
				continue
			}
			base, _, _ := strings.Cut(in.Symbol, ".")
			if !slices.Contains([]string{"ZVZZT", "ZWZZT", "ZXZZT", "ZBZZT"}, base) {
				t.Errorf("%s %s: not a listed symbol with a known CUSIP, nor a Nasdaq test symbol", st, in.Symbol)
			}
			if in.ID[6] != '9' || in.ID[7] == 'Z' {
				t.Errorf("%s %s: synthetic CUSIP %q has no private-use issue number", st, in.Symbol, in.ID)
			}
		}
	}
}
