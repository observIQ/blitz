package datagen

import (
	"encoding/binary"
	"fmt"
	"math/rand"
	"net"
)

// NetworkIdentity represents a network subnet/VLAN in the simulated environment.
type NetworkIdentity struct {
	ID          string   // "net-01"
	Name        string   // "Server-VLAN"
	CIDR        string   // "10.10.1.0/24"
	Gateway     string   // "10.10.1.1"
	VLAN        int      // 100
	DNSServers  []string // references DC system IPs
	DHCPEnabled bool
	Zone        string // firewall zone: "trust", "untrust", "dmz", "management"
}

// CommonPorts is a pool of commonly used network ports.
var CommonPorts = NewPool(
	22, 25, 53, 80, 110, 143, 443, 993, 995,
	3306, 3389, 5432, 5985, 5986, 6379, 8080, 8443, 9200, 27017,
)

// TCPUDPProtocols is a pool of common transport protocols.
var TCPUDPProtocols = NewPool("tcp", "udp", "icmp")

// minNetworkPrefixIPv4 is the smallest IPv4 prefix length blitz treats as a
// "subnet with hosts". /30 (2 hosts), /31 (RFC 3021 router P2P), and /32
// (single host route) are not modeled as host-bearing subnets in blitz's
// simulation context.
const minNetworkPrefixIPv4 = 29

// reservedIPv4Block names an RFC-attributed range of IPv4 addresses that
// RandomPublicIPv4 refuses to emit. Add a new entry to extend coverage as
// new RFCs reserve additional blocks.
type reservedIPv4Block struct {
	rfc    string       // e.g. "RFC 6598"
	name   string       // human-readable purpose
	docURL string       // ietf.org datatracker URL
	cidrs  []*net.IPNet // parsed once at package init
}

// reservedIPv4Blocks lists IANA / IETF special-purpose IPv4 prefixes that
// RandomPublicIPv4 will not emit. RFC 6890 is the umbrella special-purpose
// address registry; entries below cite their originating RFC for traceability
// rather than referencing 6890 directly. Multicast (224.0.0.0/4) and Class E
// (240.0.0.0/4) are also excluded by RandomPublicIPv4's first-octet cap, but
// Class E is listed below under RFC 1112 for completeness; multicast is left
// off because it would never reach the post-generate filter.
var reservedIPv4Blocks = []reservedIPv4Block{
	{
		rfc:    "RFC 1112",
		name:   "Class E reserved",
		docURL: "https://datatracker.ietf.org/doc/html/rfc1112",
		cidrs:  mustParseCIDRs("240.0.0.0/4"),
	},
	{
		rfc:    "RFC 1122",
		name:   "loopback and 'this network'",
		docURL: "https://datatracker.ietf.org/doc/html/rfc1122",
		cidrs:  mustParseCIDRs("0.0.0.0/8", "127.0.0.0/8"),
	},
	{
		rfc:    "RFC 1918",
		name:   "private-use IPv4",
		docURL: "https://datatracker.ietf.org/doc/html/rfc1918",
		cidrs:  mustParseCIDRs("10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16"),
	},
	{
		rfc:    "RFC 2544",
		name:   "benchmark testing",
		docURL: "https://datatracker.ietf.org/doc/html/rfc2544",
		cidrs:  mustParseCIDRs("198.18.0.0/15"),
	},
	{
		rfc:    "RFC 3927",
		name:   "link-local IPv4",
		docURL: "https://datatracker.ietf.org/doc/html/rfc3927",
		cidrs:  mustParseCIDRs("169.254.0.0/16"),
	},
	{
		rfc:    "RFC 5736",
		name:   "IETF protocol assignments",
		docURL: "https://datatracker.ietf.org/doc/html/rfc5736",
		cidrs:  mustParseCIDRs("192.0.0.0/24"),
	},
	{
		rfc:    "RFC 5737",
		name:   "documentation TEST-NET-1/2/3",
		docURL: "https://datatracker.ietf.org/doc/html/rfc5737",
		cidrs:  mustParseCIDRs("192.0.2.0/24", "198.51.100.0/24", "203.0.113.0/24"),
	},
	{
		rfc:    "RFC 6598",
		name:   "CGNAT shared address space",
		docURL: "https://datatracker.ietf.org/doc/html/rfc6598",
		cidrs:  mustParseCIDRs("100.64.0.0/10"),
	},
}

