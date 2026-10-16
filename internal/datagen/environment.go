package datagen

import (
	"cmp"
	"fmt"
	"hash/fnv"
	"math/rand"
	"slices"
	"sync"
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

	// picks records pick's selections (hosts and devices) per key, so
	// same-key entries never share one. Guarded by picksMu.
	picksMu sync.Mutex
	picks   map[string]map[int]any
}

// AllStorageSystems returns the environment's storage-array identities.
func (e *Environment) AllStorageSystems() []*StorageSystemIdentity { return e.StorageSystems }

// AllNetworkSystems returns the environment's network-hardware identities.
func (e *Environment) AllNetworkSystems() []*NetworkSystemIdentity { return e.NetworkSystems }

// HostRequirement narrows which of the environment's hosts a generator may
// describe, so an OS-limited generator is never handed a host it could not run
// on (Windows Event Log on a Linux box, say). The zero value accepts any system.
//
// Appliance-backed generators (F5, Palo Alto) extend this with a host kind that
// selects from NetworkSystems by vendor instead of from Systems; see
// docs/environment.md.
type HostRequirement struct {
	// OS restricts selection to systems of this OS. Empty accepts any OS.
	OS OSType
	// Roles restricts selection to systems of one of these roles. Empty
	// accepts any role.
	Roles []SystemRole
	// Apps restricts selection to systems that run applications, for
	// generators describing server software.
	Apps bool
}

// overlaps reports whether some system could meet both r and o.
func (r HostRequirement) overlaps(o HostRequirement) bool {
	if r.OS != "" && o.OS != "" && r.OS != o.OS {
		return false
	}
	roles := r.Roles
	switch {
	case len(r.Roles) == 0:
		roles = o.Roles
	case len(o.Roles) > 0:
		roles = nil
		for _, role := range r.Roles {
			if slices.Contains(o.Roles, role) {
				roles = append(roles, role)
			}
		}
		if len(roles) == 0 {
			return false
		}
	}
	osType := cmp.Or(r.OS, o.OS)
	if !(r.Apps || o.Apps) || osType == "" {
		return true
	}
	if len(roles) == 0 {
		roles = []SystemRole{RoleServer, RoleWorkstation, RoleDC, RoleRouter}
	}
	for _, role := range roles {
		if len(appTemplates(osType, role)) > 0 {
			return true
		}
	}
	return false
}

func (r HostRequirement) accepts(s *SystemIdentity) bool {
	if r.OS != "" && s.OSInfo.Type != r.OS {
		return false
	}
	if r.Apps && len(appTemplates(s.OSInfo.Type, s.Role)) == 0 {
		return false
	}
	return len(r.Roles) == 0 || slices.Contains(r.Roles, s.Role)
}

// SystemForKey is SystemFor with no requirement.
func (e *Environment) SystemForKey(key string) *SystemIdentity {
	return e.SystemFor(key, 0, HostRequirement{})
}

// SystemFor deterministically selects, by a caller-supplied key (typically a
// generator component name) and ordinal (the entry's position among
// generators of that key, 0 for the first), one of the environment's Systems
// that meets req, skipping systems already selected for another ordinal of
// the same key so same-key entries describe distinct hosts.
// The same key, ordinal, and requirement always map to the same system for a
// given Environment and call order (loaders call in config order), so a
// generator resolves its host identity once and attributes every record it
// emits consistently. Returns nil when no system meets req.
func (e *Environment) SystemFor(key string, ordinal int, req HostRequirement) *SystemIdentity {
	return pick(e, key, ordinal, e.Systems, req.accepts)
}

