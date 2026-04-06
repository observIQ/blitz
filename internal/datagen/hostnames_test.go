package datagen

import (
	"math/rand"
	"strings"
	"testing"
)

func TestMythologyPoolSizes(t *testing.T) {
	pools := map[string]*Pool[string]{
		"Norse":    NorseNames,
		"Greek":    GreekNames,
		"Roman":    RomanNames,
		"Egyptian": EgyptianNames,
		"Celtic":   CelticNames,
	}
	for name, p := range pools {
		if p.Len() < 25 {
			t.Errorf("%s pool has %d names, want at least 25", name, p.Len())
		}
	}
}

func TestAllMythologyNames(t *testing.T) {
	expected := NorseNames.Len() + GreekNames.Len() + RomanNames.Len() +
		EgyptianNames.Len() + CelticNames.Len()
	if AllMythologyNames.Len() != expected {
		t.Errorf("AllMythologyNames has %d names, want %d", AllMythologyNames.Len(), expected)
	}
}

func TestRolePool(t *testing.T) {
	if Roles.Len() < 10 {
		t.Errorf("Roles pool has %d items, want at least 10", Roles.Len())
	}
}

func TestGenerateHostname(t *testing.T) {
	r := rand.New(rand.NewSource(42))

	t.Run("linux style", func(t *testing.T) {
		h := GenerateHostname(r, StyleLinux, NorseNames)
		// Should match pattern: name-role-nn
		parts := strings.Split(h, "-")
		if len(parts) != 3 {
			t.Errorf("linux hostname %q should have 3 dash-separated parts", h)
		}
		if h != strings.ToLower(h) {
			t.Errorf("linux hostname %q should be lowercase", h)
		}
	})

	t.Run("windows style", func(t *testing.T) {
		h := GenerateHostname(r, StyleWindows, RomanNames)
		// Should match pattern: NAME-ROLENN
		if h != strings.ToUpper(h) {
			t.Errorf("windows hostname %q should be uppercase", h)
		}
		if !strings.Contains(h, "-") {
			t.Errorf("windows hostname %q should contain a dash", h)
		}
	})

	t.Run("dc style", func(t *testing.T) {
		h := GenerateHostname(r, StyleDC, GreekNames)
		// Should match pattern: NAME-DCnn
		if h != strings.ToUpper(h) {
			t.Errorf("dc hostname %q should be uppercase", h)
		}
		if !strings.Contains(h, "-DC") {
			t.Errorf("dc hostname %q should contain -DC", h)
		}
	})

	t.Run("nil names defaults to AllMythologyNames", func(t *testing.T) {
		h := GenerateHostname(r, StyleLinux, nil)
		if h == "" {
			t.Error("hostname should not be empty with nil names pool")
		}
	})
}

func TestGenerateHostnames(t *testing.T) {
	t.Run("deterministic with same seed", func(t *testing.T) {
		h1 := GenerateHostnames(42, 5, StyleLinux, NorseNames)
		h2 := GenerateHostnames(42, 5, StyleLinux, NorseNames)
		if len(h1) != len(h2) {
			t.Fatalf("different lengths: %d vs %d", len(h1), len(h2))
		}
		for i := range h1 {
			if h1[i] != h2[i] {
				t.Errorf("hostname[%d]: %q != %q", i, h1[i], h2[i])
			}
		}
	})

	t.Run("different seeds produce different hostnames", func(t *testing.T) {
		h1 := GenerateHostnames(1, 10, StyleLinux, NorseNames)
		h2 := GenerateHostnames(2, 10, StyleLinux, NorseNames)
		allSame := true
		for i := range h1 {
			if h1[i] != h2[i] {
				allSame = false
				break
			}
		}
		if allSame {
			t.Error("different seeds produced identical hostnames")
		}
	})

	t.Run("requested count is honored", func(t *testing.T) {
		h := GenerateHostnames(42, 7, StyleWindows, RomanNames)
		if len(h) != 7 {
			t.Errorf("expected 7 hostnames, got %d", len(h))
		}
	})
}

func TestHostnameFormats(t *testing.T) {
	r := rand.New(rand.NewSource(1))

	// Generate a bunch and check they aren't empty
	for i := 0; i < 50; i++ {
		for _, style := range []HostnameStyle{StyleLinux, StyleWindows, StyleDC} {
			h := GenerateHostname(r, style, AllMythologyNames)
			if h == "" {
				t.Errorf("empty hostname for style %d", style)
			}
			if len(h) > 30 {
				t.Errorf("hostname %q is too long (%d chars)", h, len(h))
			}
		}
	}
}