// reservedIPv6Block is the IPv6 parallel to reservedIPv4Block. The struct
// exists now so future IPv6 random-public emission work has the paradigm in
// place; see PIPE-1001 for the ticket that will populate reservedIPv6Blocks
// and add RandomPublicIPv6 / RandomIPInCIDRv6.
type reservedIPv6Block struct {
	rfc    string
	name   string
	docURL string
	cidrs  []*net.IPNet
}

// reservedIPv6Blocks lists IANA / IETF special-purpose IPv6 prefixes that
// RandomPublicIPv6 will not emit. RFC 6890 is the umbrella special-purpose
// registry; entries cite their originating RFC for traceability.
var reservedIPv6Blocks = []reservedIPv6Block{
	{
		rfc:    "RFC 4291",
		name:   "unspecified, loopback, IPv4-mapped, link-local, multicast",
		docURL: "https://datatracker.ietf.org/doc/html/rfc4291",
		cidrs:  mustParseCIDRs("::/128", "::1/128", "::ffff:0:0/96", "fe80::/10", "ff00::/8"),
	},
	{
		rfc:    "RFC 4193",
		name:   "unique local addresses",
		docURL: "https://datatracker.ietf.org/doc/html/rfc4193",
		cidrs:  mustParseCIDRs("fc00::/7"),
	},
	{
		rfc:    "RFC 3849",
		name:   "documentation",
		docURL: "https://datatracker.ietf.org/doc/html/rfc3849",
		cidrs:  mustParseCIDRs("2001:db8::/32"),
	},
	{
		rfc:    "RFC 5180",
		name:   "benchmarking",
		docURL: "https://datatracker.ietf.org/doc/html/rfc5180",
		cidrs:  mustParseCIDRs("2001:2::/48"),
	},
	{
		rfc:    "RFC 6052",
		name:   "IPv4/IPv6 translation",
		docURL: "https://datatracker.ietf.org/doc/html/rfc6052",
		cidrs:  mustParseCIDRs("64:ff9b::/96"),
	},
	{
		rfc:    "RFC 6666",
		name:   "discard-only prefix",
		docURL: "https://datatracker.ietf.org/doc/html/rfc6666",
		cidrs:  mustParseCIDRs("100::/64"),
	},
}

// minNetworkPrefixIPv6 is the smallest IPv6 prefix length blitz treats as a
// host-bearing subnet. Prefixes longer than /64 (e.g. /127 point-to-point
// links, /128 host routes) are not modeled as subnets-with-hosts here.
const minNetworkPrefixIPv6 = 64

// isReservedIPv6 reports whether ip falls in any reservedIPv6Blocks entry.
func isReservedIPv6(ip net.IP) bool {
	for _, block := range reservedIPv6Blocks {
		for _, cidr := range block.cidrs {
			if cidr.Contains(ip) {
				return true
			}
		}
	}
	return false
}

// ValidateIPv6CIDR returns nil if cidr is a parseable IPv6 CIDR with a prefix
// of /64 or shorter. It errors on unparseable input, IPv4 input, and prefixes
// longer than /64.
func ValidateIPv6CIDR(cidr string) error {
	ip, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		return fmt.Errorf("invalid CIDR %q: %w", cidr, err)
	}
	if ip.To4() != nil {
		return fmt.Errorf("CIDR %q is IPv4, not IPv6", cidr)
	}
	ones, _ := ipNet.Mask.Size()
	if ones > minNetworkPrefixIPv6 {
		return fmt.Errorf("CIDR %q has prefix /%d; blitz IPv6 networks require /%d or shorter", cidr, ones, minNetworkPrefixIPv6)
	}
	return nil
}

// RandomPublicIPv6 generates a random global-unicast (2000::/3) IPv6 address
// that is not in any reserved block.
func RandomPublicIPv6(r *rand.Rand) string {
	return randomPublicIPv6(r, isReservedIPv6)
}

// randomPublicIPv6 is the testable core of RandomPublicIPv6: it takes the
// reserved-check as a parameter so the reject-and-retry path can be exercised
// deterministically (the reserved sub-blocks inside 2000::/3 are too sparse to
// hit by chance).
func randomPublicIPv6(r *rand.Rand, reserved func(net.IP) bool) string {
	for {
		var b [16]byte
		for i := range b {
			b[i] = byte(r.Intn(256)) // #nosec G404
		}
		// Force global-unicast 2000::/3: set the top 3 bits to 001.
		b[0] = (b[0] & 0x1f) | 0x20
		ip := net.IP(b[:])
		if reserved(ip) {
			continue
		}
		return ip.String()
	}
}