// pick deterministically selects, by key and ordinal, one of the candidates
// that accepts admits, skipping any already selected for another ordinal of
// the same key so same-key entries get distinct hosts or devices. A repeated
// key and ordinal returns its earlier selection. Returns the zero value when
// no candidate is admitted, and shares a taken one when every admitted
// candidate is taken, rather than fail.
func pick[T comparable](e *Environment, key string, ordinal int, candidates []T, accepts func(T) bool) T {
	var eligible []T
	for _, c := range candidates {
		if accepts(c) {
			eligible = append(eligible, c)
		}
	}
	var zero T
	if len(eligible) == 0 {
		return zero
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(key))
	// int64 throughout: uint32->int64 and int->int64 are widening (never
	// negative, never truncating), so this is correct on 32-bit targets and
	// avoids an int->uint32 narrowing conversion.
	// ordinal offsets same-key entries (two hostmetrics entries in one
	// generators: list) onto distinct candidates while eligible ones remain.
	idx := (int64(h.Sum32()) + int64(ordinal)) % int64(len(eligible))

	e.picksMu.Lock()
	defer e.picksMu.Unlock()
	if p, ok := e.picks[key][ordinal].(T); ok && accepts(p) {
		return p
	}
	taken := func(c T) bool {
		for o, p := range e.picks[key] {
			if o != ordinal && p == any(c) {
				return true
			}
		}
		return false
	}
	chosen := eligible[idx] // every eligible candidate taken: share rather than fail
	for i := range int64(len(eligible)) {
		if c := eligible[(idx+i)%int64(len(eligible))]; !taken(c) {
			chosen = c
			break
		}
	}
	if e.picks == nil {
		e.picks = map[string]map[int]any{}
	}
	if e.picks[key] == nil {
		e.picks[key] = map[int]any{}
	}
	e.picks[key][ordinal] = chosen
	return chosen
}

