package datagen

import (
	"math/rand"
	"strings"
	"testing"
)

func TestWindowsServices(t *testing.T) {
	if WindowsServices.Len() < 20 {
		t.Errorf("WindowsServices has %d items, want at least 20", WindowsServices.Len())
	}
	// Case-insensitive uniqueness — `Netlogon` and `NetLogon` would refer
	// to the same Windows SCM service.
	seen := make(map[string]string, WindowsServices.Len())
	for _, name := range WindowsServices.All() {
		lower := strings.ToLower(name)
		if prior, ok := seen[lower]; ok {
			t.Errorf("WindowsServices contains case-duplicate names: %q and %q", prior, name)
		}
		seen[lower] = name
	}
	// Spot-check canonical service names that match Microsoft's SCM registrations.
	expected := []string{"SQLSERVERAGENT", "DFSR", "Netlogon"}
	have := make(map[string]bool, WindowsServices.Len())
	for _, n := range WindowsServices.All() {
		have[n] = true
	}
	for _, e := range expected {
		if !have[e] {
			t.Errorf("WindowsServices missing canonical service name %q", e)
		}
	}
}

func TestWindowsServiceDisplayNames(t *testing.T) {
	if WindowsServiceDisplayNames.Len() < 20 {
		t.Errorf("WindowsServiceDisplayNames has %d items, want at least 20", WindowsServiceDisplayNames.Len())
	}
}

func TestWindowsProcessPaths(t *testing.T) {
	if WindowsProcessPaths.Len() < 15 {
		t.Errorf("WindowsProcessPaths has %d items, want at least 15", WindowsProcessPaths.Len())
	}
	r := rand.New(rand.NewSource(42))
	for i := 0; i < 50; i++ {
		path := WindowsProcessPaths.Random(r)
		if !strings.Contains(path, `\`) && !strings.Contains(path, "/") {
			t.Errorf("process path %q should contain a path separator", path)
		}
	}
}

func TestWindowsRegistryPaths(t *testing.T) {
	if WindowsRegistryPaths.Len() < 5 {
		t.Errorf("WindowsRegistryPaths has %d items, want at least 5", WindowsRegistryPaths.Len())
	}
	r := rand.New(rand.NewSource(42))
	for i := 0; i < 20; i++ {
		path := WindowsRegistryPaths.Random(r)
		if !strings.HasPrefix(path, "HK") {
			t.Errorf("registry path %q should start with HK", path)
		}
	}
}

func TestWindowsTaskPaths(t *testing.T) {
	if WindowsTaskPaths.Len() < 5 {
		t.Errorf("WindowsTaskPaths has %d items, want at least 5", WindowsTaskPaths.Len())
	}
	r := rand.New(rand.NewSource(42))
	for i := 0; i < 20; i++ {
		path := WindowsTaskPaths.Random(r)
		if !strings.HasPrefix(path, `\`) {
			t.Errorf("task path %q should start with backslash", path)
		}
	}
}
