package datagen

import (
	"fmt"
	"math/rand"
)

// Capability facets. A NetworkSystemIdentity composes zero or more of these;
// a nil facet pointer means the device lacks that capability. Real products
// compose facets (a BIG-IP does load balancing + firewall + L3; a Catalyst
// 9300 does L2 + limited L3).
//
// Deferred facets (per PIPE-927's architecture: VPN, WAN-optimization,
// forward-proxy, DPI) are intentionally not modeled here. Add them when a
// simulator needs them rather than speculatively.

// L2SwitchingCapability describes layer-2 switching.
type L2SwitchingCapability struct {
	VLANCount    int
	MACTableSize int
	STPMode      string // "rstp", "mstp", "pvst+"
}

// L3RoutingCapability describes layer-3 routing.
type L3RoutingCapability struct {
	Protocols   []string // "static", "ospf", "bgp", "isis"
	FIBSize     int
	BGPEnabled  bool
	OSPFEnabled bool
}

// FirewallCapability describes stateful firewalling.
type FirewallCapability struct {
	RuleCount          int
	NAT                bool
	VPNTermination     bool
	StatefulInspection bool
}

// LoadBalancingCapability describes ADC / load-balancing.
type LoadBalancingCapability struct {
	VirtualServers int
	PoolMembers    int
	Persistence    string // "source-addr", "cookie", "ssl-sid"
	SSLOffload     bool
}

// WirelessCapability describes wireless-LAN controller function.
type WirelessCapability struct {
	RadioCount     int
	ControllerMode string // "embedded", "dedicated"
	MaxAPs         int
}

// NetworkSystemIdentity is a first-class network-hardware machine: a
// vendor/model/serial box running an embedded ApplianceOS, with a set of
// composable capability facets, data interfaces, and a management interface.
type NetworkSystemIdentity struct {
	Vendor ApplianceVendor
	Model  string
	Serial string
	OS     *ApplianceOS

	Interfaces          []NetworkInterface
	ManagementInterface *NetworkInterface

	// Capability facets — nil means the device lacks that capability.
	L2Switching   *L2SwitchingCapability
	L3Routing     *L3RoutingCapability
	Firewall      *FirewallCapability
	LoadBalancing *LoadBalancingCapability
	Wireless      *WirelessCapability

	AdminUserRef *UserIdentity
}

// facetMask is a bitset of the capability facets a model composes.
type facetMask uint8

const (
	facetL2 facetMask = 1 << iota
	facetL3
	facetFirewall
	facetLB
	facetWireless
)

// networkModelSpec describes a concrete network model: its vendor, OS family,
// the facets it composes, and a data-interface (port) count range.
type networkModelSpec struct {
	vendor   ApplianceVendor
	model    string
	osFamily ApplianceOSFamily
	facets   facetMask
	minPorts int
	maxPorts int
}

// networkModels is the first vendor pool spanning the seven appliance vendors.
// Facet composition mirrors the real product's role (a Catalyst 9300 is an
// access switch: L2 + limited L3; a PA-3220 is an NGFW: firewall + L3).
var networkModels = []networkModelSpec{
	{VendorCisco, "Catalyst 9300-48P", FamilyIOSXE, facetL2 | facetL3, 48, 48},
	{VendorCisco, "Catalyst 9500-48Y4C", FamilyIOSXE, facetL2 | facetL3, 48, 52},
	{VendorCisco, "Nexus 9336C-FX2", FamilyNXOS, facetL2 | facetL3, 36, 36},
	{VendorCisco, "Catalyst 9800-40 WLC", FamilyIOSXE, facetWireless | facetL3, 4, 8},
	{VendorArista, "DCS-7050SX3-48YC8", FamilyEOS, facetL2 | facetL3, 48, 56},
	{VendorArista, "DCS-7280SR3-48YC8", FamilyEOS, facetL2 | facetL3, 48, 56},
	{VendorJuniper, "EX4300-48T", FamilyJunos, facetL2 | facetL3, 48, 48},
	{VendorJuniper, "SRX1500", FamilyJunos, facetFirewall | facetL3, 16, 16},
	{VendorJuniper, "MX240", FamilyJunos, facetL3, 4, 48},
	{VendorF5, "BIG-IP i5800", FamilyBIGIP, facetLB | facetFirewall | facetL3, 8, 8},
	{VendorPaloAlto, "PA-3220", FamilyPANOS, facetFirewall | facetL3, 12, 12},
	{VendorFortinet, "FortiGate 100F", FamilyFortiOS, facetFirewall | facetL3, 22, 22},
}

