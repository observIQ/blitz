package datagen

import (
	"fmt"
	"hash/fnv"
	"math/rand"
	"time"

	"go.uber.org/zap"
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
	Domain         *DomainIdentity
	Networks       []*NetworkIdentity
	Users          []*UserIdentity
	Groups         []*GroupIdentity
	Systems        []*SystemIdentity
	StorageSystems []*StorageSystemIdentity
	NetworkSystems []*NetworkSystemIdentity
}

// AllStorageSystems returns the environment's storage-array identities.
func (e *Environment) AllStorageSystems() []*StorageSystemIdentity { return e.StorageSystems }

// AllNetworkSystems returns the environment's network-hardware identities.
func (e *Environment) AllNetworkSystems() []*NetworkSystemIdentity { return e.NetworkSystems }

// SystemForKey deterministically selects one of the environment's Systems by a
// caller-supplied key (typically a generator component name, or a component
// plus worker index for per-worker host granularity). The same key always maps
// to the same system for a given Environment, so a generator resolves its host
// identity once and attributes every record it emits consistently. Returns nil
// when the environment has no systems.
func (e *Environment) SystemForKey(key string) *SystemIdentity {
	if len(e.Systems) == 0 {
		return nil
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(key))
	// int64 throughout: uint32->int64 and int->int64 are widening (never
	// negative, never truncating), so this is correct on 32-bit targets and
	// avoids an int->uint32 narrowing conversion.
	idx := int64(h.Sum32()) % int64(len(e.Systems))
	return e.Systems[idx]
}

