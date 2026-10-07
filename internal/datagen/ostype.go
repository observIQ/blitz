package datagen

import (
	"fmt"
	"strings"
)

// OS taxonomy helpers (PIPE-1036).
//
// Two axes are deliberately kept separate:
//   - Simulate-as (the fake identity): the bounded set of OSes blitz can render
//     a coherent host for — linux, windows, macos. ParseOSType gates the
//     user-facing `os:` knob against this set.
//   - Run-on (the real host): whatever the process actually runs on, reported
//     by runtime.GOOS. OSTypeFromGOOS maps that without rejecting values outside
//     the fake set (freebsd, aix, ...), since blitz may run on and truthfully
//     report such a host.
//
// SemconvOSType bridges internal naming to the wire: blitz names macOS "macos"
// internally and to the user, but a real OpenTelemetry pipeline stamps
// os.type=darwin, so that is the value emitted on records.

// simulatableOSes is the set of OSType values the fake-identity path can render
// a coherent host for. ParseOSType gates the user-facing `os:` knob against it.
var simulatableOSes = map[OSType]bool{
	OSLinux: true, OSWindows: true, OSMacOS: true,
	OSESXi: true, OSXenDom0: true, OSNutanixAHV: true, OSOpenStackKVM: true,
	OSAIX: true, OSSolaris: true, OSFreeBSD: true, OSOpenBSD: true,
}

// osAliases maps accepted spellings to their canonical OSType.
var osAliases = map[string]OSType{
	"darwin":      OSMacOS,
	"vmware":      OSESXi,
	"vmware-esxi": OSESXi,
	"xen":         OSXenDom0,
	"ahv":         OSNutanixAHV,
	"kvm":         OSOpenStackKVM,
}

// ParseOSType maps a user-supplied OS string to an OSType for the fake-identity
// path. It accepts every simulate-able OS plus a few aliases (e.g. "darwin" for
// macos, "vmware" for esxi). Unknown values return an error.
func ParseOSType(s string) (OSType, error) {
	key := strings.ToLower(strings.TrimSpace(s))
	if alias, ok := osAliases[key]; ok {
		return alias, nil
	}
	if os := OSType(key); simulatableOSes[os] {
		return os, nil
	}
	return "", fmt.Errorf("datagen: unsupported OS %q (want one of: linux, windows, macos, esxi, xen-dom0, nutanix-ahv, openstack-kvm, aix, solaris, freebsd, openbsd)", s)
}

// OSTypeFromGOOS maps a runtime.GOOS value to an OSType for the real-host path.
// The three simulate-able OSes normalize to their constants; any other GOOS
// passes through unchanged rather than being rejected.
func OSTypeFromGOOS(goos string) OSType {
	switch goos {
	case "linux":
		return OSLinux
	case "windows":
		return OSWindows
	case "darwin":
		return OSMacOS
	default:
		return OSType(goos)
	}
}

// SemconvOSType returns the OpenTelemetry semantic-convention os.type value for
// o. It differs from the OSType constant for macOS (macos -> darwin) and for the
// hypervisor-host Linux flavors, which are Linux under the hood (-> linux). The
// Unix families (aix, solaris, freebsd, openbsd) are already valid semconv
// os.type values, so they pass through unchanged.
func (o OSType) SemconvOSType() string {
	switch {
	case o == OSMacOS:
		return "darwin"
	case hypervisorHostLinux[o]:
		return "linux"
	default:
		return string(o)
	}
}
