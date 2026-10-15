package datagen

import (
	"testing"
)

// TestRequiredHostsAlwaysPresent pins that an environment built for a
// generator that needs a particular host (a domain controller, a macOS host)
// always holds one, deterministically, so the generator never falls back to
// a synthetic identity. Systems the random draw produced stay unchanged.
func TestRequiredHostsAlwaysPresent(t *testing.T) {
	reqs := []HostRequirement{
		{OS: OSWindows, Roles: []SystemRole{RoleDC}},
		{OS: OSMacOS},
		{OS: OSLinux, Roles: []SystemRole{RoleServer}},
	}
	seeds := func(seed int64) *SeedConfig {
		return &SeedConfig{Shared: seed, Systems: -1, Users: -1, Groups: -1, Services: -1,
			Applications: -1, Networks: -1, Domains: -1, StorageSystems: -1, NetworkSystems: -1}
	}
	for seed := int64(0); seed < 200; seed++ {
		build := func() *Environment {
			env, err := GenerateEnvironment(seeds(seed), &EnvironmentOpts{SystemCount: 5, RequiredHosts: reqs})
			if err != nil {
				t.Fatal(err)
			}
			return env
		}
		env := build()
		for _, req := range reqs {
			s := env.SystemFor("key", 0, req)
			if s == nil {
				t.Fatalf("seed %d: no host meets %+v", seed, req)
			}
			if again := build().SystemFor("key", 0, req); again.Hostname != s.Hostname {
				t.Fatalf("seed %d: %+v host not deterministic: %s then %s", seed, req, s.Hostname, again.Hostname)
			}
		}
		plain, err := GenerateEnvironment(seeds(seed), &EnvironmentOpts{SystemCount: 5})
		if err != nil {
			t.Fatal(err)
		}
		for i, s := range plain.Systems {
			if env.Systems[i].Hostname != s.Hostname {
				t.Fatalf("seed %d: required hosts changed drawn system %d: %s became %s", seed, i, s.Hostname, env.Systems[i].Hostname)
			}
		}
	}
}
