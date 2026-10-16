package catalog

import "testing"

// TestValidSecurityID pins each check-digit scheme against real identifiers
// (Apple's CUSIP and ISIN, BAE Systems' SEDOL) and rejects a wrong check
// digit, a wrong length, and characters outside each scheme.
func TestValidSecurityID(t *testing.T) {
	for _, tc := range []struct {
		id, source string
		want       bool
	}{
		{"037833100", SecurityIDSourceCUSIP, true},
		{"91282CRP8", SecurityIDSourceCUSIP, true},
		{"037833101", SecurityIDSourceCUSIP, false},
		{"03783310", SecurityIDSourceCUSIP, false},
		{"0378-3100", SecurityIDSourceCUSIP, false},
		{"0263494", SecurityIDSourceSEDOL, true},
		{"0263495", SecurityIDSourceSEDOL, false},
		{"026349A", SecurityIDSourceSEDOL, false},
		{"A263494", SecurityIDSourceSEDOL, false},
		{"US0378331005", SecurityIDSourceISIN, true},
		{"US0378331006", SecurityIDSourceISIN, false},
		{"1S0378331005", SecurityIDSourceISIN, false},
		{"REPO-UST-ON1", SecurityIDSourceISIN, false},
		{"anything", "8", true},
	} {
		if got := ValidSecurityID(tc.id, tc.source); got != tc.want {
			t.Errorf("ValidSecurityID(%q, %q) = %v, want %v", tc.id, tc.source, got, tc.want)
		}
	}
}

// TestAlnumValue pins the character values the check digits use: digits
// 0-9, letters 10-35, and CUSIP's *, @, # as 36-38; anything else is 0.
func TestAlnumValue(t *testing.T) {
	for c, want := range map[byte]int{'0': 0, '9': 9, 'A': 10, 'Z': 35, '*': 36, '@': 37, '#': 38, 'a': 0} {
		if got := alnumValue(c); got != want {
			t.Errorf("alnumValue(%q) = %d, want %d", c, got, want)
		}
	}
}