// RandomIPInCIDRv6 returns a random address within the given IPv6 CIDR. For an
// unparseable or non-IPv6 CIDR it falls back to RandomPublicIPv6.
func RandomIPInCIDRv6(r *rand.Rand, cidr string) string {
	ip, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		return RandomPublicIPv6(r)
	}
	if ip.To4() != nil {
		return RandomPublicIPv6(r)
	}
	base := ipNet.IP.To16()
	mask := ipNet.Mask
	result := make(net.IP, 16)
	for i := 0; i < 16; i++ {
		result[i] = (base[i] & mask[i]) | (byte(r.Intn(256)) &^ mask[i]) // #nosec G404
	}
	return result.String()
}

// mustParseCIDRs parses the given CIDR strings at package init time. Bad
// input here means a typo in a hardcoded literal in this file, so panicking
// is the right failure mode — it surfaces immediately on the first test run.
func mustParseCIDRs(cidrs ...string) []*net.IPNet {
	out := make([]*net.IPNet, 0, len(cidrs))
	for _, c := range cidrs {
		_, ipNet, err := net.ParseCIDR(c)
		if err != nil {
			panic(fmt.Sprintf("datagen: invalid CIDR literal %q: %v", c, err))
		}
		out = append(out, ipNet)
	}
	return out
}

// isReservedIPv4 reports whether the given IPv4 address falls in any block
// listed in reservedIPv4Blocks.
func isReservedIPv4(ip net.IP) bool {
	for _, block := range reservedIPv4Blocks {
		for _, cidr := range block.cidrs {
			if cidr.Contains(ip) {
				return true
			}
		}
	}
	return false
}

// ValidateCIDR returns nil if cidr is a parseable IPv4 CIDR with a prefix
// length suitable for use as a blitz network (≤ /29, i.e. at least 8 total
// addresses / 6 usable hosts). It returns an error for unparseable input,
// non-IPv4 input, and prefix lengths /30, /31, /32 — which represent
// point-to-point links and host routes rather than subnets-with-hosts in
// blitz's simulation context.
//
// Config-loading code paths that accept user-supplied CIDRs (e.g. via YAML or
// CLI overrides) MUST call this and reject the entire config on failure.
// PIPE-1002 tracks wiring the call sites once user-configurable networks
// land. RandomIPInCIDR retains a soft fallback for defense in depth, but
// validation is the strict gate.
func ValidateCIDR(cidr string) error {
	ip, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		return fmt.Errorf("invalid CIDR %q: %w", cidr, err)
	}
	if ip.To4() == nil {
		return fmt.Errorf("CIDR %q is not IPv4; IPv6 support is tracked in PIPE-1001", cidr)
	}
	ones, bits := ipNet.Mask.Size()
	if bits != 32 {
		return fmt.Errorf("CIDR %q has non-IPv4 mask width %d", cidr, bits)
	}
	if ones > minNetworkPrefixIPv4 {
		return fmt.Errorf("CIDR %q has prefix /%d; blitz networks require /%d or shorter (≥ 6 usable hosts)", cidr, ones, minNetworkPrefixIPv4)
	}
	return nil
}

// Validate checks the NetworkIdentity for blitz-compatible configuration.
// Currently validates CIDR; extend as additional invariants are added.
func (n *NetworkIdentity) Validate() error {
	return ValidateCIDR(n.CIDR)
}

// GenerateDefaultNetworks returns the standard set of network identities.
func GenerateDefaultNetworks() []*NetworkIdentity {
	return []*NetworkIdentity{
		{
			ID:          "net-01",
			Name:        "Server-VLAN",
			CIDR:        "10.10.1.0/24",
			Gateway:     "10.10.1.1",
			VLAN:        100,
			DHCPEnabled: false,
			Zone:        "trust",
		},
		{
			ID:          "net-02",
			Name:        "Workstation-LAN",
			CIDR:        "10.10.10.0/24",
			Gateway:     "10.10.10.1",
			VLAN:        200,
			DHCPEnabled: true,
			Zone:        "trust",
		},
		{
			ID:          "net-03",
			Name:        "DMZ",
			CIDR:        "10.10.20.0/24",
			Gateway:     "10.10.20.1",
			VLAN:        300,
			DHCPEnabled: false,
			Zone:        "dmz",
		},
		{
			ID:          "net-04",
			Name:        "Management",
			CIDR:        "10.10.99.0/24",
			Gateway:     "10.10.99.1",
			VLAN:        999,
			DHCPEnabled: false,
			Zone:        "management",
		},
	}
}

