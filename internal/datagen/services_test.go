package datagen

import (
	"math/rand"
	"testing"
)

func TestGenerateServicesForSystem(t *testing.T) {
	t.Run("windows server gets windows services", func(t *testing.T) {
		r := rand.New(rand.NewSource(42))
		services := GenerateServicesForSystem(r, OSWindows, RoleServer, "MARS-WEB01")
		if len(services) < 3 {
			t.Errorf("expected at least 3 services, got %d", len(services))
		}
		for _, s := range services {
			if s.SystemRef != "MARS-WEB01" {
				t.Errorf("expected SystemRef 'MARS-WEB01', got %q", s.SystemRef)
			}
			if s.Name == "" {
				t.Error("service Name should not be empty")
			}
			if s.DisplayName == "" {
				t.Error("service DisplayName should not be empty")
			}
		}
	})

	t.Run("linux server gets linux services", func(t *testing.T) {
		r := rand.New(rand.NewSource(42))
		services := GenerateServicesForSystem(r, OSLinux, RoleServer, "thor-web-01")
		if len(services) < 3 {
			t.Errorf("expected at least 3 services, got %d", len(services))
		}
	})

	t.Run("DC always emits the full dcServices set", func(t *testing.T) {
		r := rand.New(rand.NewSource(42))
		services := GenerateServicesForSystem(r, OSWindows, RoleDC, "ZEUS-DC01")
		if len(services) != len(dcServices) {
			t.Errorf("DC should emit all %d dcServices, got %d", len(dcServices), len(services))
		}
		// AD-critical services must always be present.
		required := []string{"NTDS", "DNS", "KDC", "Netlogon"}
		have := make(map[string]bool, len(services))
		for _, s := range services {
			have[s.Name] = true
		}
		for _, name := range required {
			if !have[name] {
				t.Errorf("DC missing AD-critical service %q", name)
			}
		}
	})

	t.Run("workstation services come from workstation template pool", func(t *testing.T) {
		r := rand.New(rand.NewSource(42))
		services := GenerateServicesForSystem(r, OSWindows, RoleWorkstation, "WS01")
		valid := make(map[string]bool, len(windowsWorkstationServices))
		for _, tmpl := range windowsWorkstationServices {
			valid[tmpl.name] = true
		}
		for _, s := range services {
			if !valid[s.Name] {
				t.Errorf("workstation got service %q not in windowsWorkstationServices pool", s.Name)
			}
		}
	})

	t.Run("non-DC roles can produce subsetted output", func(t *testing.T) {
		// Across many seeds, the windowsServerServices template (>5 entries)
		// should emit fewer than len(windowsServerServices) for at least one
		// seed — proving subset behavior is retained for non-DC roles.
		fullSize := len(windowsServerServices)
		sawSubset := false
		for seed := int64(1); seed <= 50 && !sawSubset; seed++ {
			r := rand.New(rand.NewSource(seed))
			services := GenerateServicesForSystem(r, OSWindows, RoleServer, "SRV01")
			if len(services) < fullSize {
				sawSubset = true
			}
		}
		if !sawSubset {
			t.Errorf("expected at least one seed in [1,50] to produce a subset of windowsServerServices (full=%d)", fullSize)
		}
	})

	t.Run("deterministic", func(t *testing.T) {
		r1 := rand.New(rand.NewSource(99))
		r2 := rand.New(rand.NewSource(99))
		s1 := GenerateServicesForSystem(r1, OSWindows, RoleServer, "TEST")
		s2 := GenerateServicesForSystem(r2, OSWindows, RoleServer, "TEST")
		if len(s1) != len(s2) {
			t.Fatalf("different lengths: %d vs %d", len(s1), len(s2))
		}
		for i := range s1 {
			if s1[i].Name != s2[i].Name {
				t.Errorf("service[%d]: %q vs %q", i, s1[i].Name, s2[i].Name)
			}
		}
	})
}