// Validate reports whether the NetworkSystemIdentity is well-formed: a
// family-coherent OS vendor, at least one capability facet, and coherent
// interfaces. Returns an error rather than panicking, per the datagen
// error-return convention.
func (n *NetworkSystemIdentity) Validate() error {
	if n.Vendor == "" {
		return fmt.Errorf("network system vendor must not be empty")
	}
	if n.Model == "" {
		return fmt.Errorf("network system model must not be empty")
	}
	if n.Serial == "" {
		return fmt.Errorf("network system serial must not be empty")
	}
	if n.OS == nil {
		return fmt.Errorf("network system %q has nil OS", n.Model)
	}
	if err := n.OS.Validate(); err != nil {
		return fmt.Errorf("network system %q OS: %w", n.Model, err)
	}
	if n.L2Switching == nil && n.L3Routing == nil && n.Firewall == nil && n.LoadBalancing == nil && n.Wireless == nil {
		return fmt.Errorf("network system %q has no capability facets", n.Model)
	}
	if len(n.Interfaces) == 0 {
		return fmt.Errorf("network system %q has no data interfaces", n.Model)
	}
	if n.ManagementInterface == nil {
		return fmt.Errorf("network system %q has nil management interface", n.Model)
	}
	return nil
}

// generateNetworkSystem builds a NetworkSystemIdentity for a specific model
// spec with deterministic output for a given RNG state.
func generateNetworkSystem(r *rand.Rand, spec networkModelSpec) *NetworkSystemIdentity {
	os := GenerateApplianceOS(r, spec.osFamily)

	portCount := randRange(r, spec.minPorts, spec.maxPorts)
	interfaces := make([]NetworkInterface, portCount)
	for i := range interfaces {
		interfaces[i] = NetworkInterface{
			Name:       interfacePortName(spec.vendor, i),
			MACAddress: RandomMAC(r),
		}
	}

	n := &NetworkSystemIdentity{
		Vendor:     spec.vendor,
		Model:      spec.model,
		Serial:     networkSerial(r, spec.vendor),
		OS:         &os,
		Interfaces: interfaces,
		ManagementInterface: &NetworkInterface{
			Name:       "mgmt0",
			IPv4:       RandomPrivateIPv4(r),
			MACAddress: RandomMAC(r),
		},
	}

	if spec.facets.has(facetL2) {
		n.L2Switching = generateL2Switching(r)
	}
	if spec.facets.has(facetL3) {
		n.L3Routing = generateL3Routing(r)
	}
	if spec.facets.has(facetFirewall) {
		n.Firewall = generateFirewall(r)
	}
	if spec.facets.has(facetLB) {
		n.LoadBalancing = generateLoadBalancing(r)
	}
	if spec.facets.has(facetWireless) {
		n.Wireless = generateWireless(r)
	}
	return n
}

// RandomNetworkSystemIdentity returns a network device drawn at random from the
// built-in vendor pool.
func RandomNetworkSystemIdentity(r *rand.Rand) *NetworkSystemIdentity {
	spec := networkModels[r.Intn(len(networkModels))] // #nosec G404
	return generateNetworkSystem(r, spec)
}

