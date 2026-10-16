package datagen

import (
	"math/rand"
	"strings"
	"testing"
	"time"
)

// TestDomainControllersRunWindowsServer pins that a domain controller only
// ever runs a Windows Server release, whether built alone or in an
// environment (prod hosts share a pinned baseline, non-prod hosts roll newer).
func TestDomainControllersRunWindowsServer(t *testing.T) {
	domain := GenerateDomainIdentity(1, "blitz.local", time.Unix(1700000000, 0))
	for seed := int64(0); seed < 200; seed++ {
		sys, err := GenerateSystemIdentity(rand.New(rand.NewSource(seed)), OSWindows, RoleDC, domain, GreekNames) // #nosec G404
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(sys.OSInfo.Name, "Windows Server") {
			t.Fatalf("seed %d: DC runs %q", seed, sys.OSInfo.Name)
		}
	}
	dc := HostRequirement{OS: OSWindows, Roles: []SystemRole{RoleDC}}
	for systems := 1; systems <= 20; systems++ {
		for seed := int64(0); seed < 50; seed++ {
			env, err := GenerateEnvironment(&SeedConfig{Shared: seed, Systems: -1, Users: -1, Groups: -1, Services: -1,
				Applications: -1, Networks: -1, Domains: -1, StorageSystems: -1, NetworkSystems: -1},
				&EnvironmentOpts{SystemCount: systems, RequiredHosts: []HostRequirement{dc, dc}})
			if err != nil {
				t.Fatal(err)
			}
			for _, s := range env.Systems {
				if s.Role == RoleDC && !strings.Contains(s.OSInfo.Name, "Windows Server") {
					t.Fatalf("systems %d seed %d: DC %s (%s tier) runs %q", systems, seed, s.Hostname, s.Tier, s.OSInfo.Name)
				}
			}
		}
	}
}
