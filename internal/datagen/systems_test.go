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

func TestVersionPools(t *testing.T) {
	if LinuxVersions.Len() < 3 {
		t.Errorf("LinuxVersions has %d items, want at least 3", LinuxVersions.Len())
	}
	if WindowsVersions.Len() < 3 {
		t.Errorf("WindowsVersions has %d items, want at least 3", WindowsVersions.Len())
	}
	if MacOSVersions.Len() < 3 {
		t.Errorf("MacOSVersions has %d items, want at least 3", MacOSVersions.Len())
	}
}

func TestGenerateSystemIdentity(t *testing.T) {
	r := rand.New(rand.NewSource(42))
	domain := GenerateDomainIdentity(42, "contoso.com", time.Now())

	t.Run("linux server", func(t *testing.T) {
		sys := GenerateSystemIdentity(r, OSLinux, RoleServer, domain, NorseNames)
		if sys.OS != OSLinux {
			t.Errorf("expected OS %q, got %q", OSLinux, sys.OS)
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
		sys := GenerateSystemIdentity(r, OSWindows, RoleWorkstation, domain, RomanNames)
		if sys.OS != OSWindows {
			t.Errorf("expected OS %q, got %q", OSWindows, sys.OS)
		}
		if sys.Hostname != strings.ToUpper(sys.Hostname) {
			t.Errorf("windows hostname %q should be uppercase", sys.Hostname)
		}
	})

	t.Run("domain controller", func(t *testing.T) {
		sys := GenerateSystemIdentity(r, OSWindows, RoleDC, domain, GreekNames)
		if sys.Role != RoleDC {
			t.Errorf("expected Role %q, got %q", RoleDC, sys.Role)
		}
		if !strings.Contains(sys.Hostname, "-DC") {
			t.Errorf("DC hostname %q should contain '-DC'", sys.Hostname)
		}
	})

	t.Run("has cert", func(t *testing.T) {
		sys := GenerateSystemIdentity(r, OSLinux, RoleServer, domain, NorseNames)
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
		sys := GenerateSystemIdentity(r, OSWindows, RoleServer, domain, RomanNames)
		if !strings.HasPrefix(sys.OUPath, "OU=") {
			t.Errorf("OUPath %q should start with 'OU='", sys.OUPath)
		}
		if !strings.Contains(sys.OUPath, "DC=contoso") {
			t.Errorf("OUPath %q should contain 'DC=contoso'", sys.OUPath)
		}
	})
}

func TestGenerateSystemIdentityPanicsOnNilInputs(t *testing.T) {
	r := rand.New(rand.NewSource(1))
	domain := GenerateDomainIdentity(1, "", time.Now())

	t.Run("nil domain panics", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Error("expected panic on nil domain, got none")
			}
		}()
		GenerateSystemIdentity(r, OSLinux, RoleServer, nil, NorseNames)
	})

	t.Run("domain with nil CA panics", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Error("expected panic on nil domain.CA, got none")
			}
		}()
		brokenDomain := &DomainIdentity{Name: domain.Name, DomainSID: domain.DomainSID, CA: nil}
		GenerateSystemIdentity(r, OSLinux, RoleServer, brokenDomain, NorseNames)
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
			sys := GenerateSystemIdentity(r, OSLinux, role, domain, NorseNames)
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
