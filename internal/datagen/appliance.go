package datagen

import (
	"fmt"
	"math/rand"
	"regexp"
)

// ApplianceVendor identifies the maker of an embedded/appliance operating
// system, spelled as the vendor is named in device telemetry. Appliance
// vendors ship closed-source OSes on storage arrays and network hardware that
// are not "Linux" from any consumer's perspective, so they are tracked
// separately from datagen's general-purpose OSType.
type ApplianceVendor string

const (
	VendorHPE      ApplianceVendor = "HPE"
	VendorF5       ApplianceVendor = "F5"
	VendorCisco    ApplianceVendor = "Cisco"
	VendorArista   ApplianceVendor = "Arista"
	VendorJuniper  ApplianceVendor = "Juniper"
	VendorPaloAlto ApplianceVendor = "Palo Alto Networks"
	VendorFortinet ApplianceVendor = "Fortinet"
)

// ApplianceOSFamily identifies the OS family running on an appliance, spelled
// as the device reports it (e.g. via "show version" / SNMP sysDescr / vendor
// API). Each family maps to exactly one vendor (see applianceOSVendor).
type ApplianceOSFamily string

const (
	FamilyNimbleOS    ApplianceOSFamily = "NimbleOS"      // HPE Nimble arrays
	Family3PAROS      ApplianceOSFamily = "HPE 3PAR OS"   // HPE 3PAR arrays
	FamilyAlletraOS   ApplianceOSFamily = "Array OS"      // HPE Alletra 6000 (NimbleOS lineage)
	FamilyStoreOnceOS ApplianceOSFamily = "HPE StoreOnce" // HPE StoreOnce backup appliances
	FamilyBIGIP       ApplianceOSFamily = "BIG-IP"        // F5 (TMOS is the underlying OS; devices report "BIG-IP")
	FamilyIOSXE       ApplianceOSFamily = "Cisco IOS XE"  // Cisco Catalyst / IOS-XE routers & switches
	FamilyNXOS        ApplianceOSFamily = "Cisco NX-OS"   // Cisco Nexus data-center switches
	FamilyEOS         ApplianceOSFamily = "Arista EOS"    // Arista switches
	FamilyJunos       ApplianceOSFamily = "Junos"         // Juniper routers, switches, SRX firewalls
	FamilyPANOS       ApplianceOSFamily = "PAN-OS"        // Palo Alto Networks NGFWs
	FamilyFortiOS     ApplianceOSFamily = "FortiOS"       // Fortinet FortiGate NGFWs
)

// ApplianceOS is the {Vendor, Family, Version} triple describing the embedded
// OS an appliance runs, e.g. {HPE, NimbleOS, 6.1.2.0} or {F5, BIG-IP,
// 17.1.0.3}. String renders the OS the way the device itself reports it.
type ApplianceOS struct {
	Vendor  ApplianceVendor
	Family  ApplianceOSFamily
	Version string
}

// applianceVersionRE matches a dotted version with a leading numeric segment
// and at least one more dot-separated segment. It accepts the real formats
// appliance OSes report: pure-numeric (NimbleOS "6.1.2.0", IOS-XE "17.12.3"),
// letter-suffixed (EOS "4.31.2F", Junos "23.4R1"), and parenthesized builds
// (NX-OS "10.3(4a)"), while rejecting non-versions like "latest" and
// single-segment values like "6".
var applianceVersionRE = regexp.MustCompile(`^\d+(\.[0-9A-Za-z()\-]+)+$`)

// applianceOSSelfReportFmt overrides how specific families render their OS
// self-report. Families not listed fall back to "<Family> <Version>", which is
// already correct for NimbleOS, BIG-IP, PAN-OS, Arista EOS, and the HPE storage
// families. The overrides capture the vendor's exact phrasing.
var applianceOSSelfReportFmt = map[ApplianceOSFamily]string{
	FamilyIOSXE:   "Cisco IOS XE Software, Version %s",
	FamilyNXOS:    "Cisco Nexus Operating System (NX-OS) Software, Version %s",
	FamilyJunos:   "Junos: %s",
	FamilyFortiOS: "FortiOS v%s",
}

