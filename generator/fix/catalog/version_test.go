package catalog

import "testing"

func TestVersionBeginString(t *testing.T) {
	cases := []struct {
		v    Version
		want string
	}{
		{V42, "FIX.4.2"},
		{V44, "FIX.4.4"},
		{V50SP2, "FIXT.1.1"},
		{VersionUnknown, ""},
	}
	for _, c := range cases {
		if got := c.v.BeginString(); got != c.want {
			t.Errorf("Version(%d).BeginString() = %q, want %q", c.v, got, c.want)
		}
	}
}

func TestVersionApplVerID(t *testing.T) {
	if V42.ApplVerID() != "" {
		t.Errorf("V42 must not carry ApplVerID")
	}
	if V44.ApplVerID() != "" {
		t.Errorf("V44 must not carry ApplVerID")
	}
	if V50SP2.ApplVerID() != "9" {
		t.Errorf("V50SP2.ApplVerID() = %q, want %q", V50SP2.ApplVerID(), "9")
	}
}

func TestAllVersionsCovered(t *testing.T) {
	vs := AllVersions()
	if len(vs) != 3 {
		t.Fatalf("AllVersions() = %d entries, want 3", len(vs))
	}
	for _, v := range vs {
		if v.BeginString() == "" {
			t.Errorf("version %v missing BeginString", v)
		}
		if v.String() == "unknown" {
			t.Errorf("version %v missing String() label", v)
		}
	}
}