// RandomIPv4 generates a random IPv4 address (not tied to any NetworkIdentity).
func RandomIPv4(r *rand.Rand) string {
	return fmt.Sprintf("%d.%d.%d.%d",
		r.Intn(256), // #nosec G404
		r.Intn(256), // #nosec G404
		r.Intn(256), // #nosec G404
		r.Intn(256)) // #nosec G404
}

// RandomPrivateIPv4 generates a random RFC1918 private IPv4 address.
func RandomPrivateIPv4(r *rand.Rand) string {
	// Pick a random RFC1918 range
	switch r.Intn(3) { // #nosec G404
	case 0: // 10.0.0.0/8
		return fmt.Sprintf("10.%d.%d.%d", r.Intn(256), r.Intn(256), r.Intn(254)+1) // #nosec G404
	case 1: // 172.16.0.0/12
		return fmt.Sprintf("172.%d.%d.%d", 16+r.Intn(16), r.Intn(256), r.Intn(254)+1) // #nosec G404
	default: // 192.168.0.0/16
		return fmt.Sprintf("192.168.%d.%d", r.Intn(256), r.Intn(254)+1) // #nosec G404
	}
}

// RandomPublicIPv4 generates a random IPv4 address that is not in any of the
// reserved blocks listed in reservedIPv4Blocks. The first-octet cap of 224
// also excludes multicast (224.0.0.0/4) and Class E (240.0.0.0/4) up front,
// reducing post-generate rejections.
func RandomPublicIPv4(r *rand.Rand) string {
	for {
		a := r.Intn(224) + 1 // #nosec G404 — avoid 0.x.x.x and multicast (224+)
		b := r.Intn(256)     // #nosec G404
		c := r.Intn(256)     // #nosec G404
		d := r.Intn(254) + 1 // #nosec G404

		ip := net.IPv4(byte(a), byte(b), byte(c), byte(d))
		if isReservedIPv4(ip) {
			continue
		}
		// Belt-and-suspenders: the 224 cap should make IsMulticast unreachable,
		// but keep it in case the cap changes.
		if ip.IsMulticast() {
			continue
		}
		return fmt.Sprintf("%d.%d.%d.%d", a, b, c, d)
	}
}

// RandomIPv6 generates a random IPv6 address.
func RandomIPv6(r *rand.Rand) string {
	var groups [8]uint16
	for i := range groups {
		groups[i] = uint16(r.Intn(65536)) // #nosec G404,G115
	}
	return fmt.Sprintf("%x:%x:%x:%x:%x:%x:%x:%x",
		groups[0], groups[1], groups[2], groups[3],
		groups[4], groups[5], groups[6], groups[7])
}

// RandomMAC generates a random MAC address with locally administered bit set.
func RandomMAC(r *rand.Rand) string {
	var mac [6]byte
	for i := range mac {
		mac[i] = byte(r.Intn(256)) // #nosec G404
	}
	// Set locally administered bit, clear multicast bit
	mac[0] = (mac[0] | 0x02) & 0xfe
	return fmt.Sprintf("%02x:%02x:%02x:%02x:%02x:%02x",
		mac[0], mac[1], mac[2], mac[3], mac[4], mac[5])
}

// RandomIPInCIDR returns a random host address within the given IPv4 CIDR,
// excluding the network and broadcast addresses for /28 and shorter prefixes.
//
// Blitz expects callers to validate the CIDR via ValidateCIDR or
// NetworkIdentity.Validate before runtime; the strict gate is config-load
// validation (see PIPE-1002). As a defense-in-depth fallback for unvalidated
// inputs, this function returns a random IPv4 from RandomIPv4(r) when the
// CIDR is unparseable, not IPv4, or has a prefix length outside the
// supported /29-or-shorter range. Reaching the fallback indicates a
// validation gap, not user-facing behavior to rely on.
func RandomIPInCIDR(r *rand.Rand, cidr string) string {
	ip, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		return RandomIPv4(r)
	}

	ones, bits := ipNet.Mask.Size()
	if bits != 32 {
		return RandomIPv4(r)
	}
	if ones > minNetworkPrefixIPv4 {
		// /30, /31, /32 — not subnets-with-hosts in blitz's context.
		return RandomIPv4(r)
	}

	networkIP := binary.BigEndian.Uint32(ip.To4())
	hostBits := 32 - ones
	maxHost := (1 << hostBits) - 2 // exclude network and broadcast

	host := uint32(r.Intn(maxHost)) + 1 // #nosec G404,G115
	resultIP := make(net.IP, 4)
	binary.BigEndian.PutUint32(resultIP, networkIP+host)
	return resultIP.String()
}
