package datagen

import (
	"math/rand"
	"strings"
	"testing"
	"time"
)

func TestOSTypes(t *testing.T) {
	types := []OSType{OSLinux, OSWindows, OSMacOS}
	for _, os := range types {
		if os == "" {
			t.Error("OSType should not be empty")
		}
	}
}

func TestArchTypes(t *testing.T) {
	types := []Arch{ArchAMD64, ArchARM64, ArchX86}
	for _, a := range types {
		if a == "" {
			t.Error("Arch should not be empty")
		}
	}
}

func TestSystemRoles(t *testing.T) {
	roles := []SystemRole{RoleServer, RoleWorkstation, RoleDC, RoleRouter}
	for _, r := range roles {
		if r == "" {
			t.Error("SystemRole should not be empty")
		}
	}
}

func TestReleasePools(t *testing.T) {
	if len(linuxReleases) < 3 {
		t.Errorf("linuxReleases has %d items, want at least 3", len(linuxReleases))
	}
	if len(windowsReleases) < 3 {
		t.Errorf("windowsReleases has %d items, want at least 3", len(windowsReleases))
	}
	if len(macReleases) < 3 {
		t.Errorf("macReleases has %d items, want at least 3", len(macReleases))
	}
}

func TestGenerateSystemIdentity(t *testing.T) {
	r := rand.New(rand.NewSource(42))
	domain := GenerateDomainIdentity(42, "contoso.com", time.Now())

	t.Run("linux server", func(t *testing.T) {
		sys, err := GenerateSystemIdentity(r, OSLinux, RoleServer, domain, NorseNames)
		if err != nil {
			t.Fatalf("GenerateSystemIdentity: %v", err)
		}
		if sys.OSInfo.Type != OSLinux {
			t.Errorf("expected OS %q, got %q", OSLinux, sys.OSInfo.Type)
		}
		if sys.Role != RoleServer {
			t.Errorf("expected Role %q, got %q", RoleServer, sys.Role)
		}
		if sys.Domain != "contoso.com" {
			t.Errorf("expected Domain 'contoso.com', got %q", sys.Domain)
		}
		if !strings.HasSuffix(sys.FQDN, ".contoso.com") {
			t.Errorf("FQDN %q should end with '.contoso.com'", sys.FQDN)
		}
		if sys.CPUCores < 4 || sys.CPUCores > 64 {
			t.Errorf("server CPUCores %d out of range [4,64]", sys.CPUCores)
		}
		if sys.MemoryMB < 16384 {
			t.Errorf("server MemoryMB %d should be >= 16384", sys.MemoryMB)
		}
	})

	t.Run("windows workstation", func(t *testing.T) {
		sys, err := GenerateSystemIdentity(r, OSWindows, RoleWorkstation, domain, RomanNames)
		if err != nil {
			t.Fatalf("GenerateSystemIdentity: %v", err)
		}
		if sys.OSInfo.Type != OSWindows {
			t.Errorf("expected OS %q, got %q", OSWindows, sys.OSInfo.Type)
		}
		if sys.Hostname != strings.ToUpper(sys.Hostname) {
			t.Errorf("windows hostname %q should be uppercase", sys.Hostname)
		}
	})

	t.Run("domain controller", func(t *testing.T) {
		sys, err := GenerateSystemIdentity(r, OSWindows, RoleDC, domain, GreekNames)
		if err != nil {
			t.Fatalf("GenerateSystemIdentity: %v", err)
		}
		if sys.Role != RoleDC {
			t.Errorf("expected Role %q, got %q", RoleDC, sys.Role)
		}
		if !strings.Contains(sys.Hostname, "-DC") {
			t.Errorf("DC hostname %q should contain '-DC'", sys.Hostname)
		}
	})

	t.Run("has cert", func(t *testing.T) {
		sys, err := GenerateSystemIdentity(r, OSLinux, RoleServer, domain, NorseNames)
		if err != nil {
			t.Fatalf("GenerateSystemIdentity: %v", err)
		}
		if sys.Cert == nil {
			t.Fatal("expected system to have a cert")
		}
		if sys.Cert.SubjectCN != sys.FQDN {
			t.Errorf("cert SubjectCN %q should equal FQDN %q", sys.Cert.SubjectCN, sys.FQDN)
		}
		if sys.Cert.Issuer != domain.CA.CommonName {
			t.Errorf("cert Issuer %q should equal CA CommonName %q", sys.Cert.Issuer, domain.CA.CommonName)
		}
	})

	t.Run("has OU path", func(t *testing.T) {
		sys, err := GenerateSystemIdentity(r, OSWindows, RoleServer, domain, RomanNames)
		if err != nil {
			t.Fatalf("GenerateSystemIdentity: %v", err)
		}
		if !strings.HasPrefix(sys.OUPath, "OU=") {
			t.Errorf("OUPath %q should start with 'OU='", sys.OUPath)
		}
		if !strings.Contains(sys.OUPath, "DC=contoso") {
			t.Errorf("OUPath %q should contain 'DC=contoso'", sys.OUPath)
		}
	})
}

