package resource

import (
	"os"
	"reflect"
	"testing"

	"github.com/observiq/blitz/internal/datagen"
)

// fullIdentity is a completely-populated host identity for projection tests.
func fullIdentity() *datagen.SystemIdentity {
	return &datagen.SystemIdentity{
		Hostname: "THOR-WEB-01",
		HostID:   "6b3a2f1e-1122-3344-5566-778899aabbcc",
		Arch:     datagen.ArchAMD64,
		Tier:     datagen.TierProd,
		OSInfo: datagen.OSInfo{
			Type:        datagen.OSWindows,
			Name:        "Microsoft Windows Server 2022",
			Version:     "10.0.20348.2762",
			BuildID:     "20348",
			Description: "Microsoft Windows [Version 10.0.20348.2762]",
		},
		Interfaces: []datagen.NetworkInterface{
			{Name: "Ethernet0", IPv4: "10.10.1.20", IPv6: "fe80::1", MACAddress: "00:1a:2b:3c:4d:5e"},
		},
	}
}

func TestFromIdentityFullProjection(t *testing.T) {
	rec := FromIdentity(fullIdentity(), "wel").Record()

	want := map[string]string{
		"host.name":                   "THOR-WEB-01",
		"host.id":                     "6b3a2f1e-1122-3344-5566-778899aabbcc",
		"host.arch":                   "amd64",
		"os.type":                     "windows",
		"os.name":                     "Microsoft Windows Server 2022",
		"os.version":                  "10.0.20348.2762",
		"os.build_id":                 "20348",
		"os.description":              "Microsoft Windows [Version 10.0.20348.2762]",
		"deployment.environment.name": "production",
		"telemetry.source":            "wel",
	}
	for k, v := range want {
		if rec[k] != v {
			t.Errorf("%s = %v, want %q", k, rec[k], v)
		}
	}
}

func TestFromIdentityMacOSTypeIsDarwin(t *testing.T) {
	sys := &datagen.SystemIdentity{
		Hostname: "brigid-mbp",
		OSInfo:   datagen.OSInfo{Type: datagen.OSMacOS, Name: "macOS", Version: "14.6.1"},
	}
	rec := FromIdentity(sys, "json").Record()
	if rec["os.type"] != "darwin" {
		t.Errorf("os.type = %v, want darwin (semconv value for macOS)", rec["os.type"])
	}
}

func TestFromIdentityOmitsEmptyFields(t *testing.T) {
	// A bare identity: only a hostname, everything else zero-valued.
	sys := &datagen.SystemIdentity{Hostname: "sparse-01"}
	rec := FromIdentity(sys, "apache").Record()

	if rec["host.name"] != "sparse-01" {
		t.Errorf("host.name = %v, want sparse-01", rec["host.name"])
	}
	// No empty os.type / os.name / host.id / host.arch / deployment.* should be present.
	for _, k := range []string{"host.id", "host.arch", "os.type", "os.name", "os.version", "os.build_id", "os.description", "deployment.environment.name", "host.ip", "host.mac"} {
		if _, ok := rec[k]; ok {
			t.Errorf("expected %q to be omitted for a sparse identity, got %v", k, rec[k])
		}
	}
}

func TestFromIdentityHostIPAndMACArrays(t *testing.T) {
	sys := &datagen.SystemIdentity{
		Hostname: "multi-nic",
		Interfaces: []datagen.NetworkInterface{
			{IPv4: "10.10.1.20", IPv6: "fe80::1", MACAddress: "00:1a:2b:3c:4d:5e"},
			{IPv4: "10.10.2.30", IPv6: "", MACAddress: "00:1a:2b:3c:4d:5f"},
		},
	}
	rec := FromIdentity(sys, "paloalto").Record()

	ips, ok := rec["host.ip"].([]string)
	if !ok {
		t.Fatalf("host.ip = %T, want []string (OTLP ArrayValue via PIPE-1253)", rec["host.ip"])
	}
	wantIPs := []string{"10.10.1.20", "fe80::1", "10.10.2.30"}
	if !reflect.DeepEqual(ips, wantIPs) {
		t.Errorf("host.ip = %v, want %v", ips, wantIPs)
	}

	macs, ok := rec["host.mac"].([]string)
	if !ok {
		t.Fatalf("host.mac = %T, want []string", rec["host.mac"])
	}
	wantMACs := []string{"00:1a:2b:3c:4d:5e", "00:1a:2b:3c:4d:5f"}
	if !reflect.DeepEqual(macs, wantMACs) {
		t.Errorf("host.mac = %v, want %v", macs, wantMACs)
	}
}

func TestFromIdentityNilFallsBackToHostname(t *testing.T) {
	rec := FromIdentity(nil, "nginx").Record()

	h, _ := os.Hostname()
	if h == "" {
		h = "blitz"
	}
	if rec["host.name"] != h {
		t.Errorf("host.name = %v, want process hostname %q on nil identity", rec["host.name"], h)
	}
	if rec["telemetry.source"] != "nginx" {
		t.Errorf("telemetry.source = %v, want nginx", rec["telemetry.source"])
	}
	// No simulated os.* attributes when there is no identity.
	if _, ok := rec["os.type"]; ok {
		t.Errorf("os.type should be absent on nil identity, got %v", rec["os.type"])
	}
}

func TestFromIdentityDoesNotEmitHostImage(t *testing.T) {
	sys := fullIdentity()
	sys.Image = &datagen.HostImage{ID: "ami-0abc", Name: "win-2022", Version: "20240115"}
	rec := FromIdentity(sys, "wel").Record()

	for _, k := range []string{"host.image.id", "host.image.name", "host.image.version"} {
		if _, ok := rec[k]; ok {
			t.Errorf("%q must not be emitted (unwired framework hook), got %v", k, rec[k])
		}
	}
}

func TestFromIdentityExtrasApplied(t *testing.T) {
	rec := FromIdentity(fullIdentity(), "wel", "wel.role", "dc").Record()
	if rec["wel.role"] != "dc" {
		t.Errorf("extras not applied: wel.role = %v, want dc", rec["wel.role"])
	}
}
