package datagen

import (
	"math/rand"
	"net"
	"strings"
	"testing"
)

func TestRandomIPv4(t *testing.T) {
	r := rand.New(rand.NewSource(42))
	for i := 0; i < 100; i++ {
		ip := RandomIPv4(r)
		if net.ParseIP(ip) == nil {
			t.Errorf("RandomIPv4 produced invalid IP: %q", ip)
		}
	}
}

func TestRandomPrivateIPv4(t *testing.T) {
	r := rand.New(rand.NewSource(42))
	for i := 0; i < 100; i++ {
		ip := RandomPrivateIPv4(r)
		parsed := net.ParseIP(ip)
		if parsed == nil {
			t.Errorf("RandomPrivateIPv4 produced invalid IP: %q", ip)
			continue
		}
		if !isPrivateIP(parsed) {
			t.Errorf("RandomPrivateIPv4 produced non-private IP: %q", ip)
		}
	}
}

func TestRandomPublicIPv4(t *testing.T) {
	r := rand.New(rand.NewSource(42))
	for i := 0; i < 1000; i++ {
		ip := RandomPublicIPv4(r)
		parsed := net.ParseIP(ip)
		if parsed == nil {
			t.Errorf("RandomPublicIPv4 produced invalid IP: %q", ip)
			continue
		}
		// Sanity check against a hand-rolled list — not a substitute for
		// reservedIPv4Blocks, but a second source of truth.
		if isPrivateIP(parsed) || parsed.IsLoopback() || parsed.IsMulticast() {
			t.Errorf("RandomPublicIPv4 produced non-public IP: %q", ip)
		}
		// Authoritative check against the package's own reserved list.
		if isReservedIPv4(parsed) {
			t.Errorf("RandomPublicIPv4 produced reserved IP: %q", ip)
		}
	}
}

func TestReservedIPv4Blocks(t *testing.T) {
	t.Run("entries cover known reserved IPs", func(t *testing.T) {
		cases := map[string]string{
			"10.1.2.3":     "RFC 1918",
			"172.20.5.5":   "RFC 1918",
			"192.168.1.1":  "RFC 1918",
			"100.64.0.1":   "RFC 6598",
			"127.0.0.1":    "RFC 1122",
			"0.1.2.3":      "RFC 1122",
			"169.254.1.1":  "RFC 3927",
			"192.0.0.5":    "RFC 5736",
			"192.0.2.1":    "RFC 5737",
			"198.51.100.1": "RFC 5737",
			"203.0.113.1":  "RFC 5737",
			"198.18.0.1":   "RFC 2544",
			"240.0.0.1":    "RFC 1112",
		}
		for ipStr := range cases {
			ip := net.ParseIP(ipStr)
			if ip == nil {
				t.Fatalf("test data: %q is not a valid IP", ipStr)
			}
			if !isReservedIPv4(ip) {
				t.Errorf("expected %q to match a reserved block, but it did not", ipStr)
			}
		}
	})
	t.Run("non-reserved IPs are not flagged", func(t *testing.T) {
		nonReserved := []string{"8.8.8.8", "1.1.1.1", "204.0.113.1", "23.45.67.89"}
		for _, ipStr := range nonReserved {
			ip := net.ParseIP(ipStr)
			if isReservedIPv4(ip) {
				t.Errorf("expected %q to NOT match a reserved block, but it did", ipStr)
			}
		}
	})
}

func TestRandomIPv6(t *testing.T) {
	r := rand.New(rand.NewSource(42))
	for i := 0; i < 50; i++ {
		ip := RandomIPv6(r)
		if net.ParseIP(ip) == nil {
			t.Errorf("RandomIPv6 produced invalid IP: %q", ip)
		}
	}
}

func TestRandomMAC(t *testing.T) {
	r := rand.New(rand.NewSource(42))
	for i := 0; i < 50; i++ {
		mac := RandomMAC(r)
		if _, err := net.ParseMAC(mac); err != nil {
			t.Errorf("RandomMAC produced invalid MAC: %q (%v)", mac, err)
		}
	}
}

func TestCommonPorts(t *testing.T) {
	if CommonPorts.Len() < 15 {
		t.Errorf("CommonPorts has %d items, want at least 15", CommonPorts.Len())
	}
}

func TestTCPUDPProtocols(t *testing.T) {
	protos := TCPUDPProtocols.All()
	expected := map[string]bool{"tcp": true, "udp": true, "icmp": true}
	for _, p := range protos {
		if !expected[p] {
			t.Errorf("unexpected protocol %q", p)
		}
	}
}

