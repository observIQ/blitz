package resource

import (
	"github.com/observiq/blitz/internal/datagen"
)

// FromIdentity builds a StaticResources for a generator worker from a resolved
// datagen host identity. It projects the identity's OpenTelemetry host.* / os.* /
// deployment.* resource attributes, stamps telemetry.source = source, and appends
// any per-generator constants in extras (same even-length key/value convention as
// Default). Empty identity fields are omitted rather than emitted blank.
//
// A nil sys means no simulated environment is wired: FromIdentity falls back to
// the running host's name, matching Default(source, extras...), so every
// generator has a single uniform construction path regardless of whether an
// Environment is present.
//
// host.image.* is deliberately never emitted: SystemIdentity.Image is an unwired
// framework hook for a future CloudIdentity source (PIPE-1036).
func FromIdentity(sys *datagen.SystemIdentity, source string, extras ...string) *StaticResources {
	if sys == nil {
		return NewStaticResources(WithHost(Hostname(), source, extras...))
	}
	attrs := projectIdentity(sys)
	attrs["telemetry.source"] = source
	for i := 0; i+1 < len(extras); i += 2 {
		attrs[extras[i]] = extras[i+1]
	}
	return NewStaticResources(attrs)
}

// projectIdentity maps a host identity to its OpenTelemetry resource attributes:
// host.name / host.id / host.arch, the os.* set (os.type carrying the semconv
// value, so macOS becomes darwin), host.ip[] / host.mac[] gathered from the
// identity's interfaces, and deployment.environment.name. Empty fields are
// omitted. It does not set telemetry.source (a per-generator constant) and never
// emits host.image.* (an unwired framework hook).
func projectIdentity(sys *datagen.SystemIdentity) map[string]any {
	attrs := make(map[string]any, 12)
	putNonEmpty(attrs, "host.name", sys.Hostname)
	putNonEmpty(attrs, "host.id", sys.HostID)
	putNonEmpty(attrs, "host.arch", string(sys.Arch))
	putNonEmpty(attrs, "os.type", sys.OSInfo.Type.SemconvOSType())
	putNonEmpty(attrs, "os.name", sys.OSInfo.Name)
	putNonEmpty(attrs, "os.version", sys.OSInfo.Version)
	putNonEmpty(attrs, "os.build_id", sys.OSInfo.BuildID)
	putNonEmpty(attrs, "os.description", sys.OSInfo.Description)
	putNonEmpty(attrs, "deployment.environment.name", string(sys.Tier))

	var ips, macs []string
	for _, iface := range sys.Interfaces {
		if iface.IPv4 != "" {
			ips = append(ips, iface.IPv4)
		}
		if iface.IPv6 != "" {
			ips = append(ips, iface.IPv6)
		}
		if iface.MACAddress != "" {
			macs = append(macs, iface.MACAddress)
		}
	}
	if len(ips) > 0 {
		attrs["host.ip"] = ips
	}
	if len(macs) > 0 {
		attrs["host.mac"] = macs
	}
	return attrs
}

// putNonEmpty sets m[key] = val only when val is non-empty, so blank identity
// fields never produce empty-string resource attributes.
func putNonEmpty(m map[string]any, key, val string) {
	if val != "" {
		m[key] = val
	}
}

// FromAppliance builds a StaticResources from a simulated network appliance:
// host.name is the device hostname, os.name/os.version its appliance OS
// (PAN-OS 11.0.4, ...), host.ip/host.mac its management interface, and
// device.manufacturer/device.model.name its vendor and model. A nil n falls
// back to the running host's name, as FromIdentity(nil) does.
func FromAppliance(n *datagen.NetworkSystemIdentity, source string) *StaticResources {
	if n == nil {
		return NewStaticResources(WithHost(Hostname(), source))
	}
	attrs := map[string]any{"telemetry.source": source}
	putNonEmpty(attrs, "host.name", n.Hostname)
	putNonEmpty(attrs, "device.manufacturer", string(n.Vendor))
	putNonEmpty(attrs, "device.model.name", n.Model)
	if n.OS != nil {
		putNonEmpty(attrs, "os.name", string(n.OS.Family))
		putNonEmpty(attrs, "os.version", n.OS.Version)
	}
	if m := n.ManagementInterface; m != nil {
		if m.IPv4 != "" {
			attrs["host.ip"] = []string{m.IPv4}
		}
		if m.MACAddress != "" {
			attrs["host.mac"] = []string{m.MACAddress}
		}
	}
	return NewStaticResources(attrs)
}