func generateL2Switching(r *rand.Rand) *L2SwitchingCapability {
	modes := []string{"rstp", "mstp", "pvst+"}
	macSizes := []int{16000, 32000, 64000, 96000}
	return &L2SwitchingCapability{
		VLANCount:    randRange(r, 8, 512),
		MACTableSize: macSizes[r.Intn(len(macSizes))], // #nosec G404
		STPMode:      modes[r.Intn(len(modes))],       // #nosec G404
	}
}

func generateL3Routing(r *rand.Rand) *L3RoutingCapability {
	bgp := r.Intn(2) == 0  // #nosec G404
	ospf := r.Intn(2) == 0 // #nosec G404
	protocols := []string{"static", "connected"}
	if ospf {
		protocols = append(protocols, "ospf")
	}
	if bgp {
		protocols = append(protocols, "bgp")
	}
	return &L3RoutingCapability{
		Protocols:   protocols,
		FIBSize:     randRange(r, 4000, 256000),
		BGPEnabled:  bgp,
		OSPFEnabled: ospf,
	}
}

func generateFirewall(r *rand.Rand) *FirewallCapability {
	return &FirewallCapability{
		RuleCount:          randRange(r, 50, 5000),
		NAT:                true,
		VPNTermination:     r.Intn(2) == 0, // #nosec G404
		StatefulInspection: true,
	}
}

func generateLoadBalancing(r *rand.Rand) *LoadBalancingCapability {
	modes := []string{"source-addr", "cookie", "ssl-sid"}
	return &LoadBalancingCapability{
		VirtualServers: randRange(r, 1, 200),
		PoolMembers:    randRange(r, 2, 500),
		Persistence:    modes[r.Intn(len(modes))], // #nosec G404
		SSLOffload:     true,
	}
}

func generateWireless(r *rand.Rand) *WirelessCapability {
	maxAPs := []int{50, 100, 500, 1000}
	return &WirelessCapability{
		RadioCount:     randRange(r, 2, 8),
		ControllerMode: "embedded",
		MaxAPs:         maxAPs[r.Intn(len(maxAPs))], // #nosec G404
	}
}

// randomAlnumUpper returns n uppercase-alphanumeric characters.
func randomAlnumUpper(r *rand.Rand, n int) string {
	const cs = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = cs[r.Intn(len(cs))] // #nosec G404
	}
	return string(b)
}

// randomDigits returns n decimal digits.
func randomDigits(r *rand.Rand, n int) string {
	const cs = "0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = cs[r.Intn(len(cs))] // #nosec G404
	}
	return string(b)
}

// networkSerial returns a vendor-plausible serial number. Formats are
// representative of each vendor's real serials, not authoritative.
func networkSerial(r *rand.Rand, vendor ApplianceVendor) string {
	switch vendor {
	case VendorCisco:
		return "FCW" + randomAlnumUpper(r, 8)
	case VendorArista:
		return "JPE" + randomDigits(r, 8)
	case VendorJuniper:
		return "JN" + randomAlnumUpper(r, 10)
	case VendorF5:
		return "f5-" + randomHex(r, 6)
	case VendorPaloAlto:
		return randomDigits(r, 12)
	case VendorFortinet:
		return "FGT" + randomDigits(r, 11)
	default:
		return randomAlnumUpper(r, 12)
	}
}

// hasFacet reports whether the mask includes the given facet.
func (m facetMask) has(f facetMask) bool { return m&f != 0 }

// interfacePortName returns a vendor-conventional data-port name for index i.
func interfacePortName(vendor ApplianceVendor, i int) string {
	switch vendor {
	case VendorCisco:
		return fmt.Sprintf("GigabitEthernet1/0/%d", i+1)
	case VendorArista:
		return fmt.Sprintf("Ethernet%d", i+1)
	case VendorJuniper:
		return fmt.Sprintf("ge-0/0/%d", i)
	case VendorF5:
		return fmt.Sprintf("1.%d", i+1)
	default:
		return fmt.Sprintf("port%d", i+1)
	}
}
