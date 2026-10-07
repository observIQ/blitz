package datagen

import (
	"math/rand"
	"regexp"
	"testing"
)

// expandedOSes is the PIPE-1260 additions: ESXi, the hypervisor-host Linux
// flavors, and the Unix families. semconv is the os.type wire value each emits.
var expandedOSes = []struct {
	os      OSType
	semconv string
	name    string // substring expected in os.name
}{
	{OSESXi, "esxi", "ESXi"},
	{OSXenDom0, "linux", "XCP-ng"},
	{OSNutanixAHV, "linux", "Nutanix"},
	{OSOpenStackKVM, "linux", ""}, // a real Linux distro; os.name is the distro
	{OSAIX, "aix", "AIX"},
	{OSSolaris, "solaris", "Solaris"},
	{OSFreeBSD, "freebsd", "FreeBSD"},
	{OSOpenBSD, "openbsd", "OpenBSD"},
}

func TestParseOSType_Expanded(t *testing.T) {
	for _, e := range expandedOSes {
		got, err := ParseOSType(string(e.os))
		if err != nil {
			t.Errorf("ParseOSType(%q): unexpected error: %v", e.os, err)
			continue
		}
		if got != e.os {
			t.Errorf("ParseOSType(%q) = %q, want %q", e.os, got, e.os)
		}
	}
}

func TestSemconvOSType_Expanded(t *testing.T) {
	for _, e := range expandedOSes {
		if got := e.os.SemconvOSType(); got != e.semconv {
			t.Errorf("%q.SemconvOSType() = %q, want %q", e.os, got, e.semconv)
		}
	}
}

func TestGenerateOSInfo_Expanded(t *testing.T) {
	for _, e := range expandedOSes {
		info := GenerateOSInfo(rand.New(rand.NewSource(1)), e.os) // #nosec G404
		if info.Type != e.os {
			t.Errorf("%s: Type = %q, want %q", e.os, info.Type, e.os)
		}
		if info.Name == "" || info.Version == "" || info.BuildID == "" || info.Description == "" {
			t.Errorf("%s: incomplete OSInfo: %+v", e.os, info)
		}
		if e.name != "" && !regexp.MustCompile(regexp.QuoteMeta(e.name)).MatchString(info.Name) {
			t.Errorf("%s: os.name = %q, want to contain %q", e.os, info.Name, e.name)
		}
		// Deterministic for a given (seed, os).
		again := GenerateOSInfo(rand.New(rand.NewSource(1)), e.os) // #nosec G404
		if info != again {
			t.Errorf("%s: GenerateOSInfo not deterministic: %+v vs %+v", e.os, info, again)
		}
	}
}

func TestGenerateHostID_Expanded(t *testing.T) {
	uuidLower := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	linux32 := regexp.MustCompile(`^[0-9a-f]{32}$`)
	aixRE := regexp.MustCompile(`^00[0-9A-F]{6}4C00$`)
	solarisRE := regexp.MustCompile(`^[0-9a-f]{8}$`)

	cases := map[OSType]*regexp.Regexp{
		OSESXi:         uuidLower,
		OSFreeBSD:      uuidLower,
		OSOpenBSD:      uuidLower,
		OSXenDom0:      linux32,
		OSNutanixAHV:   linux32,
		OSOpenStackKVM: linux32,
		OSAIX:          aixRE,
		OSSolaris:      solarisRE,
	}
	for os, re := range cases {
		id := GenerateHostID(rand.New(rand.NewSource(1)), os) // #nosec G404
		if !re.MatchString(id) {
			t.Errorf("%s host.id = %q, want match %s", os, id, re)
		}
	}
}

func TestGenerateServicesForSystem_Expanded(t *testing.T) {
	contains := func(svcs []*ServiceIdentity, name string) bool {
		for _, s := range svcs {
			if s.Name == name {
				return true
			}
		}
		return false
	}

	// Every expanded OS in a server role gets its bespoke daemon set, which is
	// larger than the one-service default fallback.
	for _, e := range expandedOSes {
		svcs := GenerateServicesForSystem(rand.New(rand.NewSource(1)), e.os, RoleServer, "host1") // #nosec G404
		if len(svcs) < 3 {
			t.Errorf("%s server services = %d, want a bespoke set (>=3), not the default fallback", e.os, len(svcs))
		}
	}

	// The virtualization hosts must carry their signature virtualization daemon.
	// These pools are small enough that the full set is always returned.
	xen := GenerateServicesForSystem(rand.New(rand.NewSource(1)), OSXenDom0, RoleServer, "h") // #nosec G404
	if !contains(xen, "xapi") {
		t.Errorf("Xen dom0 services missing xapi: %+v", xen)
	}
	for _, os := range []OSType{OSNutanixAHV, OSOpenStackKVM} {
		svcs := GenerateServicesForSystem(rand.New(rand.NewSource(1)), os, RoleServer, "h") // #nosec G404
		if !contains(svcs, "libvirtd") {
			t.Errorf("%s services missing libvirtd: %+v", os, svcs)
		}
	}
}
