package datagen

import (
	"strings"
	"testing"
	"time"
)

// fixedNow is the test reference timestamp. Pinned for reproducible CA validity windows.
var fixedNow = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

func TestGenerateDomainIdentity(t *testing.T) {
	t.Run("default domain name", func(t *testing.T) {
		d := GenerateDomainIdentity(42, "", fixedNow)
		if d.Name != "blitz.local" {
			t.Errorf("expected default domain name 'blitz.local', got %q", d.Name)
		}
		if d.NetBIOSName != "BLITZ" {
			t.Errorf("expected NetBIOS name 'BLITZ', got %q", d.NetBIOSName)
		}
	})

	t.Run("custom domain name", func(t *testing.T) {
		d := GenerateDomainIdentity(42, "contoso.com", fixedNow)
		if d.Name != "contoso.com" {
			t.Errorf("expected domain name 'contoso.com', got %q", d.Name)
		}
		if d.NetBIOSName != "CONTOSO" {
			t.Errorf("expected NetBIOS name 'CONTOSO', got %q", d.NetBIOSName)
		}
	})

	t.Run("forest name equals domain name", func(t *testing.T) {
		d := GenerateDomainIdentity(42, "example.org", fixedNow)
		if d.ForestName != d.Name {
			t.Errorf("ForestName %q should equal Name %q", d.ForestName, d.Name)
		}
	})

	t.Run("domain SID format", func(t *testing.T) {
		d := GenerateDomainIdentity(42, "", fixedNow)
		if !strings.HasPrefix(d.DomainSID, "S-1-5-21-") {
			t.Errorf("DomainSID %q should start with S-1-5-21-", d.DomainSID)
		}
	})

	t.Run("has sites", func(t *testing.T) {
		d := GenerateDomainIdentity(42, "", fixedNow)
		if len(d.Sites) < 1 {
			t.Error("expected at least 1 site")
		}
		if d.Sites[0] != "Default-First-Site-Name" {
			t.Errorf("first site should be 'Default-First-Site-Name', got %q", d.Sites[0])
		}
	})

	t.Run("has CA", func(t *testing.T) {
		d := GenerateDomainIdentity(42, "contoso.com", fixedNow)
		if d.CA == nil {
			t.Fatal("expected CA to be set")
		}
		if !strings.Contains(d.CA.CommonName, "contoso") {
			t.Errorf("CA CommonName %q should contain 'contoso'", d.CA.CommonName)
		}
		if d.CA.ValidTo.Before(d.CA.ValidFrom) {
			t.Error("CA ValidTo should be after ValidFrom")
		}
		if len(d.CA.Thumbprint) != 40 {
			t.Errorf("CA Thumbprint should be 40 hex chars, got %d", len(d.CA.Thumbprint))
		}
		if len(d.CA.SerialNumber) == 0 {
			t.Error("CA SerialNumber should not be empty")
		}
	})

	t.Run("CRL distribution point includes full DC chain", func(t *testing.T) {
		d := GenerateDomainIdentity(42, "contoso.com", fixedNow)
		want := "DC=contoso,DC=com"
		if !strings.Contains(d.CA.CRLDistPoint, want) {
			t.Errorf("CRLDistPoint %q should contain %q", d.CA.CRLDistPoint, want)
		}
	})

	t.Run("CA validity window pinned to supplied now", func(t *testing.T) {
		d := GenerateDomainIdentity(42, "contoso.com", fixedNow)
		wantFrom := fixedNow.AddDate(-5, 0, 0)
		wantTo := fixedNow.AddDate(5, 0, 0)
		if !d.CA.ValidFrom.Equal(wantFrom) {
			t.Errorf("ValidFrom = %v, want %v", d.CA.ValidFrom, wantFrom)
		}
		if !d.CA.ValidTo.Equal(wantTo) {
			t.Errorf("ValidTo = %v, want %v", d.CA.ValidTo, wantTo)
		}
	})

	t.Run("deterministic from seed and now", func(t *testing.T) {
		d1 := GenerateDomainIdentity(99, "test.local", fixedNow)
		d2 := GenerateDomainIdentity(99, "test.local", fixedNow)
		if d1.DomainSID != d2.DomainSID {
			t.Error("same seed should produce same DomainSID")
		}
		if d1.CA.Thumbprint != d2.CA.Thumbprint {
			t.Error("same seed should produce same CA Thumbprint")
		}
		if !d1.CA.ValidFrom.Equal(d2.CA.ValidFrom) {
			t.Error("same now should produce same CA ValidFrom")
		}
	})
}

func TestCertAuthorityDates(t *testing.T) {
	d := GenerateDomainIdentity(42, "", fixedNow)
	if d.CA.ValidFrom.After(fixedNow) {
		t.Error("CA ValidFrom should be in the past relative to now")
	}
	if d.CA.ValidTo.Before(fixedNow) {
		t.Error("CA ValidTo should be in the future relative to now")
	}
}

func TestDomainToDC(t *testing.T) {
	cases := map[string]string{
		"contoso.com":         "DC=contoso,DC=com",
		"sub.contoso.com":     "DC=sub,DC=contoso,DC=com",
		"blitz.local":         "DC=blitz,DC=local",
		"single":              "DC=single",
		"a.b.c.d.example.org": "DC=a,DC=b,DC=c,DC=d,DC=example,DC=org",
	}
	for input, want := range cases {
		if got := domainToDC(input); got != want {
			t.Errorf("domainToDC(%q) = %q, want %q", input, got, want)
		}
	}
}
