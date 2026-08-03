package datagen

import (
	"math/rand"
	"testing"
	"time"
)

func TestGenerateEnvironment(t *testing.T) {
	seeds := NewSeedConfig()
	seeds.Shared = 42
	opts := &EnvironmentOpts{
		DomainName:   "contoso.com",
		SystemCount:  10,
		UserCount:    20,
		GroupCount:   8,
		NetworkCount: 4,
	}

	env, err := GenerateEnvironment(seeds, opts)
	if err != nil {
		t.Fatalf("GenerateEnvironment: %v", err)
	}

	t.Run("domain set", func(t *testing.T) {
		if env.Domain == nil {
			t.Fatal("Domain should not be nil")
		}
		if env.Domain.Name != "contoso.com" {
			t.Errorf("expected domain 'contoso.com', got %q", env.Domain.Name)
		}
	})

	t.Run("systems count", func(t *testing.T) {
		if len(env.Systems) != 10 {
			t.Errorf("expected 10 systems, got %d", len(env.Systems))
		}
	})

	t.Run("users count", func(t *testing.T) {
		if len(env.Users) != 20 {
			t.Errorf("expected 20 users, got %d", len(env.Users))
		}
	})

	t.Run("groups present", func(t *testing.T) {
		if len(env.Groups) < 8 {
			t.Errorf("expected at least 8 groups, got %d", len(env.Groups))
		}
	})

	t.Run("networks present", func(t *testing.T) {
		if len(env.Networks) < 4 {
			t.Errorf("expected at least 4 networks, got %d", len(env.Networks))
		}
	})

	t.Run("systems have services and apps", func(t *testing.T) {
		hasServices := false
		hasApps := false
		for _, sys := range env.Systems {
			if len(sys.Services) > 0 {
				hasServices = true
			}
			if len(sys.Applications) > 0 {
				hasApps = true
			}
		}
		if !hasServices {
			t.Error("at least one system should have services")
		}
		if !hasApps {
			t.Error("at least one system should have applications")
		}
	})

	t.Run("systems have network interfaces", func(t *testing.T) {
		hasInterfaces := false
		for _, sys := range env.Systems {
			if len(sys.Interfaces) > 0 {
				hasInterfaces = true
				break
			}
		}
		if !hasInterfaces {
			t.Error("at least one system should have network interfaces")
		}
	})
}

func TestGenerateEnvironmentComposesAppliances(t *testing.T) {
	env, err := GenerateEnvironment(&SeedConfig{Shared: 42}, &EnvironmentOpts{StorageSystemCount: 3, NetworkSystemCount: 5})
	if err != nil {
		t.Fatalf("GenerateEnvironment: %v", err)
	}

	if len(env.StorageSystems) != 3 {
		t.Fatalf("StorageSystems = %d, want 3", len(env.StorageSystems))
	}
	if len(env.NetworkSystems) != 5 {
		t.Fatalf("NetworkSystems = %d, want 5", len(env.NetworkSystems))
	}
	for _, s := range env.AllStorageSystems() {
		if err := s.Validate(); err != nil {
			t.Errorf("storage %s invalid: %v", s.Model, err)
		}
		if s.ManagementInterface.SubnetID == "" {
			t.Errorf("storage %s management interface not bound to a subnet", s.Model)
		}
	}
	for _, n := range env.AllNetworkSystems() {
		if err := n.Validate(); err != nil {
			t.Errorf("network %s invalid: %v", n.Model, err)
		}
		if n.ManagementInterface.SubnetID == "" {
			t.Errorf("network %s management interface not bound to a subnet", n.Model)
		}
	}
}

func TestGenerateEnvironmentDeterministicAppliances(t *testing.T) {
	mk := func() *Environment {
		s := NewSeedConfig()
		s.Shared = 7
		env, err := GenerateEnvironment(s, &EnvironmentOpts{StorageSystemCount: 2, NetworkSystemCount: 2})
		if err != nil {
			t.Fatalf("GenerateEnvironment: %v", err)
		}
		return env
	}
	a, b := mk(), mk()
	if a.StorageSystems[0].Serial != b.StorageSystems[0].Serial {
		t.Error("storage systems not deterministic across runs")
	}
	if a.NetworkSystems[0].Serial != b.NetworkSystems[0].Serial {
		t.Error("network systems not deterministic across runs")
	}
}

