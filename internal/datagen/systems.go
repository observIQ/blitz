package datagen

import (
	"fmt"
	"math/rand"
	"strings"
	"time"
)

// OSType represents an operating system type.
type OSType string

const (
	OSLinux   OSType = "linux"
	OSWindows OSType = "windows"
	OSMacOS   OSType = "macos"
)

// Arch represents a CPU architecture.
type Arch string

// Arch values are the OpenTelemetry semconv host.arch value set. Random system
// generation uses the common ones (amd64, arm64); the rest are selectable via
// explicit configuration (e.g. an s390x mainframe or ppc64 host).
const (
	ArchAMD64 Arch = "amd64"
	ArchARM32 Arch = "arm32"
	ArchARM64 Arch = "arm64"
	ArchIA64  Arch = "ia64"
	ArchPPC32 Arch = "ppc32"
	ArchPPC64 Arch = "ppc64"
	ArchS390X Arch = "s390x"
	ArchX86   Arch = "x86"
)

// ParseArch maps a user-supplied CPU architecture string to an Arch, accepting
// the OpenTelemetry semconv host.arch value set. Unknown values return an error.
func ParseArch(s string) (Arch, error) {
	switch a := Arch(strings.ToLower(strings.TrimSpace(s))); a {
	case ArchAMD64, ArchARM32, ArchARM64, ArchIA64, ArchPPC32, ArchPPC64, ArchS390X, ArchX86:
		return a, nil
	default:
		return "", fmt.Errorf("datagen: unsupported architecture %q", s)
	}
}

// SystemRole represents a machine's role in the environment.
type SystemRole string

const (
	RoleServer      SystemRole = "server"
	RoleWorkstation SystemRole = "workstation"
	RoleDC          SystemRole = "dc"
	RoleRouter      SystemRole = "router"
)

// SystemIdentity represents a machine in the simulated environment.
type SystemIdentity struct {
	Hostname string
	FQDN     string // hostname + domain
	OSInfo   OSInfo // os.type / os.name / os.version / os.build_id / os.description
	HostID   string // host.id (OS-appropriate machine identifier)
	Arch     Arch
	Role     SystemRole
	Tier     DeploymentTier // deployment.environment.name
	Domain   string         // back-reference to DomainIdentity.Name
	OUPath   string         // "OU=Servers,DC=contoso,DC=com"

	// Hardware
	CPUCores int
	MemoryMB int
	DiskGB   int

	// Network interfaces (populated by environment generation)
	Interfaces []NetworkInterface

	// TLS cert issued by the domain CA
	Cert *CertInfo

	// Sub-identities (populated by environment generation)
	Services     []*ServiceIdentity
	Applications []*ApplicationIdentity
}

// NetworkInterface represents a NIC bound to a network subnet.
type NetworkInterface struct {
	Name       string // "eth0", "Ethernet0"
	IPv4       string
	IPv6       string
	MACAddress string
	SubnetID   string // references NetworkIdentity.ID
	VLAN       int
}

// CertInfo represents a TLS certificate issued by the domain CA.
type CertInfo struct {
	SubjectCN    string // = FQDN
	Issuer       string // = DomainIdentity.CA.CommonName
	SerialNumber string // hex serial
	Thumbprint   string // SHA1 hex (40 chars)
	ValidFrom    time.Time
	ValidTo      time.Time
	SANs         []string // [FQDN, hostname, IPv4]
}

