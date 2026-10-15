package datagen

import (
	"testing"
)

// TestRequiredVendorsAlwaysPresent pins that an environment built for an
// appliance generator always holds a device of that vendor, deterministically,
// so the generator describes a real appliance instead of a fallback name.
func TestRequiredVendorsAlwaysPresent(t *testing.T) {
	for seed := int64(0); seed < 100; seed++ {
		build := func() *Environment {
			env, err := GenerateEnvironment(&SeedConfig{Shared: seed, Systems: -1, Users: -1, Groups: -1, Services: -1,
				Applications: -1, Networks: -1, Domains: -1, StorageSystems: -1, NetworkSystems: -1},
				&EnvironmentOpts{RequiredVendors: []ApplianceVendor{VendorPaloAlto, VendorF5}})
			if err != nil {
				t.Fatal(err)
			}
			return env
		}
		env := build()
		for _, v := range []ApplianceVendor{VendorPaloAlto, VendorF5} {
			n := env.ApplianceFor("key", 0, v)
			if n == nil || n.Vendor != v {
				t.Fatalf("seed %d: no %s appliance", seed, v)
			}
			if n.Hostname == "" {
				t.Fatalf("seed %d: %s appliance has no hostname", seed, v)
			}
			if again := build().ApplianceFor("key", 0, v); again.Hostname != n.Hostname || again.Serial != n.Serial {
				t.Fatalf("seed %d: %s appliance is not deterministic", seed, v)
			}
		}
		seen := map[string]bool{}
		for _, n := range env.NetworkSystems {
			if seen[n.Hostname] {
				t.Fatalf("seed %d: duplicate appliance hostname %q", seed, n.Hostname)
			}
			seen[n.Hostname] = true
		}
	}
}

// TestSystemForOrdinalSpreadsSameTypeEntries pins that two generators of the
// same type (a generators: list with two hostmetrics entries) land on
// different hosts when the environment has enough eligible ones.
func TestSystemForOrdinalSpreadsSameTypeEntries(t *testing.T) {
	env := &Environment{Systems: []*SystemIdentity{
		{Hostname: "a", OSInfo: OSInfo{Type: OSLinux}}, {Hostname: "b", OSInfo: OSInfo{Type: OSLinux}},
		{Hostname: "c", OSInfo: OSInfo{Type: OSLinux}},
	}}
	req := HostRequirement{OS: OSLinux}
	first, second := env.SystemFor("hostmetrics", 0, req), env.SystemFor("hostmetrics", 1, req)
	if first == nil || second == nil || first == second {
		t.Fatalf("entries 0 and 1 share host %v", first)
	}
}

// TestAppliancesCarryTheirFQDN pins that every environment device's FQDN is
// its hostname in the environment's domain, as a firewall joined to that
// domain writes it in its syslog header.
func TestAppliancesCarryTheirFQDN(t *testing.T) {
	env, err := GenerateEnvironment(NewSeedConfig(), &EnvironmentOpts{DomainName: "contoso.local", RequiredVendors: []ApplianceVendor{VendorPaloAlto}})
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range env.NetworkSystems {
		if want := n.Hostname + ".contoso.local"; n.FQDN != want {
			t.Fatalf("device %s FQDN %q, want %q", n.Hostname, n.FQDN, want)
		}
	}
}

// TestRequiredVendorsCountPerEntry pins that each entry needing a vendor gets
// its own device: two palo-alto entries describe two firewalls, not one
// firewall with two independent sequence counters.
func TestRequiredVendorsCountPerEntry(t *testing.T) {
	for seed := int64(0); seed < 50; seed++ {
		env, err := GenerateEnvironment(&SeedConfig{Shared: seed, Systems: -1, Users: -1, Groups: -1, Services: -1,
			Applications: -1, Networks: -1, Domains: -1, StorageSystems: -1, NetworkSystems: -1},
			&EnvironmentOpts{RequiredVendors: []ApplianceVendor{VendorPaloAlto, VendorPaloAlto}})
		if err != nil {
			t.Fatal(err)
		}
		a, b := env.ApplianceFor("palo-alto", 0, VendorPaloAlto), env.ApplianceFor("palo-alto", 1, VendorPaloAlto)
		if a == nil || b == nil || a == b {
			t.Fatalf("seed %d: entries 0 and 1 share firewall %v", seed, a)
		}
	}
}