// String renders the OS the way the device reports it, e.g. "NimbleOS 6.1.2.0",
// "Junos: 23.4R1", or "FortiOS v7.4.3".
func (a ApplianceOS) String() string {
	if f, ok := applianceOSSelfReportFmt[a.Family]; ok {
		return fmt.Sprintf(f, a.Version)
	}
	return fmt.Sprintf("%s %s", a.Family, a.Version)
}

// Validate reports whether the ApplianceOS is well-formed: non-empty vendor,
// family, and a dotted version in one of the real appliance formats. Returns an
// error rather than panicking, per the datagen error-return convention
// (PIPE-1003).
func (a ApplianceOS) Validate() error {
	if a.Vendor == "" {
		return fmt.Errorf("appliance OS vendor must not be empty")
	}
	if a.Family == "" {
		return fmt.Errorf("appliance OS family must not be empty")
	}
	if a.Version == "" {
		return fmt.Errorf("appliance OS version must not be empty")
	}
	if !applianceVersionRE.MatchString(a.Version) {
		return fmt.Errorf("appliance OS version %q is not a recognized version format", a.Version)
	}
	return nil
}

// applianceOSVendor maps each OS family to its one true vendor. This is what
// makes vendor/family coherence structural: a NimbleOS is always HPE.
var applianceOSVendor = map[ApplianceOSFamily]ApplianceVendor{
	FamilyNimbleOS:    VendorHPE,
	Family3PAROS:      VendorHPE,
	FamilyAlletraOS:   VendorHPE,
	FamilyStoreOnceOS: VendorHPE,
	FamilyBIGIP:       VendorF5,
	FamilyIOSXE:       VendorCisco,
	FamilyNXOS:        VendorCisco,
	FamilyEOS:         VendorArista,
	FamilyJunos:       VendorJuniper,
	FamilyPANOS:       VendorPaloAlto,
	FamilyFortiOS:     VendorFortinet,
}

// applianceOSVersions holds real published version strings per family, in the
// format the device reports (note NX-OS's parenthesized build and the
// letter-suffixed network-OS releases). Representative, not exhaustive.
var applianceOSVersions = map[ApplianceOSFamily][]string{
	FamilyNimbleOS:    {"6.1.2.0", "6.1.1.100", "6.0.0.400", "5.3.1.0"},
	Family3PAROS:      {"3.3.1.410", "3.3.1.485", "3.3.1.215"},
	FamilyAlletraOS:   {"6.1.2.502", "6.1.2.400", "6.0.0.900"},
	FamilyStoreOnceOS: {"4.3.13", "4.3.9", "4.2.3"},
	FamilyBIGIP:       {"17.1.0.3", "16.1.4", "15.1.10.2"},
	FamilyIOSXE:       {"17.12.3", "17.9.4", "17.6.5"},
	FamilyNXOS:        {"10.3(4a)", "10.2(5)", "9.3(12)"},
	FamilyEOS:         {"4.31.2F", "4.30.4M", "4.29.6M"},
	FamilyJunos:       {"23.4R1", "22.4R3", "21.4R3"},
	FamilyPANOS:       {"11.1.3", "11.0.4", "10.2.9"},
	FamilyFortiOS:     {"7.4.3", "7.2.8", "7.0.14"},
}

// GenerateApplianceOS returns a valid ApplianceOS for the given family with a
// random real version drawn from that family's version pool. The vendor is
// derived from the family, so a NimbleOS result is always HPE and can never
// carry another vendor's family. An unknown family yields a zero vendor and a
// "0.0" placeholder version, which Validate rejects — callers should pass a
// known family constant.
func GenerateApplianceOS(r *rand.Rand, family ApplianceOSFamily) ApplianceOS {
	versions := applianceOSVersions[family]
	version := "0.0"
	if len(versions) > 0 {
		version = versions[r.Intn(len(versions))] // #nosec G404
	}
	return ApplianceOS{
		Vendor:  applianceOSVendor[family],
		Family:  family,
		Version: version,
	}
}
