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

// ParseOSType maps a user-supplied OS string to an OSType for the fake-identity
// path. It accepts the three simulate-able OSes, treating "darwin" as an alias
// for "macos". Unknown values return an error.
func ParseOSType(s string) (OSType, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "linux":
		return OSLinux, nil
	case "windows":
		return OSWindows, nil
	case "macos", "darwin":
		return OSMacOS, nil
	default:
		return "", fmt.Errorf("datagen: unsupported OS %q (want one of: linux, windows, macos)", s)
	}
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
// o, which differs from the OSType constant only for macOS (macos -> darwin).
func (o OSType) SemconvOSType() string {
	if o == OSMacOS {
		return "darwin"
	}
	return string(o)
}
