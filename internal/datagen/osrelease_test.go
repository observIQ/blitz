package datagen

import (
	"math/rand"
	"regexp"
	"strings"
	"testing"
)

func TestGenerateOSInfo_Coherent(t *testing.T) {
	for _, os := range []OSType{OSLinux, OSWindows, OSMacOS} {
		r := rand.New(rand.NewSource(1)) // #nosec G404
		info := GenerateOSInfo(r, os)
		if info.Type != os {
			t.Errorf("%s: Type = %q, want %q", os, info.Type, os)
		}
		if info.Name == "" || info.Version == "" || info.BuildID == "" || info.Description == "" {
			t.Errorf("%s: incomplete OSInfo: %+v", os, info)
		}
	}
}

func TestGenerateOSInfo_Deterministic(t *testing.T) {
	a := GenerateOSInfo(rand.New(rand.NewSource(7)), OSWindows) // #nosec G404
	b := GenerateOSInfo(rand.New(rand.NewSource(7)), OSWindows) // #nosec G404
	if a != b {
		t.Errorf("GenerateOSInfo not deterministic: %+v vs %+v", a, b)
	}
}

func TestGenerateOSInfo_WindowsUBR(t *testing.T) {
	// Windows os.description is the ver-string carrying a real UBR, and
	// os.build_id is the bare build number.
	info := GenerateOSInfo(rand.New(rand.NewSource(3)), OSWindows) // #nosec G404
	if !strings.HasPrefix(info.Description, "Microsoft Windows [Version 10.0.") {
		t.Errorf("Windows description = %q, want ver-string form", info.Description)
	}
	if !regexp.MustCompile(`^\d+$`).MatchString(info.BuildID) {
		t.Errorf("Windows build_id = %q, want bare build number", info.BuildID)
	}
	// The build number appears in the version quad.
	if !strings.Contains(info.Version, info.BuildID) {
		t.Errorf("Windows version %q should contain build_id %q", info.Version, info.BuildID)
	}
}

func TestGenerateOSInfo_MacOS(t *testing.T) {
	info := GenerateOSInfo(rand.New(rand.NewSource(2)), OSMacOS) // #nosec G404
	if info.Name != "macOS" {
		t.Errorf("macOS name = %q, want macOS", info.Name)
	}
	// Description is "macOS <version> (<build>)".
	want := "macOS " + info.Version + " (" + info.BuildID + ")"
	if info.Description != want {
		t.Errorf("macOS description = %q, want %q", info.Description, want)
	}
}

func TestPickHalfIndex(t *testing.T) {
	r := rand.New(rand.NewSource(1)) // #nosec G404
	for i := 0; i < 30; i++ {
		if idx := pickHalfIndex(r, 10, true); idx < 0 || idx >= 5 {
			t.Fatalf("older index %d out of [0,5)", idx)
		}
		if idx := pickHalfIndex(r, 10, false); idx < 5 || idx >= 10 {
			t.Fatalf("newer index %d out of [5,10)", idx)
		}
	}
	// Single-element pool: the guard avoids Intn(0); both halves resolve to 0.
	if pickHalfIndex(r, 1, true) != 0 {
		t.Error("n=1 older should be 0")
	}
	if pickHalfIndex(r, 1, false) != 0 {
		t.Error("n=1 newer should be 0")
	}
}

func TestGenerateHostID(t *testing.T) {
	linuxRE := regexp.MustCompile(`^[0-9a-f]{32}$`)
	uuidLowerRE := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	uuidUpperRE := regexp.MustCompile(`^[0-9A-F]{8}-[0-9A-F]{4}-[0-9A-F]{4}-[0-9A-F]{4}-[0-9A-F]{12}$`)

	linux := GenerateHostID(rand.New(rand.NewSource(1)), OSLinux) // #nosec G404
	if !linuxRE.MatchString(linux) {
		t.Errorf("Linux host.id = %q, want 32-char lowercase hex", linux)
	}
	win := GenerateHostID(rand.New(rand.NewSource(1)), OSWindows) // #nosec G404
	if !uuidLowerRE.MatchString(win) {
		t.Errorf("Windows host.id = %q, want GUID", win)
	}
	mac := GenerateHostID(rand.New(rand.NewSource(1)), OSMacOS) // #nosec G404
	if !uuidUpperRE.MatchString(mac) {
		t.Errorf("macOS host.id = %q, want uppercase UUID", mac)
	}

	// Deterministic.
	if GenerateHostID(rand.New(rand.NewSource(9)), OSLinux) != GenerateHostID(rand.New(rand.NewSource(9)), OSLinux) { // #nosec G404
		t.Error("GenerateHostID not deterministic")
	}
}