// EnvironmentOpts controls the size and shape of the generated environment.
type EnvironmentOpts struct {
	DomainName         string // e.g., "contoso.com". Default: "blitz.local"
	SystemCount        int    // number of machines. Default: 20
	UserCount          int    // number of users. Default: 50
	GroupCount         int    // number of groups. Default: 10
	NetworkCount       int    // number of subnets. Default: 4. Values beyond the built-in default catalog are synthesized using IdentityNetworks.
	StorageSystemCount int    // number of storage arrays. Default: 2.
	NetworkSystemCount int    // number of network devices. Default: 4.
	// RequiredVendors are appliance vendors the configured generators
	// describe, one per entry; the environment holds at least as many devices
	// of each vendor as it is listed.
	RequiredVendors []ApplianceVendor
	// RequiredHosts are hosts the configured generators describe (a domain
	// controller, a macOS host), one per entry; the environment holds at least
	// as many hosts meeting each requirement as it is listed.
	RequiredHosts []HostRequirement
	// RequiredRouters is how many routing (L3) devices of any vendor the
	// configured generators describe, such as flow exporters; the environment
	// holds at least that many.
	RequiredRouters   int
	DomainAdminsCount int         // exact Domain Admins membership; 0 (or negative) = use the user-count-scaled default in the datagen package.
	Now               time.Time   // wall-clock anchor for time-dependent fields (e.g. CertAuthority validity window). Zero value = time.Now() at GenerateEnvironment call; see Environment docstring for determinism implications.
	Logger            *zap.Logger // logger for datagen diagnostics; nil disables logging (no global-logger fallback).
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
	systems, err := genSystems(systemSeed, servicesSeed, applicationsSeed, opts.SystemCount, opts.RequiredHosts, domain, networks, opts.Logger)
	if err != nil {
		return nil, err
	}

	// Appliance identities (PIPE-1035): storage arrays and network hardware,
	// each with its own seed and a management interface bound to a subnet.
	storageSeed := seeds.ResolveSeed(IdentityStorageSystems)
	storageSystems := generateStorageSystems(storageSeed, opts.StorageSystemCount, networks)

	networkSystemSeed := seeds.ResolveSeed(IdentityNetworkSystems)
	networkSystems := generateNetworkSystems(networkSystemSeed, opts.NetworkSystemCount, networks, opts.RequiredVendors, opts.RequiredRouters)
	for _, n := range networkSystems {
		n.FQDN = fqdnFor(n.Hostname, domain.Name)
	}

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
// Devices for required vendors the random draw is short of are appended from
// the same RNG after the drawn fleet is bound, so the fleet stays
// deterministic per seed and requiring a vendor never moves a drawn device.
// Every device then gets a unique hostname.
func generateNetworkSystems(seed int64, count int, networks []*NetworkIdentity, required []ApplianceVendor, routers int) []*NetworkSystemIdentity {
	r := rand.New(rand.NewSource(seed)) // #nosec G404
	mgmt := managementNetwork(networks)
	out := make([]*NetworkSystemIdentity, 0, count+len(required))
	add := func(n *NetworkSystemIdentity) {
		bindManagementInterface(r, n.ManagementInterface, mgmt)
		out = append(out, n)
	}
	for i := 0; i < count; i++ {
		add(RandomNetworkSystemIdentity(r))
	}
	need := map[ApplianceVendor]int{}
	for _, v := range required {
		need[v]++
		if vendorCount(out, v) < need[v] {
			add(NetworkSystemIdentityFor(r, v))
		}
	}
	for routerCount(out) < routers {
		add(RandomNetworkSystemIdentity(r)) // every network model routes (L3)
	}
	assignApplianceHostnames(out)
	return out
}

func routerCount(ns []*NetworkSystemIdentity) int {
	n := 0
	for _, s := range ns {
		if s.L3Routing != nil {
			n++
		}
	}
	return n
}

func vendorCount(ns []*NetworkSystemIdentity, v ApplianceVendor) int {
	n := 0
	for _, s := range ns {
		if s.Vendor == v {
			n++
		}
	}
	return n
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

// fleetSpecs and fleetWeights are the drawn fleet's OS/role distribution:
// ~10% DC, ~40% Windows server, ~30% Linux server, ~15% workstation, ~5%
// router.
var (
	fleetSpecs = []osRoleSpec{
		{OSWindows, RoleDC, GreekNames},
		{OSWindows, RoleServer, RomanNames},
		{OSLinux, RoleServer, NorseNames},
		{OSWindows, RoleWorkstation, RomanNames},
		{OSLinux, RoleRouter, EgyptianNames},
	}
	fleetWeights = []float64{0.10, 0.40, 0.30, 0.15, 0.05}
)

// generateSystems creates systems with a realistic OS/role distribution.
// systemSeed seeds the core identity RNG (OS choice, hostname, interface,
// resource specs). servicesSeed and applicationsSeed seed independent RNGs
// for service and application generation so changes to either seed only
// re-randomize that slice.
func generateSystems(systemSeed, servicesSeed, applicationsSeed int64, count int, required []HostRequirement, domain *DomainIdentity, networks []*NetworkIdentity, logger *zap.Logger) ([]*SystemIdentity, error) {
	r := rand.New(rand.NewSource(systemSeed))                   // #nosec G404
	rServices := rand.New(rand.NewSource(servicesSeed))         // #nosec G404
	rApplications := rand.New(rand.NewSource(applicationsSeed)) // #nosec G404

	specs, weights := fleetSpecs, fleetWeights

	// Prod-tier baseline OS release per family, chosen once so every prod host
	// of a family is pinned to the same conservative release (real fleets keep
	// prod uniform). Non-prod hosts roll newer and vary per host.
	prodBaseline := map[OSType]OSInfo{
		OSLinux:   osInfoForTier(r, OSLinux, RoleServer, true),
		OSWindows: osInfoForTier(r, OSWindows, RoleServer, true),
		OSMacOS:   osInfoForTier(r, OSMacOS, RoleWorkstation, true),
	}
	// Domain controllers run Windows Server, so they get their own baseline.
	prodDCBaseline := osInfoForTier(r, OSWindows, RoleDC, true)

	build := func(spec osRoleSpec) (*SystemIdentity, error) {
		sys, err := GenerateSystemIdentity(r, spec.os, spec.role, domain, spec.pool)
		if err != nil {
			return nil, err
		}

		// Assign a deployment tier and cluster the OS release by it: prod hosts
		// share the pinned family baseline; non-prod hosts roll newer and vary.
		sys.Tier = weightedSelect(r, DeploymentTiers, deploymentTierWeights)
		switch {
		case sys.Tier == TierProd && spec.role == RoleDC:
			sys.OSInfo = prodDCBaseline
		case sys.Tier == TierProd:
			info, ok := prodBaseline[spec.os]
			if !ok {
				// The OSes beyond Linux, Windows, and macOS (the hypervisor
				// hosts and Unix families) get their baseline on first use.
				info = osInfoForTier(r, spec.os, spec.role, true)
				prodBaseline[spec.os] = info
			}
			sys.OSInfo = info
		default:
			sys.OSInfo = osInfoForTier(r, spec.os, spec.role, false)
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
		return sys, nil
	}

	systems := make([]*SystemIdentity, 0, count+len(required))
	for i := 0; i < count; i++ {
		// Pick OS/role using weighted selection
		sys, err := build(weightedSelect(r, specs, weights))
		if err != nil {
			return nil, err
		}
		systems = append(systems, sys)
	}
	// Each entry's requirement needs its own host: an entry needs one host
	// beyond every earlier entry whose requirement overlaps its own, since
	// any of those may have taken a host it accepts. Hosts the random draw is short of
	// are appended from the same RNGs, so the drawn systems stay unchanged and
	// the fleet stays deterministic per seed.
	for i, req := range required {
		need := 1
		for _, prev := range required[:i] {
			if prev.overlaps(req) {
				need++
			}
		}
		have := 0
		for _, s := range systems {
			if req.accepts(s) {
				have++
			}
		}
		for ; have < need; have++ {
			sys, err := build(specFor(r, req))
			if err != nil {
				return nil, err
			}
			systems = append(systems, sys)
		}
	}

	return systems, nil
}

// specFor is the OS, role, and hostname pool of a host that meets req. When
// fleet specs fit req (OS, roles, and applications), it draws one by the
// fleet's weights, so a requirement allowing several roles gets them in fleet
// proportions. Otherwise it builds one: req's first role (a server by
// default, a workstation for macOS) and OS (by default Windows for a DC or
// workstation, Linux otherwise).
func specFor(r *rand.Rand, req HostRequirement) osRoleSpec {
	var specs []osRoleSpec
	var weights []float64
	for i, spec := range fleetSpecs {
		if (req.OS == "" || req.OS == spec.os) &&
			(len(req.Roles) == 0 || slices.Contains(req.Roles, spec.role)) &&
			(!req.Apps || len(appTemplates(spec.os, spec.role)) > 0) {
			specs = append(specs, spec)
			weights = append(weights, fleetWeights[i])
		}
	}
	if len(specs) > 0 {
		return weightedSelect(r, specs, weights)
	}
	role := RoleServer
	if len(req.Roles) > 0 {
		role = req.Roles[0]
	} else if req.OS == OSMacOS {
		role = RoleWorkstation
	}
	os := req.OS
	// Safety net, unreachable today: every role has a fleet spec that runs
	// applications, so a requirement without an OS always fits one. Kept so a
	// future role or application rule still gets a sensible host.
	if os == "" {
		os = OSLinux
		if role == RoleDC || role == RoleWorkstation {
			os = OSWindows
		}
	}
	switch {
	case role == RoleDC:
		return osRoleSpec{os, role, GreekNames}
	case role == RoleRouter:
		return osRoleSpec{os, role, EgyptianNames}
	case role == RoleServer && os == OSLinux:
		// Safety net, unreachable today: the fleet table has a Linux server,
		// which the draw above returns. Kept for a future table change.
		return osRoleSpec{os, role, NorseNames}
	default:
		return osRoleSpec{os, role, RomanNames}
	}
}

// weightedSelect picks an item using weighted random selection.
func weightedSelect[T any](r *rand.Rand, items []T, weights []float64) T {
	total := 0.0
	for _, w := range weights {
		total += w
	}
	roll := r.Float64() * total // #nosec G404 -- weights need not sum to 1
	cumulative := 0.0
	for i, w := range weights {
		cumulative += w
		if roll < cumulative {
			return items[i]
		}
	}
	// Safety net: float rounding can leave roll at the total, past the last
	// cumulative weight.
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
	case OSESXi:
		return "vmk0" // the management VMkernel adapter
	case OSAIX:
		return "en0" // the IP interface over adapter ent0
	case OSSolaris:
		return "net0" // Solaris 11 generic link name
	case OSFreeBSD, OSOpenBSD:
		// BSD interfaces are named after their driver; em(4)
		// (Intel PRO/1000) stands in until NIC models are simulated.
		return "em0"
	case OSXenDom0:
		return "xenbr0" // XCP-ng's management bridge over eth0
	case OSNutanixAHV:
		return "br0" // the AHV host's OVS management bridge
	default:
		return "eth0"
	}
}

// FlowExporterFor deterministically selects, by key and ordinal, one of the
// environment's routing (L3) devices, the device a flow generator's records
// come from: any vendor when vendor is empty, else only that vendor's. Returns
// nil when the environment has none; build it with RequiredRouters or
// RequiredVendors to guarantee one.
func (e *Environment) FlowExporterFor(key string, ordinal int, vendor ApplianceVendor) *NetworkSystemIdentity {
	return pick(e, key, ordinal, e.NetworkSystems, func(n *NetworkSystemIdentity) bool {
		return n.L3Routing != nil && (vendor == "" || n.Vendor == vendor)
	})
}

// ApplianceFor deterministically selects, by key, one of the environment's
// network devices of vendor, the appliance an appliance generator (Palo Alto,
// F5) describes. Returns nil when the environment has none; build it with
// EnvironmentOpts.RequiredVendors to guarantee one.
func (e *Environment) ApplianceFor(key string, ordinal int, vendor ApplianceVendor) *NetworkSystemIdentity {
	return pick(e, key, ordinal, e.NetworkSystems, func(n *NetworkSystemIdentity) bool {
		return n.Vendor == vendor
	})
}
