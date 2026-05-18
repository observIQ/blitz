package datagen

import (
	"fmt"
	"math/rand"
	"time"
)

// Environment is the top-level container for all generated identities.
//
// Determinism contract: given the same (SeedConfig, opts.Now) pair, two
// calls to GenerateEnvironment produce an identical Environment. With
// opts.Now unset, defaultOpts substitutes time.Now() — the resulting CA
// validity window is still anchored to "now" in the same *relative* sense
// (5 years before to 5 years after), but the absolute timestamps and the
// CertAuthority's ValidFrom/ValidTo values will differ across runs.
//
// For fully reproducible tests/snapshots, set opts.Now to a fixed
// timestamp; the rest of the Environment derives from the seeds.
type Environment struct {
	Domain   *DomainIdentity
	Networks []*NetworkIdentity
	Users    []*UserIdentity
	Groups   []*GroupIdentity
	Systems  []*SystemIdentity
}

// EnvironmentOpts controls the size and shape of the generated environment.
type EnvironmentOpts struct {
	DomainName        string    // e.g., "contoso.com". Default: "blitz.local"
	SystemCount       int       // number of machines. Default: 20
	UserCount         int       // number of users. Default: 50
	GroupCount        int       // number of groups. Default: 10
	NetworkCount      int       // number of subnets. Default: 4. Values beyond the built-in default catalog are synthesized using IdentityNetworks.
	DomainAdminsCount int       // exact Domain Admins membership; 0 (or negative) = use the user-count-scaled default in the datagen package.
	Now               time.Time // wall-clock anchor for time-dependent fields (e.g. CertAuthority validity window). Zero value = time.Now() at GenerateEnvironment call; see Environment docstring for determinism implications.
}

// defaultOpts returns EnvironmentOpts with defaults applied.
func defaultOpts(opts *EnvironmentOpts) *EnvironmentOpts {
	if opts == nil {
		opts = &EnvironmentOpts{}
	}
	if opts.DomainName == "" {
		opts.DomainName = "blitz.local"
	}
	if opts.SystemCount <= 0 {
		opts.SystemCount = 20
	}
	if opts.UserCount <= 0 {
		opts.UserCount = 50
	}
	if opts.GroupCount <= 0 {
		opts.GroupCount = 10
	}
	if opts.NetworkCount <= 0 {
		opts.NetworkCount = 4
	}
	if opts.Now.IsZero() {
		opts.Now = time.Now()
	}
	return opts
}

// OS/role distribution for system generation.
type osRoleSpec struct {
	os   OSType
	role SystemRole
	pool *Pool[string]
}

// GenerateEnvironment produces a fully cross-referenced environment from seeds and options.
//
// Per-identity-type seeds drive each independent stage of generation:
// IdentityDomains for the domain + CA, IdentityNetworks for any synthesized
// subnets beyond the built-in defaults, IdentityUsers for users,
// IdentityGroups for groups, IdentitySystems for system core identity,
// IdentityServices for services attached to each system, and
// IdentityApplications for applications attached to each system. Each stage
// uses a fresh RNG seeded from its own field, so changing one seed only
// re-randomizes that slice of the output.
func GenerateEnvironment(seeds *SeedConfig, opts *EnvironmentOpts) *Environment {
	opts = defaultOpts(opts)

	// Generate domain
	domainSeed := seeds.ResolveSeed(IdentityDomains)
	domain := GenerateDomainIdentity(domainSeed, opts.DomainName, opts.Now)

	// Generate networks — defaults + synthesized extras when NetworkCount > default catalog size.
	networksSeed := seeds.ResolveSeed(IdentityNetworks)
	networks := generateNetworksList(networksSeed, opts.NetworkCount)

	// Generate users
	userSeed := seeds.ResolveSeed(IdentityUsers)
	users := GenerateUsers(userSeed, opts.UserCount, domain)

	// Generate groups
	groupSeed := seeds.ResolveSeed(IdentityGroups)
	groups := GenerateGroups(groupSeed, opts.GroupCount, opts.DomainAdminsCount, domain, users)

	// Generate systems with a mix of OS/roles. Services and applications
	// have their own seeded RNGs so changing IdentityServices or
	// IdentityApplications independently re-randomizes those slices.
	systemSeed := seeds.ResolveSeed(IdentitySystems)
	servicesSeed := seeds.ResolveSeed(IdentityServices)
	applicationsSeed := seeds.ResolveSeed(IdentityApplications)
	systems := generateSystems(systemSeed, servicesSeed, applicationsSeed, opts.SystemCount, domain, networks)

	return &Environment{
		Domain:   domain,
		Networks: networks,
		Users:    users,
		Groups:   groups,
		Systems:  systems,
	}
}

