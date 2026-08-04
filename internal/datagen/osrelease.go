package datagen

import (
	"fmt"
	"math/rand"
	"strings"
)

// OSInfo is the OpenTelemetry os.* projection source for a system: the fields
// map 1:1 to os.type / os.name / os.version / os.build_id / os.description. The
// values within one OSInfo are internally consistent (a real name/version/
// build/description that actually go together), sourced from authentic release
// data (see the pools below), not synthesized piecemeal.
type OSInfo struct {
	Type        OSType // os.type
	Name        string // os.name        e.g. "Ubuntu", "Microsoft Windows Server 2022", "macOS"
	Version     string // os.version     e.g. "22.04.5", "10.0.20348.2762", "14.6.1"
	BuildID     string // os.build_id    e.g. "5.15.0-91-generic", "20348", "23G80"
	Description string // os.description e.g. "Ubuntu 22.04.5 LTS"
}

// linuxRelease is one authentic Linux distro release. kernel becomes
// os.build_id; pretty is the os-release PRETTY_NAME (os.description).
type linuxRelease struct{ name, version, kernel, pretty string }

// windowsRelease is one Windows product. build is the CurrentBuildNumber
// (os.build_id); ubrs are real Update Build Revisions from the product's update
// history — one is chosen per generation to form the full version + ver-string.
type windowsRelease struct {
	name, build string
	ubrs        []int
}

// macRelease is one authentic macOS point release (version + ProductBuildVersion).
type macRelease struct{ version, build string }

// Authentic release data. Linux: os-release NAME / VERSION_ID / kernel /
// PRETTY_NAME. Windows: product name / build number / real UBRs from MS update
// history. macOS: verified version→build pairs (Apple/Wikipedia).
var (
	linuxReleases = []linuxRelease{
		{"Ubuntu", "22.04.5", "5.15.0-91-generic", "Ubuntu 22.04.5 LTS"},
		{"Debian GNU/Linux", "12", "6.1.0-18-amd64", "Debian GNU/Linux 12 (bookworm)"},
		{"Debian GNU/Linux", "11", "5.10.0-27-amd64", "Debian GNU/Linux 11 (bullseye)"},
		{"Red Hat Enterprise Linux", "9.3", "5.14.0-362.el9.x86_64", "Red Hat Enterprise Linux 9.3 (Plow)"},
		{"Fedora Linux", "39", "6.6.9-200.fc39.x86_64", "Fedora Linux 39 (Server Edition)"},
	}

	windowsReleases = []windowsRelease{
		{"Microsoft Windows Server 2022", "20348", []int{2227, 2322, 2340, 2762, 4405, 4529, 4893, 5020, 5139, 5386}},
		{"Microsoft Windows Server 2019", "17763", []int{8276, 8389, 8511, 8647, 8755, 8880, 9020}},
		{"Microsoft Windows Server 2016", "14393", []int{2724, 8330, 8783, 8868, 8957, 9062, 9140}},
		{"Microsoft Windows 11 Pro", "22631", []int{6199, 6276, 6345, 6491, 6649, 6783, 6936, 7079, 7219, 7376}},
		{"Microsoft Windows 10 Pro", "19045", []int{6575, 6691, 6809, 6937, 7058, 7184, 7291, 7417, 7548}},
	}

	macReleases = []macRelease{
		{"14.2.1", "23C71"}, {"14.3.1", "23D60"}, {"14.4", "23E214"}, {"14.4.1", "23E224"},
		{"14.5", "23F79"}, {"14.6", "23G80"}, {"14.6.1", "23G93"}, {"14.7", "23H124"},
		{"14.7.1", "23H222"}, {"14.7.2", "23H311"},
	}
)

// GenerateOSInfo returns a coherent OSInfo for the given OS type, drawn from the
// authentic release pools. Windows picks one real UBR per selection.
// Deterministic for a given (r, os).
func GenerateOSInfo(r *rand.Rand, os OSType) OSInfo {
	switch os {
	case OSWindows:
		rel := windowsReleases[r.Intn(len(windowsReleases))] // #nosec G404
		ubr := rel.ubrs[r.Intn(len(rel.ubrs))]               // #nosec G404
		version := fmt.Sprintf("10.0.%s.%d", rel.build, ubr)
		return OSInfo{
			Type:        OSWindows,
			Name:        rel.name,
			Version:     version,
			BuildID:     rel.build,
			Description: fmt.Sprintf("Microsoft Windows [Version %s]", version),
		}
	case OSMacOS:
		rel := macReleases[r.Intn(len(macReleases))] // #nosec G404
		return OSInfo{
			Type:        OSMacOS,
			Name:        "macOS",
			Version:     rel.version,
			BuildID:     rel.build,
			Description: fmt.Sprintf("macOS %s (%s)", rel.version, rel.build),
		}
	default:
		rel := linuxReleases[r.Intn(len(linuxReleases))] // #nosec G404
		return OSInfo{
			Type:        OSLinux,
			Name:        rel.name,
			Version:     rel.version,
			BuildID:     rel.kernel,
			Description: rel.pretty,
		}
	}
}

// GenerateHostID returns an OS-appropriate host.id: a /etc/machine-id-style
// 32-char lowercase hex string on Linux, a registry MachineGuid-style GUID on
// Windows, and an uppercase IOPlatformUUID on macOS.
func GenerateHostID(r *rand.Rand, os OSType) string {
	h := randomHex(r, 16) // 32 lowercase hex chars
	switch os {
	case OSWindows:
		return formatUUID(h)
	case OSMacOS:
		return strings.ToUpper(formatUUID(h))
	default:
		return h
	}
}

// formatUUID inserts UUID dashes (8-4-4-4-12) into a 32-char hex string.
func formatUUID(hex32 string) string {
	return hex32[0:8] + "-" + hex32[8:12] + "-" + hex32[12:16] + "-" + hex32[16:20] + "-" + hex32[20:32]
}