func TestManagementNetwork(t *testing.T) {
	mgmt := &NetworkIdentity{ID: "net-mgmt", Zone: "management", CIDR: "10.0.0.0/24"}
	trust := &NetworkIdentity{ID: "net-trust", Zone: "trust", CIDR: "10.1.0.0/24"}
	if got := managementNetwork([]*NetworkIdentity{trust, mgmt}); got != mgmt {
		t.Errorf("want the management-zone network, got %v", got)
	}
	if got := managementNetwork([]*NetworkIdentity{trust}); got != trust {
		t.Errorf("want the first network as fallback, got %v", got)
	}
	if got := managementNetwork(nil); got != nil {
		t.Errorf("want nil for empty networks, got %v", got)
	}
}

func TestBindManagementInterface(t *testing.T) {
	r := rand.New(rand.NewSource(1))
	iface := &NetworkInterface{Name: "mgmt0", IPv4: "192.168.1.5"}

	// A nil subnet leaves the interface untouched.
	bindManagementInterface(r, iface, nil)
	if iface.SubnetID != "" {
		t.Errorf("nil subnet should leave SubnetID empty, got %q", iface.SubnetID)
	}
	// A nil interface is a no-op and must not panic.
	bindManagementInterface(r, nil, &NetworkIdentity{ID: "x", CIDR: "10.0.0.0/24"})
	// Binding to a subnet sets the subnet ID and an in-CIDR address.
	bindManagementInterface(r, iface, &NetworkIdentity{ID: "net-9", CIDR: "10.9.0.0/24"})
	if iface.SubnetID != "net-9" {
		t.Errorf("SubnetID = %q, want net-9", iface.SubnetID)
	}
}

func TestGenerateEnvironmentDefaults(t *testing.T) {
	seeds := NewSeedConfig()
	seeds.Shared = 42
	env, err := GenerateEnvironment(seeds, nil)
	if err != nil {
		t.Fatalf("GenerateEnvironment: %v", err)
	}

	if env.Domain.Name != "blitz.local" {
		t.Errorf("default domain should be 'blitz.local', got %q", env.Domain.Name)
	}
	if len(env.Systems) != 20 {
		t.Errorf("default system count should be 20, got %d", len(env.Systems))
	}
	if len(env.Users) != 50 {
		t.Errorf("default user count should be 50, got %d", len(env.Users))
	}
}

func TestGenerateEnvironmentDeterministic(t *testing.T) {
	// Pinning opts.Now is required for full determinism — see the
	// Environment docstring.
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	seeds1 := NewSeedConfig()
	seeds1.Shared = 42
	seeds2 := NewSeedConfig()
	seeds2.Shared = 42
	opts := &EnvironmentOpts{
		SystemCount: 5,
		UserCount:   10,
		GroupCount:  5,
		Now:         now,
	}

	env1, err := GenerateEnvironment(seeds1, opts)
	if err != nil {
		t.Fatalf("GenerateEnvironment: %v", err)
	}
	env2, err := GenerateEnvironment(seeds2, opts)
	if err != nil {
		t.Fatalf("GenerateEnvironment: %v", err)
	}

	if env1.Domain.DomainSID != env2.Domain.DomainSID {
		t.Error("same seed should produce same DomainSID")
	}
	if !env1.Domain.CA.ValidFrom.Equal(env2.Domain.CA.ValidFrom) {
		t.Error("same (seed, Now) should produce same CA ValidFrom")
	}

	for i := range env1.Systems {
		if env1.Systems[i].Hostname != env2.Systems[i].Hostname {
			t.Errorf("system[%d]: hostnames differ: %q vs %q",
				i, env1.Systems[i].Hostname, env2.Systems[i].Hostname)
		}
	}

	for i := range env1.Users {
		if env1.Users[i].Username != env2.Users[i].Username {
			t.Errorf("user[%d]: usernames differ: %q vs %q",
				i, env1.Users[i].Username, env2.Users[i].Username)
		}
	}
}