// generateNetworksList returns the requested number of NetworkIdentity entries,
// starting with the built-in default catalog and synthesizing additional
// subnets (using seed for randomized VLAN/zone selection) when count exceeds
// the catalog size.
func generateNetworksList(seed int64, count int) []*NetworkIdentity {
	defaults := GenerateDefaultNetworks()
	if count <= len(defaults) {
		return defaults[:count]
	}

	r := rand.New(rand.NewSource(seed)) // #nosec G404
	zones := []string{"trust", "untrust", "dmz", "management"}

	networks := make([]*NetworkIdentity, len(defaults), count)
	copy(networks, defaults)
	for i := len(defaults); i < count; i++ {
		idx := i - len(defaults)
		// Synthesize 10.30+/8 to avoid the default catalog's 10.10.X.0/24
		// allocations. Octet rolls in 0-255 give >65k unique /24 subnets.
		second := 30 + idx/256
		third := idx % 256
		cidr := fmt.Sprintf("10.%d.%d.0/24", second, third)
		networks = append(networks, &NetworkIdentity{
			ID:          fmt.Sprintf("net-%02d", i+1),
			Name:        fmt.Sprintf("Extended-Subnet-%02d", idx+1),
			CIDR:        cidr,
			Gateway:     fmt.Sprintf("10.%d.%d.1", second, third),
			VLAN:        1000 + idx,
			DHCPEnabled: r.Intn(2) == 0,            // #nosec G404
			Zone:        zones[r.Intn(len(zones))], // #nosec G404
		})
	}
	return networks
}

// generateSystems creates systems with a realistic OS/role distribution.
// systemSeed seeds the core identity RNG (OS choice, hostname, interface,
// resource specs). servicesSeed and applicationsSeed seed independent RNGs
// for service and application generation so changes to either seed only
// re-randomize that slice.
func generateSystems(systemSeed, servicesSeed, applicationsSeed int64, count int, domain *DomainIdentity, networks []*NetworkIdentity) []*SystemIdentity {
	r := rand.New(rand.NewSource(systemSeed))                   // #nosec G404
	rServices := rand.New(rand.NewSource(servicesSeed))         // #nosec G404
	rApplications := rand.New(rand.NewSource(applicationsSeed)) // #nosec G404

	// Distribution: ~10% DC, ~40% Windows server, ~30% Linux server, ~15% workstation, ~5% router
	specs := []osRoleSpec{
		{OSWindows, RoleDC, GreekNames},
		{OSWindows, RoleServer, RomanNames},
		{OSLinux, RoleServer, NorseNames},
		{OSWindows, RoleWorkstation, RomanNames},
		{OSLinux, RoleRouter, EgyptianNames},
	}
	weights := []float64{0.10, 0.40, 0.30, 0.15, 0.05}

	systems := make([]*SystemIdentity, count)
	for i := 0; i < count; i++ {
		// Pick OS/role using weighted selection
		spec := weightedSelect(r, specs, weights)
		sys := GenerateSystemIdentity(r, spec.os, spec.role, domain, spec.pool)

		// Services and applications use their own RNGs so the seeds in
		// SeedConfig actually drive what's generated, per identity type.
		sys.Services = GenerateServicesForSystem(rServices, sys.OS, sys.Role, sys.Hostname)
		sys.Applications = GenerateApplicationsForSystem(rApplications, sys.OS, sys.Role, sys.Hostname)

		// Assign network interface
		if len(networks) > 0 {
			net := pickNetworkForRole(r, sys.Role, networks)
			iface := NetworkInterface{
				Name:       interfaceName(sys.OS),
				IPv4:       RandomIPInCIDR(r, net.CIDR),
				IPv6:       RandomIPv6(r),
				MACAddress: RandomMAC(r),
				SubnetID:   net.ID,
				VLAN:       net.VLAN,
			}
			sys.Interfaces = []NetworkInterface{iface}
		}

		systems[i] = sys
	}

	return systems
}

// weightedSelect picks an item using weighted random selection.
func weightedSelect[T any](r *rand.Rand, items []T, weights []float64) T {
	roll := r.Float64() // #nosec G404
	cumulative := 0.0
	for i, w := range weights {
		cumulative += w
		if roll < cumulative {
			return items[i]
		}
	}
	return items[len(items)-1]
}

// pickNetworkForRole selects an appropriate network based on system role.
func pickNetworkForRole(r *rand.Rand, role SystemRole, networks []*NetworkIdentity) *NetworkIdentity {
	// Try to match by zone convention
	for _, n := range networks {
		switch {
		case role == RoleDC && n.Zone == "trust" && n.Name == "Server-VLAN":
			return n
		case role == RoleServer && n.Zone == "trust" && n.Name == "Server-VLAN":
			return n
		case role == RoleWorkstation && n.Name == "Workstation-LAN":
			return n
		case role == RoleRouter && n.Zone == "management":
			return n
		}
	}
	// Fallback: random network
	return networks[r.Intn(len(networks))] // #nosec G404
}

// interfaceName returns a conventional interface name for the OS.
func interfaceName(os OSType) string {
	switch os {
	case OSWindows:
		return "Ethernet0"
	case OSMacOS:
		return "en0"
	default:
		return "eth0"
	}
}
