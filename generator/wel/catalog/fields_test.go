package catalog

import (
	"math/rand"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

func TestRandomSID(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	sid := RandomSID(rng, "S-1-5-21-1234567890-1234567890-1234567890")
	if !strings.HasPrefix(sid, "S-1-5-21-1234567890-1234567890-1234567890-") {
		t.Errorf("SID should have domain prefix, got %q", sid)
	}
	// Check RID is a number
	parts := strings.Split(sid, "-")
	if len(parts) != 8 {
		t.Fatalf("expected 8 SID components, got %d: %q", len(parts), sid)
	}
	rid, err := strconv.Atoi(parts[7])
	if err != nil {
		t.Fatalf("RID is not a number: %v", err)
	}
	if rid < 1000 || rid > 51000 {
		t.Errorf("RID %d out of expected range [1000, 51000]", rid)
	}
}

func TestRandomLogonID(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	id := RandomLogonID(rng)
	if !strings.HasPrefix(id, "0x") {
		t.Errorf("LogonID should start with 0x, got %q", id)
	}
	if len(id) < 4 {
		t.Errorf("LogonID too short: %q", id)
	}
}

func TestRandomProcessID(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	pid := RandomProcessID(rng)
	if !strings.HasPrefix(pid, "0x") {
		t.Errorf("ProcessID should start with 0x, got %q", pid)
	}
}

func TestRandomPort(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	for i := 0; i < 100; i++ {
		port := RandomPort(rng)
		p, err := strconv.Atoi(port)
		if err != nil {
			t.Fatalf("port is not a number: %v", err)
		}
		if p < 1024 || p > 65535 {
			t.Errorf("port %d out of range [1024, 65535]", p)
		}
	}
}

func TestRandomIPv4(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	ip := RandomIPv4(rng)
	match, _ := regexp.MatchString(`^\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}$`, ip)
	if !match {
		t.Errorf("invalid IPv4 format: %q", ip)
	}
}

func TestRandomGUID(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	guid := RandomGUID(rng)
	match, _ := regexp.MatchString(`^\{[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\}$`, guid)
	if !match {
		t.Errorf("invalid GUID format: %q", guid)
	}
}

func TestRandomHexID(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	hex := RandomHexID(rng, 8)
	if !strings.HasPrefix(hex, "0x") {
		t.Errorf("HexID should start with 0x, got %q", hex)
	}
	if len(hex) != 18 { // "0x" + 16 hex chars
		t.Errorf("HexID should be 18 chars for 8 bytes, got %d: %q", len(hex), hex)
	}
}

func TestPickUsername(t *testing.T) {
	rng := rand.New(rand.NewSource(42))

	t.Run("from list", func(t *testing.T) {
		names := []string{"alice", "bob", "charlie"}
		name := PickUsername(rng, names)
		found := false
		for _, n := range names {
			if n == name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("PickUsername returned %q which is not in the list", name)
		}
	})

	t.Run("empty list fallback", func(t *testing.T) {
		name := PickUsername(rng, nil)
		if name == "" {
			t.Error("PickUsername returned empty string for nil list")
		}
	})
}

func TestPickHostname(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	names := []string{"host1", "host2"}
	name := PickHostname(rng, names)
	if name != "host1" && name != "host2" {
		t.Errorf("unexpected hostname: %q", name)
	}
}

func TestPickIP(t *testing.T) {
	rng := rand.New(rand.NewSource(42))

	t.Run("from list", func(t *testing.T) {
		ips := []string{"10.0.0.1", "10.0.0.2"}
		ip := PickIP(rng, ips)
		if ip != "10.0.0.1" && ip != "10.0.0.2" {
			t.Errorf("unexpected IP: %q", ip)
		}
	})

	t.Run("empty list fallback", func(t *testing.T) {
		ip := PickIP(rng, nil)
		match, _ := regexp.MatchString(`^\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}$`, ip)
		if !match {
			t.Errorf("fallback IP invalid format: %q", ip)
		}
	})
}

func TestRandomAccessMask(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	mask := RandomAccessMask(rng)
	if !strings.HasPrefix(mask, "0x") {
		t.Errorf("AccessMask should start with 0x, got %q", mask)
	}
}

func TestRandomLogonType(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	validTypes := map[string]bool{
		"2": true, "3": true, "4": true, "5": true,
		"7": true, "8": true, "9": true, "10": true, "11": true,
	}
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		lt := RandomLogonType(rng)
		if !validTypes[lt] {
			t.Fatalf("invalid logon type: %q", lt)
		}
		seen[lt] = true
	}
	// Should have seen at least 3 different types in 100 draws
	if len(seen) < 3 {
		t.Errorf("expected variety in logon types, only saw %d unique values", len(seen))
	}
}

func TestRandomPrivilegeList(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	priv := RandomPrivilegeList(rng)
	if priv == "" {
		t.Error("privilege list should not be empty")
	}
	// Should contain at least one known privilege
	knownPrivs := []string{"Se"}
	found := false
	for _, kp := range knownPrivs {
		if strings.Contains(priv, kp) {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("privilege list should contain known privileges: %q", priv)
	}
}

func TestKeywordsToHex(t *testing.T) {
	tests := []struct {
		kw       uint64
		expected string
	}{
		{0x8020000000000000, "0x8020000000000000"},
		{0x8010000000000000, "0x8010000000000000"},
		{0, "0x0"},
	}
	for _, tt := range tests {
		got := KeywordsToHex(tt.kw)
		if got != tt.expected {
			t.Errorf("KeywordsToHex(%d) = %q, want %q", tt.kw, got, tt.expected)
		}
	}
}