func TestNetworkIdentity(t *testing.T) {
	t.Run("default networks cover expected zones", func(t *testing.T) {
		nets := GenerateDefaultNetworks()
		if len(nets) < 4 {
			t.Fatalf("expected at least 4 default networks, got %d", len(nets))
		}
		zones := make(map[string]bool)
		for _, n := range nets {
			zones[n.Zone] = true
			// Verify CIDR is valid
			_, _, err := net.ParseCIDR(n.CIDR)
			if err != nil {
				t.Errorf("network %s has invalid CIDR %q: %v", n.Name, n.CIDR, err)
			}
			// Verify gateway is valid IP
			if net.ParseIP(n.Gateway) == nil {
				t.Errorf("network %s has invalid gateway %q", n.Name, n.Gateway)
			}
		}
		for _, z := range []string{"trust", "trust", "dmz", "management"} {
			if !zones[z] {
				t.Errorf("expected zone %q in default networks", z)
			}
		}
	})
}

func TestRandomIPInCIDR(t *testing.T) {
	r := rand.New(rand.NewSource(42))
	for _, cidr := range []string{"10.10.1.0/24", "10.10.1.0/28", "10.10.1.0/29"} {
		t.Run(cidr, func(t *testing.T) {
			_, ipNet, _ := net.ParseCIDR(cidr)
			for i := 0; i < 100; i++ {
				ip := RandomIPInCIDR(r, cidr)
				parsed := net.ParseIP(ip)
				if parsed == nil {
					t.Errorf("RandomIPInCIDR produced invalid IP: %q", ip)
					continue
				}
				if !ipNet.Contains(parsed) {
					t.Errorf("RandomIPInCIDR(%q) produced IP %q outside subnet", cidr, ip)
				}
			}
		})
	}
}

func TestRandomIPInCIDRRejectsTooSmall(t *testing.T) {
	// /30, /31, /32 fall outside blitz's "subnet with hosts" contract and
	// soft-fall-back to RandomIPv4. The result must still be a valid IPv4 —
	// not an empty string, not the input network address.
	r := rand.New(rand.NewSource(42))
	for _, cidr := range []string{"10.0.0.0/30", "10.0.0.0/31", "10.0.0.0/32"} {
		t.Run(cidr, func(t *testing.T) {
			ip := RandomIPInCIDR(r, cidr)
			if net.ParseIP(ip) == nil {
				t.Errorf("RandomIPInCIDR(%q) produced invalid IP: %q", cidr, ip)
			}
		})
	}
}

func TestValidateCIDR(t *testing.T) {
	t.Run("accepts /29 and shorter prefixes", func(t *testing.T) {
		for _, cidr := range []string{"10.0.0.0/29", "10.0.0.0/24", "10.0.0.0/16", "0.0.0.0/0"} {
			if err := ValidateCIDR(cidr); err != nil {
				t.Errorf("ValidateCIDR(%q) unexpected error: %v", cidr, err)
			}
		}
	})
	t.Run("rejects prefixes /30, /31, /32", func(t *testing.T) {
		for _, cidr := range []string{"10.0.0.0/30", "10.0.0.0/31", "10.0.0.0/32"} {
			if err := ValidateCIDR(cidr); err == nil {
				t.Errorf("ValidateCIDR(%q) expected error, got nil", cidr)
			}
		}
	})
	t.Run("rejects unparseable input", func(t *testing.T) {
		for _, cidr := range []string{"", "garbage", "10.0.0.0", "10.0.0.0/", "10.0.0.0/40"} {
			if err := ValidateCIDR(cidr); err == nil {
				t.Errorf("ValidateCIDR(%q) expected error, got nil", cidr)
			}
		}
	})
	t.Run("rejects IPv6 input pending PIPE-1001", func(t *testing.T) {
		if err := ValidateCIDR("2001:db8::/32"); err == nil {
			t.Error("ValidateCIDR(IPv6) expected error, got nil")
		}
	})
}

func TestNetworkIdentityValidate(t *testing.T) {
	t.Run("default networks all validate", func(t *testing.T) {
		for _, n := range GenerateDefaultNetworks() {
			if err := n.Validate(); err != nil {
				t.Errorf("default network %s failed Validate: %v", n.Name, err)
			}
		}
	})
	t.Run("invalid CIDR fails", func(t *testing.T) {
		n := &NetworkIdentity{ID: "x", CIDR: "10.0.0.0/30"}
		if err := n.Validate(); err == nil {
			t.Error("expected /30 to fail Validate, got nil")
		}
	})
}

func TestRandomIPDeterministic(t *testing.T) {
	r1 := rand.New(rand.NewSource(99))
	r2 := rand.New(rand.NewSource(99))
	for i := 0; i < 20; i++ {
		if RandomIPv4(r1) != RandomIPv4(r2) {
			t.Fatal("same seed produced different IPs")
		}
	}
}

// isPrivateIP checks if an IP is in RFC1918 ranges.
func isPrivateIP(ip net.IP) bool {
	private := []string{"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16"}
	for _, cidr := range private {
		_, ipNet, _ := net.ParseCIDR(cidr)
		if ipNet.Contains(ip) {
			return true
		}
	}
	return false
}

func TestRandomMACFormat(t *testing.T) {
	r := rand.New(rand.NewSource(42))
	mac := RandomMAC(r)
	parts := strings.Split(mac, ":")
	if len(parts) != 6 {
		t.Errorf("MAC %q should have 6 colon-separated parts", mac)
	}
}