// EnvironmentOpts controls the size and shape of the generated environment.
type EnvironmentOpts struct {
	DomainName         string      // e.g., "contoso.com". Default: "blitz.local"
	SystemCount        int         // number of machines. Default: 20
	UserCount          int         // number of users. Default: 50
	GroupCount         int         // number of groups. Default: 10
	NetworkCount       int         // number of subnets. Default: 4. Values beyond the built-in default catalog are synthesized using IdentityNetworks.
	StorageSystemCount int         // number of storage arrays. Default: 2.
	NetworkSystemCount int         // number of network devices. Default: 4.
	DomainAdminsCount  int         // exact Domain Admins membership; 0 (or negative) = use the user-count-scaled default in the datagen package.
	Now                time.Time   // wall-clock anchor for time-dependent fields (e.g. CertAuthority validity window). Zero value = time.Now() at GenerateEnvironment call; see Environment docstring for determinism implications.
	Logger             *zap.Logger // logger for datagen diagnostics; nil disables logging (no global-logger fallback).
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
	if opts.StorageSystemCount <= 0 {
		opts.StorageSystemCount = 2
	}
	if opts.NetworkSystemCount <= 0 {
		opts.NetworkSystemCount = 4
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

// Stage seams. GenerateEnvironment calls its identity-generation stages
// through these package-level variables so tests can force a stage to fail and
// assert that GenerateEnvironment propagates the error. Production always uses
// the real implementations; the stages cannot fail today (GenerateEnvironment
// builds a valid domain), but the propagation is kept as a regression guard for
// when a stage gains a real failure mode.
var (
	genUsers   = GenerateUsers
	genGroups  = GenerateGroups
	genSystems = generateSystems
)

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
func GenerateEnvironment(seeds *SeedConfig, opts *EnvironmentOpts) (*Environment, error) {
	opts = defaultOpts(opts)

	// Generate domain
	domainSeed := seeds.ResolveSeed(IdentityDomains)
	domain := GenerateDomainIdentity(domainSeed, opts.DomainName, opts.Now)

	// Generate networks — defaults + synthesized extras when NetworkCount > default catalog size.
	networksSeed := seeds.ResolveSeed(IdentityNetworks)
	networks := generateNetworksList(networksSeed, opts.NetworkCount)

	// Generate users
	userSeed := seeds.ResolveSeed(IdentityUsers)
	users, err := genUsers(userSeed, opts.UserCount, domain)
	if err != nil {
		return nil, err
	}

	// Generate groups
	groupSeed := seeds.ResolveSeed(IdentityGroups)
	groups, err := genGroups(groupSeed, opts.GroupCount, opts.DomainAdminsCount, domain, users)
	if err != nil {
		return nil, err
	}

	// Generate systems with a mix of OS/roles. Services and applications
	// have their own seeded RNGs so changing IdentityServices or
	// IdentityApplications independently re-randomizes those slices.
	systemSeed := seeds.ResolveSeed(IdentitySystems)
	servicesSeed := seeds.ResolveSeed(IdentityServices)
	applicationsSeed := seeds.ResolveSeed(IdentityApplications)
	systems, err := genSystems(systemSeed, servicesSeed, applicationsSeed, opts.SystemCount, domain, networks, opts.Logger)
	if err != nil {
		return nil, err
	}

	// Appliance identities (PIPE-1035): storage arrays and network hardware,
	// each with its own seed and a management interface bound to a subnet.
	storageSeed := seeds.ResolveSeed(IdentityStorageSystems)
	storageSystems := generateStorageSystems(storageSeed, opts.StorageSystemCount, networks)

	networkSystemSeed := seeds.ResolveSeed(IdentityNetworkSystems)
	networkSystems := generateNetworkSystems(networkSystemSeed, opts.NetworkSystemCount, networks)

	return &Environment{
		Domain:         domain,
		Networks:       networks,
		Users:          users,
		Groups:         groups,
		Systems:        systems,
		StorageSystems: storageSystems,
		NetworkSystems: networkSystems,
	}, nil
}

// managementNetwork returns the subnet to bind appliance management interfaces
// to: the "management" zone if present, else the first network, else nil.
func managementNetwork(networks []*NetworkIdentity) *NetworkIdentity {
	for _, n := range networks {
		if n.Zone == "management" {
			return n
		}
	}
	if len(networks) > 0 {
		return networks[0]
	}
	return nil
}

// bindManagementInterface points a management interface at a subnet, giving it
// an in-CIDR address and the subnet ID. A nil interface or subnet is left
// untouched.
func bindManagementInterface(r *rand.Rand, iface *NetworkInterface, subnet *NetworkIdentity) {
	if iface == nil || subnet == nil {
		return
	}
	iface.IPv4 = RandomIPInCIDR(r, subnet.CIDR)
	iface.SubnetID = subnet.ID
}

// generateStorageSystems builds count storage arrays from seed, binding each
// management interface to a management subnet.
func generateStorageSystems(seed int64, count int, networks []*NetworkIdentity) []*StorageSystemIdentity {
	r := rand.New(rand.NewSource(seed)) // #nosec G404
	mgmt := managementNetwork(networks)
	out := make([]*StorageSystemIdentity, count)
	for i := range out {
		s := RandomStorageSystemIdentity(r)
		bindManagementInterface(r, s.ManagementInterface, mgmt)
		out[i] = s
	}
	return out
}

// generateNetworkSystems builds count network devices from seed, binding each
// management interface to a management subnet.
func generateNetworkSystems(seed int64, count int, networks []*NetworkIdentity) []*NetworkSystemIdentity {
	r := rand.New(rand.NewSource(seed)) // #nosec G404
	mgmt := managementNetwork(networks)
	out := make([]*NetworkSystemIdentity, count)
	for i := range out {
		n := RandomNetworkSystemIdentity(r)
		bindManagementInterface(r, n.ManagementInterface, mgmt)
		out[i] = n
	}
	return out
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
func generateSystems(systemSeed, servicesSeed, applicationsSeed int64, count int, domain *DomainIdentity, networks []*NetworkIdentity, logger *zap.Logger) ([]*SystemIdentity, error) {
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

	// Prod-tier baseline OS release per family, chosen once so every prod host
	// of a family is pinned to the same conservative release (real fleets keep
	// prod uniform). Non-prod hosts roll newer and vary per host.
	prodBaseline := map[OSType]OSInfo{
		OSLinux:   osInfoForTier(r, OSLinux, true),
		OSWindows: osInfoForTier(r, OSWindows, true),
		OSMacOS:   osInfoForTier(r, OSMacOS, true),
	}

	systems := make([]*SystemIdentity, count)
	for i := 0; i < count; i++ {
		// Pick OS/role using weighted selection
		spec := weightedSelect(r, specs, weights)
		sys, err := GenerateSystemIdentity(r, spec.os, spec.role, domain, spec.pool)
		if err != nil {
			return nil, err
		}

		// Assign a deployment tier and cluster the OS release by it: prod hosts
		// share the pinned family baseline; non-prod hosts roll newer and vary.
		sys.Tier = weightedSelect(r, DeploymentTiers, deploymentTierWeights)
		if sys.Tier == TierProd {
			sys.OSInfo = prodBaseline[spec.os]
		} else {
			sys.OSInfo = osInfoForTier(r, spec.os, false)
		}

		// Services and applications use their own RNGs so the seeds in
		// SeedConfig actually drive what's generated, per identity type.
		sys.Services = GenerateServicesForSystem(rServices, sys.OSInfo.Type, sys.Role, sys.Hostname)
		sys.Applications = GenerateApplicationsForSystem(rApplications, sys.OSInfo.Type, sys.Role, sys.Hostname, logger)

		// Assign network interface
		if len(networks) > 0 {
			net := pickNetworkForRole(r, sys.Role, networks)
			iface := NetworkInterface{
				Name:       interfaceName(sys.OSInfo.Type),
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

	return systems, nil
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