// TestRequiredVendorsLeaveDrawnDevicesUnchanged pins that guaranteeing a
// vendor appends its device without renumbering the drawn fleet's management
// addresses, as generateSystems does for hosts.
func TestRequiredVendorsLeaveDrawnDevicesUnchanged(t *testing.T) {
	for seed := int64(0); seed < 50; seed++ {
		build := func(req []ApplianceVendor) *Environment {
			env, err := GenerateEnvironment(&SeedConfig{Shared: seed, Systems: -1, Users: -1, Groups: -1, Services: -1,
				Applications: -1, Networks: -1, Domains: -1, StorageSystems: -1, NetworkSystems: -1},
				&EnvironmentOpts{RequiredVendors: req})
			if err != nil {
				t.Fatal(err)
			}
			return env
		}
		plain, withF5 := build(nil), build([]ApplianceVendor{VendorF5})
		for i, n := range plain.NetworkSystems {
			if got, want := withF5.NetworkSystems[i].ManagementInterface.IPv4, n.ManagementInterface.IPv4; got != want {
				t.Fatalf("seed %d: device %d management IP moved from %s to %s", seed, i, want, got)
			}
		}
	}
}

// TestRequiredHostsCountPerEntry pins that each entry with a host requirement
// gets its own host: two macOS hostmetrics entries describe two Macs.
func TestRequiredHostsCountPerEntry(t *testing.T) {
	mac := HostRequirement{OS: OSMacOS}
	for seed := int64(0); seed < 50; seed++ {
		env, err := GenerateEnvironment(&SeedConfig{Shared: seed, Systems: -1, Users: -1, Groups: -1, Services: -1,
			Applications: -1, Networks: -1, Domains: -1, StorageSystems: -1, NetworkSystems: -1},
			&EnvironmentOpts{RequiredHosts: []HostRequirement{mac, mac}})
		if err != nil {
			t.Fatal(err)
		}
		a, b := env.SystemFor("hostmetrics", 0, mac), env.SystemFor("hostmetrics", 1, mac)
		if a == nil || b == nil || a == b {
			t.Fatalf("seed %d: entries 0 and 1 share host %v", seed, a)
		}
	}
}

// TestSameKeyEntriesWithOverlappingRequirementsGetTheirOwnHosts pins that two
// entries of one generator type never share a host, even when their
// requirements differ but overlap (a wel entry with any role and one limited
// to domain controllers).
func TestSameKeyEntriesWithOverlappingRequirementsGetTheirOwnHosts(t *testing.T) {
	anyWin := HostRequirement{OS: OSWindows, Roles: []SystemRole{RoleDC, RoleServer, RoleWorkstation}}
	dc := HostRequirement{OS: OSWindows, Roles: []SystemRole{RoleDC}}
	for seed := int64(0); seed < 300; seed++ {
		env, err := GenerateEnvironment(&SeedConfig{Shared: seed, Systems: -1, Users: -1, Groups: -1, Services: -1,
			Applications: -1, Networks: -1, Domains: -1, StorageSystems: -1, NetworkSystems: -1},
			&EnvironmentOpts{RequiredHosts: []HostRequirement{anyWin, dc}})
		if err != nil {
			t.Fatal(err)
		}
		a, b := env.SystemFor("wel", 0, anyWin), env.SystemFor("wel", 1, dc)
		if a == nil || b == nil || a == b {
			t.Fatalf("seed %d: entries 0 and 1 share host %v", seed, a)
		}
		if !dc.accepts(b) {
			t.Fatalf("seed %d: entry 1 got %s, not a DC", seed, b.Role)
		}
	}
}

// TestOverlappingRequirementsGetTheirOwnHostsInSmallFleets pins the same
// contract when the drawn fleet is small: a requirement short by more than one
// host (an earlier overlapping requirement took a drawn host it does not
// accept) gets every missing host appended, not just one.
func TestOverlappingRequirementsGetTheirOwnHostsInSmallFleets(t *testing.T) {
	anyWin := HostRequirement{OS: OSWindows, Roles: []SystemRole{RoleDC, RoleServer, RoleWorkstation}}
	dc := HostRequirement{OS: OSWindows, Roles: []SystemRole{RoleDC}}
	for systems := 1; systems <= 10; systems++ {
		for seed := int64(0); seed < 100; seed++ {
			env, err := GenerateEnvironment(&SeedConfig{Shared: seed, Systems: -1, Users: -1, Groups: -1, Services: -1,
				Applications: -1, Networks: -1, Domains: -1, StorageSystems: -1, NetworkSystems: -1},
				&EnvironmentOpts{SystemCount: systems, RequiredHosts: []HostRequirement{anyWin, dc}})
			if err != nil {
				t.Fatal(err)
			}
			a, b := env.SystemFor("wel", 0, anyWin), env.SystemFor("wel", 1, dc)
			if a == nil || b == nil || a == b {
				t.Fatalf("systems %d seed %d: entries 0 and 1 share a host", systems, seed)
			}
			if !dc.accepts(b) {
				t.Fatalf("systems %d seed %d: entry 1 got %s, not a DC", systems, seed, b.Role)
			}
		}
	}
}