func TestHostImageFrameworkHook(t *testing.T) {
	// The Image hook exists so a future CloudIdentity source can populate
	// host.image.* on a system, but it is unwired today: GenerateSystemIdentity
	// must leave it nil so the projection emits no host.image.* attributes.
	r := rand.New(rand.NewSource(7))
	domain := GenerateDomainIdentity(7, "contoso.com", time.Now())

	sys, err := GenerateSystemIdentity(r, OSLinux, RoleServer, domain, NorseNames)
	if err != nil {
		t.Fatalf("GenerateSystemIdentity: %v", err)
	}
	if sys.Image != nil {
		t.Errorf("expected Image nil (unwired framework hook), got %+v", sys.Image)
	}

	// HostImage carries the OTel host.image.* semconv fields.
	img := &HostImage{ID: "ami-0abc", Name: "ubuntu-22.04", Version: "20240115"}
	if img.ID != "ami-0abc" || img.Name != "ubuntu-22.04" || img.Version != "20240115" {
		t.Errorf("HostImage fields did not round-trip: %+v", img)
	}
}

func TestGenerateSystemIdentityErrorsOnNilInputs(t *testing.T) {
	r := rand.New(rand.NewSource(1))
	domain := GenerateDomainIdentity(1, "", time.Now())

	t.Run("nil domain errors", func(t *testing.T) {
		sys, err := GenerateSystemIdentity(r, OSLinux, RoleServer, nil, NorseNames)
		if err == nil {
			t.Error("expected error on nil domain, got none")
		}
		if sys != nil {
			t.Errorf("expected nil identity on error, got %v", sys)
		}
	})

	t.Run("domain with nil CA errors", func(t *testing.T) {
		brokenDomain := &DomainIdentity{Name: domain.Name, DomainSID: domain.DomainSID, CA: nil}
		sys, err := GenerateSystemIdentity(r, OSLinux, RoleServer, brokenDomain, NorseNames)
		if err == nil {
			t.Error("expected error on nil domain.CA, got none")
		}
		if sys != nil {
			t.Errorf("expected nil identity on error, got %v", sys)
		}
	})
}

func TestSystemResourceRanges(t *testing.T) {
	r := rand.New(rand.NewSource(1))
	domain := GenerateDomainIdentity(1, "", time.Now())

	specs := map[SystemRole]struct {
		minCPU, maxCPU   int
		minMem, maxMem   int
		minDisk, maxDisk int
	}{
		RoleWorkstation: {4, 16, 8192, 32768, 256, 1024},
		RoleServer:      {4, 64, 16384, 131072, 500, 4096},
		RoleDC:          {4, 16, 16384, 65536, 500, 2048},
		RoleRouter:      {2, 4, 2048, 8192, 64, 256},
	}

	for role, spec := range specs {
		for i := 0; i < 50; i++ {
			sys, err := GenerateSystemIdentity(r, OSLinux, role, domain, NorseNames)
			if err != nil {
				t.Fatalf("GenerateSystemIdentity: %v", err)
			}
			if sys.CPUCores < spec.minCPU || sys.CPUCores > spec.maxCPU {
				t.Errorf("role %s: CPUCores %d out of [%d,%d]", role, sys.CPUCores, spec.minCPU, spec.maxCPU)
			}
			if sys.MemoryMB < spec.minMem || sys.MemoryMB > spec.maxMem {
				t.Errorf("role %s: MemoryMB %d out of [%d,%d]", role, sys.MemoryMB, spec.minMem, spec.maxMem)
			}
			if sys.DiskGB < spec.minDisk || sys.DiskGB > spec.maxDisk {
				t.Errorf("role %s: DiskGB %d out of [%d,%d]", role, sys.DiskGB, spec.minDisk, spec.maxDisk)
			}
		}
	}
}