// GenerateSystemIdentity creates a system with the given OS, role, and domain context.
//
// domain must be non-nil and have a populated CA — those fields are used
// unconditionally to construct the system's FQDN, OU path, and TLS cert.
// Passing a nil domain or a domain with a nil CA returns an error rather than
// panicking, so an embedding host handles bad input without crashing.
func GenerateSystemIdentity(r *rand.Rand, os OSType, role SystemRole, domain *DomainIdentity, names *Pool[string]) (*SystemIdentity, error) {
	if domain == nil {
		return nil, fmt.Errorf("datagen: GenerateSystemIdentity: domain must not be nil")
	}
	if domain.CA == nil {
		return nil, fmt.Errorf("datagen: GenerateSystemIdentity: domain.CA must not be nil")
	}

	// Pick hostname style based on OS
	var style HostnameStyle
	switch {
	case role == RoleDC:
		style = StyleDC
	case os == OSWindows:
		style = StyleWindows
	default:
		style = StyleLinux
	}

	hostname := GenerateHostname(r, style, names)
	fqdn := hostname + "." + domain.Name
	if os == OSWindows || role == RoleDC {
		fqdn = strings.ToLower(hostname) + "." + domain.Name
	}

	// Coherent OS release + machine id, from authentic release data.
	osInfo := GenerateOSInfo(r, os)
	hostID := GenerateHostID(r, os)

	// Pick arch
	arch := ArchAMD64
	if r.Float64() < 0.1 { // #nosec G404
		arch = ArchARM64
	}

	// Generate resource specs based on role
	cpu, mem, disk := generateResourceSpecs(r, role)

	// Generate OU path
	ouPath := generateOUPath(role, domain.Name)

	// Generate TLS cert
	cert := generateCertInfo(r, fqdn, hostname, domain.CA)

	return &SystemIdentity{
		Hostname: hostname,
		FQDN:     fqdn,
		OSInfo:   osInfo,
		HostID:   hostID,
		Arch:     arch,
		Role:     role,
		Domain:   domain.Name,
		OUPath:   ouPath,
		CPUCores: cpu,
		MemoryMB: mem,
		DiskGB:   disk,
		Cert:     cert,
	}, nil
}

// generateResourceSpecs returns CPU, memory, disk based on role.
func generateResourceSpecs(r *rand.Rand, role SystemRole) (cpu, mem, disk int) {
	switch role {
	case RoleWorkstation:
		cpu = randRange(r, 4, 16)
		mem = randRange(r, 8192, 32768)
		disk = randRange(r, 256, 1024)
	case RoleServer:
		cpu = randRange(r, 4, 64)
		mem = randRange(r, 16384, 131072)
		disk = randRange(r, 500, 4096)
	case RoleDC:
		cpu = randRange(r, 4, 16)
		mem = randRange(r, 16384, 65536)
		disk = randRange(r, 500, 2048)
	case RoleRouter:
		cpu = randRange(r, 2, 4)
		mem = randRange(r, 2048, 8192)
		disk = randRange(r, 64, 256)
	default:
		cpu = randRange(r, 4, 16)
		mem = randRange(r, 8192, 32768)
		disk = randRange(r, 256, 1024)
	}
	return
}

// randRange returns a random int in [min, max] inclusive.
func randRange(r *rand.Rand, min, max int) int {
	return min + r.Intn(max-min+1) // #nosec G404
}

// generateOUPath creates an AD OU path based on role.
func generateOUPath(role SystemRole, domainName string) string {
	parts := strings.Split(domainName, ".")
	dcParts := make([]string, len(parts))
	for i, p := range parts {
		dcParts[i] = "DC=" + p
	}
	dcSuffix := strings.Join(dcParts, ",")

	var ou string
	switch role {
	case RoleDC:
		ou = "OU=Domain Controllers"
	case RoleServer:
		ou = "OU=Servers"
	case RoleWorkstation:
		ou = "OU=Workstations"
	case RoleRouter:
		ou = "OU=Network Devices"
	default:
		ou = "OU=Computers"
	}
	return fmt.Sprintf("%s,%s", ou, dcSuffix)
}

// generateCertInfo creates a TLS cert for a system.
func generateCertInfo(r *rand.Rand, fqdn, hostname string, ca *CertAuthority) *CertInfo {
	now := time.Now()
	return &CertInfo{
		SubjectCN:    fqdn,
		Issuer:       ca.CommonName,
		SerialNumber: randomHex(r, 8),
		Thumbprint:   randomHex(r, 20),
		ValidFrom:    now.AddDate(-1, 0, 0),
		ValidTo:      now.AddDate(1, 0, 0),
		SANs:         []string{fqdn, hostname},
	}
}