func TestGenerateEnvironmentExtendsNetworks(t *testing.T) {
	// NetworkCount > the default catalog (4) synthesizes additional subnets
	// using IdentityNetworks. Previously the code silently capped at 4.
	seeds := NewSeedConfig()
	seeds.Shared = 42
	opts := &EnvironmentOpts{
		SystemCount:  5,
		UserCount:    5,
		GroupCount:   5,
		NetworkCount: 10,
		Now:          time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	env, err := GenerateEnvironment(seeds, opts)
	if err != nil {
		t.Fatalf("GenerateEnvironment: %v", err)
	}
	if len(env.Networks) != 10 {
		t.Errorf("expected 10 networks, got %d", len(env.Networks))
	}
	// Synthesized subnets get IDs net-05+ and CIDRs outside 10.10.X.0/24.
	seenIDs := make(map[string]bool)
	for _, n := range env.Networks {
		if seenIDs[n.ID] {
			t.Errorf("duplicate network ID %q", n.ID)
		}
		seenIDs[n.ID] = true
	}
}

func TestGenerateEnvironmentPerTypeSeedsIndependent(t *testing.T) {
	// Changing IdentityServices should re-randomize service assignments but
	// leave hostnames, domain, users untouched. Confirms IdentityServices is
	// actually wired up.
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	base := NewSeedConfig()
	base.Shared = 42
	opts := &EnvironmentOpts{SystemCount: 5, UserCount: 5, GroupCount: 5, NetworkCount: 4, Now: now}

	envA, err := GenerateEnvironment(base, opts)
	if err != nil {
		t.Fatalf("GenerateEnvironment: %v", err)
	}

	altered := NewSeedConfig()
	altered.Shared = 42
	altered.Services = 777 // override only services
	envB, err := GenerateEnvironment(altered, opts)
	if err != nil {
		t.Fatalf("GenerateEnvironment: %v", err)
	}

	// Domain and hostnames must be identical — they don't depend on IdentityServices.
	if envA.Domain.DomainSID != envB.Domain.DomainSID {
		t.Error("changing IdentityServices should not affect domain")
	}
	for i := range envA.Systems {
		if envA.Systems[i].Hostname != envB.Systems[i].Hostname {
			t.Errorf("system[%d]: hostname changed when only IdentityServices was overridden", i)
		}
	}

	// At least one system's services should differ — proving the seed is in play.
	servicesDiffer := false
	for i := range envA.Systems {
		if len(envA.Systems[i].Services) != len(envB.Systems[i].Services) {
			servicesDiffer = true
			break
		}
		for j := range envA.Systems[i].Services {
			if envA.Systems[i].Services[j].Name != envB.Systems[i].Services[j].Name {
				servicesDiffer = true
				break
			}
		}
		if servicesDiffer {
			break
		}
	}
	if !servicesDiffer {
		t.Error("overriding IdentityServices should re-randomize at least one system's services")
	}
}

func TestGenerateEnvironmentCARelativeValidityWindow(t *testing.T) {
	// With opts.Now unset, the CA validity window is anchored to the
	// runtime time.Now() but always spans the same relative range
	// (5 years before to 5 years after). Two back-to-back calls produce
	// different absolute timestamps but the same relative duration.
	seeds := NewSeedConfig()
	seeds.Shared = 42

	envA, err := GenerateEnvironment(seeds, &EnvironmentOpts{SystemCount: 1, UserCount: 1, GroupCount: 5})
	if err != nil {
		t.Fatalf("GenerateEnvironment: %v", err)
	}
	envB, err := GenerateEnvironment(seeds, &EnvironmentOpts{SystemCount: 1, UserCount: 1, GroupCount: 5})
	if err != nil {
		t.Fatalf("GenerateEnvironment: %v", err)
	}

	// Same relative span (both should be 10 years total, within tolerance).
	wantSpan := envA.Domain.CA.ValidTo.Sub(envA.Domain.CA.ValidFrom)
	gotSpan := envB.Domain.CA.ValidTo.Sub(envB.Domain.CA.ValidFrom)
	if wantSpan != gotSpan {
		t.Errorf("CA validity span differs: %v vs %v", wantSpan, gotSpan)
	}
	// Each window should be ~10 years (allow a hair for leap-year drift in AddDate).
	const minSpan = 9*365*24*time.Hour + 364*24*time.Hour // ~9.997 years
	if wantSpan < minSpan {
		t.Errorf("CA validity span %v shorter than expected ~10 years", wantSpan)
	}
}
