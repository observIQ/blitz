package datagen

import (
	"math/rand"
	"strings"
	"testing"
)

func TestGenerateApplicationsForSystem(t *testing.T) {
	t.Run("windows server apps", func(t *testing.T) {
		r := rand.New(rand.NewSource(42))
		apps := GenerateApplicationsForSystem(r, OSWindows, RoleServer, "SRV01", nil)
		if len(apps) < 2 {
			t.Errorf("expected at least 2 apps, got %d", len(apps))
		}
		for _, a := range apps {
			if a.SystemRef != "SRV01" {
				t.Errorf("expected SystemRef 'SRV01', got %q", a.SystemRef)
			}
			if a.Name == "" {
				t.Error("app Name should not be empty")
			}
			if a.Version == "" {
				t.Error("app Version should not be empty")
			}
			if a.Vendor == "" {
				t.Error("app Vendor should not be empty")
			}
		}
	})

	t.Run("linux server apps", func(t *testing.T) {
		r := rand.New(rand.NewSource(42))
		apps := GenerateApplicationsForSystem(r, OSLinux, RoleServer, "srv01", nil)
		if len(apps) < 2 {
			t.Errorf("expected at least 2 apps, got %d", len(apps))
		}
	})

	t.Run("DC apps", func(t *testing.T) {
		r := rand.New(rand.NewSource(42))
		apps := GenerateApplicationsForSystem(r, OSWindows, RoleDC, "DC01", nil)
		if len(apps) < 2 {
			t.Errorf("expected at least 2 apps, got %d", len(apps))
		}
	})

	t.Run("windows workstation apps come from windows pool", func(t *testing.T) {
		r := rand.New(rand.NewSource(42))
		apps := GenerateApplicationsForSystem(r, OSWindows, RoleWorkstation, "WS01", nil)
		if len(apps) < 2 {
			t.Errorf("expected at least 2 apps, got %d", len(apps))
		}
		valid := poolNames(windowsWorkstationApps)
		for _, a := range apps {
			if !valid[a.Name] {
				t.Errorf("windows workstation app %q not in windowsWorkstationApps pool", a.Name)
			}
		}
	})

	t.Run("linux workstation apps come from linux pool and have unix paths", func(t *testing.T) {
		r := rand.New(rand.NewSource(42))
		apps := GenerateApplicationsForSystem(r, OSLinux, RoleWorkstation, "tux01", nil)
		if len(apps) < 2 {
			t.Errorf("expected at least 2 apps, got %d", len(apps))
		}
		valid := poolNames(linuxWorkstationApps)
		for _, a := range apps {
			if !valid[a.Name] {
				t.Errorf("linux workstation app %q not in linuxWorkstationApps pool", a.Name)
			}
			if strings.HasPrefix(a.InstallPath, `C:\`) {
				t.Errorf("linux workstation app %q has Windows install path %q", a.Name, a.InstallPath)
			}
		}
	})

	t.Run("macos workstation apps come from macos pool and have .app paths", func(t *testing.T) {
		r := rand.New(rand.NewSource(42))
		apps := GenerateApplicationsForSystem(r, OSMacOS, RoleWorkstation, "mac01", nil)
		if len(apps) < 2 {
			t.Errorf("expected at least 2 apps, got %d", len(apps))
		}
		valid := poolNames(macosWorkstationApps)
		for _, a := range apps {
			if !valid[a.Name] {
				t.Errorf("macos workstation app %q not in macosWorkstationApps pool", a.Name)
			}
			if strings.HasPrefix(a.InstallPath, `C:\`) {
				t.Errorf("macos workstation app %q has Windows install path %q", a.Name, a.InstallPath)
			}
		}
	})

	t.Run("unsupported combination returns empty slice", func(t *testing.T) {
		r := rand.New(rand.NewSource(42))
		// macOS + Server is not modeled (would warn in production).
		apps := GenerateApplicationsForSystem(r, OSMacOS, RoleServer, "macsrv", nil)
		if len(apps) != 0 {
			t.Errorf("unsupported os/role expected empty slice, got %d apps", len(apps))
		}
	})

	t.Run("deterministic", func(t *testing.T) {
		r1 := rand.New(rand.NewSource(99))
		r2 := rand.New(rand.NewSource(99))
		a1 := GenerateApplicationsForSystem(r1, OSWindows, RoleServer, "T", nil)
		a2 := GenerateApplicationsForSystem(r2, OSWindows, RoleServer, "T", nil)
		if len(a1) != len(a2) {
			t.Fatalf("different lengths: %d vs %d", len(a1), len(a2))
		}
		for i := range a1 {
			if a1[i].Name != a2[i].Name {
				t.Errorf("app[%d]: %q vs %q", i, a1[i].Name, a2[i].Name)
			}
		}
	})
}

func poolNames(pool []appTemplate) map[string]bool {
	m := make(map[string]bool, len(pool))
	for _, t := range pool {
		m[t.name] = true
	}
	return m
}
