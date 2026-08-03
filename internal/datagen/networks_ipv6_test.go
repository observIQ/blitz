package datagen

import (
	"math/rand"
	"net"
	"testing"
)

func TestReservedIPv6Blocks(t *testing.T) {
	if len(reservedIPv6Blocks) == 0 {
		t.Fatal("reservedIPv6Blocks is empty")
	}
	for _, b := range reservedIPv6Blocks {
		if len(b.cidrs) == 0 {
			t.Errorf("%s block has no CIDRs", b.rfc)
		}
	}
}

func TestIsReservedIPv6(t *testing.T) {
	reserved := []string{"fe80::1", "2001:db8::1", "fc00::1", "::1", "ff02::1"}
	for _, s := range reserved {
		if !isReservedIPv6(net.ParseIP(s)) {
			t.Errorf("%s should be reserved", s)
		}
	}
	if isReservedIPv6(net.ParseIP("2606:4700:4700::1111")) {
		t.Error("2606:4700:4700::1111 (public) should not be reserved")
	}
}

func TestValidateIPv6CIDR(t *testing.T) {
	tests := []struct {
		cidr    string
		wantErr bool
	}{
		{"2001:db8::/32", false},
		{"2001:db8:abcd::/48", false},
		{"fd00:1234::/64", false},
		{"2001:db8::/120", true}, // longer than /64
		{"10.0.0.0/24", true},    // IPv4
		{"not-a-cidr", true},
	}
	for _, tt := range tests {
		if err := ValidateIPv6CIDR(tt.cidr); (err != nil) != tt.wantErr {
			t.Errorf("ValidateIPv6CIDR(%q) err=%v, wantErr=%v", tt.cidr, err, tt.wantErr)
		}
	}
}

func TestRandomPublicIPv6(t *testing.T) {
	r := rand.New(rand.NewSource(42))
	_, globalUnicast, _ := net.ParseCIDR("2000::/3")
	for i := 0; i < 50; i++ {
		s := RandomPublicIPv6(r)
		ip := net.ParseIP(s)
		if ip == nil || ip.To4() != nil {
			t.Fatalf("RandomPublicIPv6 returned non-IPv6 %q", s)
		}
		if !globalUnicast.Contains(ip) {
			t.Errorf("%s not in global-unicast 2000::/3", s)
		}
		if isReservedIPv6(ip) {
			t.Errorf("%s is reserved", s)
		}
	}
}

func TestRandomPublicIPv6_RejectsReserved(t *testing.T) {
	// Force the first candidate to be treated as reserved so the retry path runs.
	calls := 0
	reserved := func(net.IP) bool {
		calls++
		return calls == 1
	}
	got := randomPublicIPv6(rand.New(rand.NewSource(5)), reserved)
	if net.ParseIP(got) == nil {
		t.Fatalf("randomPublicIPv6 returned invalid IP %q", got)
	}
	if calls < 2 {
		t.Errorf("expected a reserved candidate to be rejected and retried, calls=%d", calls)
	}
}

func TestRandomIPInCIDRv6(t *testing.T) {
	r := rand.New(rand.NewSource(1))
	_, subnet, _ := net.ParseCIDR("2001:db8:abcd::/48")
	for i := 0; i < 50; i++ {
		ip := net.ParseIP(RandomIPInCIDRv6(r, "2001:db8:abcd::/48"))
		if ip == nil || !subnet.Contains(ip) {
			t.Errorf("%v not in 2001:db8:abcd::/48", ip)
		}
	}
	// Fallbacks: unparseable and IPv4 CIDRs return a public IPv6.
	if ip := net.ParseIP(RandomIPInCIDRv6(r, "garbage")); ip == nil || ip.To4() != nil {
		t.Error("bad CIDR should fall back to a public IPv6")
	}
	if ip := net.ParseIP(RandomIPInCIDRv6(r, "10.0.0.0/24")); ip == nil || ip.To4() != nil {
		t.Error("IPv4 CIDR should fall back to a public IPv6")
	}
}
