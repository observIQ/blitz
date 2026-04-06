package catalog

import (
	"fmt"
	"math/rand"
	"strings"
)

// Field generator functions produce realistic random values for Windows Event fields.
// These are used by individual event definition generators to populate EventData.

// RandomSID generates a random user/group SID within the given domain SID prefix.
func RandomSID(r *rand.Rand, domainSID string) string {
	rid := r.Intn(50000) + 1000 // #nosec G404
	return fmt.Sprintf("%s-%d", domainSID, rid)
}

// WellKnownSIDs are commonly seen Windows SIDs.
var WellKnownSIDs = map[string]string{
	"S-1-0-0":  "NULL SID",
	"S-1-1-0":  "Everyone",
	"S-1-5-7":  "ANONYMOUS LOGON",
	"S-1-5-18": "LOCAL SYSTEM",
	"S-1-5-19": "LOCAL SERVICE",
	"S-1-5-20": "NETWORK SERVICE",
}

// RandomLogonID generates a random logon session ID in hex format.
func RandomLogonID(r *rand.Rand) string {
	return fmt.Sprintf("0x%x", r.Int63n(0xFFFFFFFF)+0x1000) // #nosec G404
}

// RandomProcessID generates a random process ID in hex format.
func RandomProcessID(r *rand.Rand) string {
	return fmt.Sprintf("0x%x", r.Intn(65536)+4) // #nosec G404
}

// RandomPort generates a random ephemeral port number as a string.
func RandomPort(r *rand.Rand) string {
	return fmt.Sprintf("%d", r.Intn(64512)+1024) // #nosec G404
}

// RandomIPv4 generates a random IPv4 address in the 10.x.x.x or 192.168.x.x range.
func RandomIPv4(r *rand.Rand) string {
	if r.Intn(2) == 0 { // #nosec G404
		return fmt.Sprintf("10.%d.%d.%d", r.Intn(256), r.Intn(256), r.Intn(254)+1) // #nosec G404
	}
	return fmt.Sprintf("192.168.%d.%d", r.Intn(256), r.Intn(254)+1) // #nosec G404
}

// RandomGUID generates a random GUID in {xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx} format.
func RandomGUID(r *rand.Rand) string {
	return fmt.Sprintf("{%08x-%04x-%04x-%04x-%012x}",
		r.Uint32(),                 // #nosec G404
		r.Intn(0xFFFF),             // #nosec G404
		r.Intn(0xFFFF),             // #nosec G404
		r.Intn(0xFFFF),             // #nosec G404
		r.Int63n(0xFFFFFFFFFFFF)+1, // #nosec G404
	)
}

// RandomHexID generates a random hex identifier of the given byte length, prefixed with 0x.
func RandomHexID(r *rand.Rand, nBytes int) string {
	var sb strings.Builder
	sb.WriteString("0x")
	for i := 0; i < nBytes; i++ {
		fmt.Fprintf(&sb, "%02x", r.Intn(256)) // #nosec G404
	}
	return sb.String()
}

// RandomAccessMask generates a random access mask in hex format.
func RandomAccessMask(r *rand.Rand) string {
	masks := []uint32{
		0x1, 0x2, 0x4, 0x10, 0x20, 0x80, 0x100,
		0x1F01FF, 0x120089, 0x120116, 0x1F0FFF,
		0x80000000, 0x10000000, 0x20000000, 0x40000000,
	}
	return fmt.Sprintf("0x%x", masks[r.Intn(len(masks))]) // #nosec G404
}

// PickUsername returns a random username from the provided list.
// Falls back to a generated name if the list is empty.
func PickUsername(r *rand.Rand, usernames []string) string {
	if len(usernames) > 0 {
		return usernames[r.Intn(len(usernames))] // #nosec G404
	}
	return fallbackUsernames.items[r.Intn(len(fallbackUsernames.items))] // #nosec G404
}

// PickHostname returns a random hostname from the provided list.
// Falls back to a generated name if the list is empty.
func PickHostname(r *rand.Rand, hostnames []string) string {
	if len(hostnames) > 0 {
		return hostnames[r.Intn(len(hostnames))] // #nosec G404
	}
	return fmt.Sprintf("DESKTOP-%04X", r.Intn(0xFFFF)) // #nosec G404
}

// PickIP returns a random IP from the provided list.
// Falls back to a randomly generated internal IP if the list is empty.
func PickIP(r *rand.Rand, ips []string) string {
	if len(ips) > 0 {
		return ips[r.Intn(len(ips))] // #nosec G404
	}
	return RandomIPv4(r)
}

// LogonTypes are the valid Windows logon type values.
var LogonTypes = []string{"2", "3", "4", "5", "7", "8", "9", "10", "11"}

// RandomLogonType returns a random logon type value.
func RandomLogonType(r *rand.Rand) string {
	return LogonTypes[r.Intn(len(LogonTypes))] // #nosec G404
}

// Privileges is the list of common Windows security privileges.
var Privileges = []string{
	"SeAssignPrimaryTokenPrivilege",
	"SeAuditPrivilege",
	"SeBackupPrivilege",
	"SeChangeNotifyPrivilege",
	"SeCreateGlobalPrivilege",
	"SeCreatePagefilePrivilege",
	"SeCreatePermanentPrivilege",
	"SeCreateSymbolicLinkPrivilege",
	"SeDebugPrivilege",
	"SeImpersonatePrivilege",
	"SeIncreaseBasePriorityPrivilege",
	"SeIncreaseQuotaPrivilege",
	"SeIncreaseWorkingSetPrivilege",
	"SeLoadDriverPrivilege",
	"SeMachineAccountPrivilege",
	"SeManageVolumePrivilege",
	"SeProfileSingleProcessPrivilege",
	"SeRemoteShutdownPrivilege",
	"SeRestorePrivilege",
	"SeSecurityPrivilege",
	"SeShutdownPrivilege",
	"SeSystemEnvironmentPrivilege",
	"SeSystemProfilePrivilege",
	"SeSystemtimePrivilege",
	"SeTakeOwnershipPrivilege",
	"SeTcbPrivilege",
	"SeTimeZonePrivilege",
	"SeUndockPrivilege",
}

// RandomPrivilegeList returns a newline-delimited list of 1-4 random privileges.
func RandomPrivilegeList(r *rand.Rand) string {
	count := r.Intn(4) + 1 // #nosec G404
	selected := make([]string, count)
	for i := 0; i < count; i++ {
		selected[i] = Privileges[r.Intn(len(Privileges))] // #nosec G404
	}
	return strings.Join(selected, "\n\t\t\t")
}

// ImpersonationLevels are the valid Windows impersonation levels.
var ImpersonationLevels = []string{
	"%%1832", // Identification
	"%%1833", // Impersonation
	"%%1840", // Delegation
	"%%1841", // Denied
}

// RandomImpersonationLevel returns a random impersonation level.
func RandomImpersonationLevel(r *rand.Rand) string {
	return ImpersonationLevels[r.Intn(len(ImpersonationLevels))] // #nosec G404
}

// ElevationTypes are the token elevation type values.
var ElevationTypes = []string{
	"%%1936", // Type 1 - Full token
	"%%1937", // Type 2 - Elevated token
	"%%1938", // Type 3 - Limited token
}

// RandomElevationType returns a random token elevation type.
func RandomElevationType(r *rand.Rand) string {
	return ElevationTypes[r.Intn(len(ElevationTypes))] // #nosec G404
}

// AuthPackages are common authentication package names.
var AuthPackages = []string{"NTLM", "Kerberos", "Negotiate", "MICROSOFT_AUTHENTICATION_PACKAGE_V1_0"}

// RandomAuthPackage returns a random authentication package name.
func RandomAuthPackage(r *rand.Rand) string {
	return AuthPackages[r.Intn(len(AuthPackages))] // #nosec G404
}

// LogonProcesses are common logon process names.
var LogonProcesses = []string{"User32", "Advapi", "NtLmSsp", "Negotiate", "Kerberos", "Schannel"}

// RandomLogonProcess returns a random logon process name.
func RandomLogonProcess(r *rand.Rand) string {
	return LogonProcesses[r.Intn(len(LogonProcesses))] // #nosec G404
}

// KeywordsToHex converts a keywords uint64 to its hex string representation.
func KeywordsToHex(kw uint64) string {
	if kw == 0 {
		return "0x0"
	}
	return fmt.Sprintf("0x%x", kw)
}

// fallbackUsernames are used when no environment users are configured.
var fallbackUsernames = struct{ items []string }{
	items: []string{
		"SYSTEM", "LOCAL SERVICE", "NETWORK SERVICE",
		"Administrator", "jsmith", "mjohnson", "bwilliams",
		"sgarcia", "dmiller", "kdavis",
	},
}

// MandatoryLabels are common Windows integrity level SIDs.
var MandatoryLabels = []string{
	"S-1-16-0",     // Untrusted
	"S-1-16-4096",  // Low
	"S-1-16-8192",  // Medium
	"S-1-16-8448",  // Medium Plus
	"S-1-16-12288", // High
	"S-1-16-16384", // System
}

// RandomMandatoryLabel returns a random mandatory integrity level SID.
func RandomMandatoryLabel(r *rand.Rand) string {
	return MandatoryLabels[r.Intn(len(MandatoryLabels))] // #nosec G404
}

// KerberosTicketOptions are common Kerberos ticket option hex values.
var KerberosTicketOptions = []string{
	"0x40810010", // forwardable, renewable, name-canonicalize
	"0x40800010", // forwardable, renewable
	"0x50800000", // forwardable, forwarded
	"0x40810000", // forwardable, name-canonicalize
	"0x60810010", // forwardable, forwarded, renewable, name-canonicalize
}

// RandomTicketOptions returns a random Kerberos ticket options value.
func RandomTicketOptions(r *rand.Rand) string {
	return KerberosTicketOptions[r.Intn(len(KerberosTicketOptions))] // #nosec G404
}

// TicketEncryptionTypes are common Kerberos encryption type values.
var TicketEncryptionTypes = []string{
	"0x17", // RC4-HMAC
	"0x12", // AES256-CTS-HMAC-SHA1-96
	"0x11", // AES128-CTS-HMAC-SHA1-96
}

// RandomTicketEncryptionType returns a random Kerberos encryption type.
func RandomTicketEncryptionType(r *rand.Rand) string {
	return TicketEncryptionTypes[r.Intn(len(TicketEncryptionTypes))] // #nosec G404
}

// KerberosStatusCodes are common Kerberos result status codes.
var KerberosStatusCodes = []string{
	"0x0",  // Success
	"0x6",  // Principal unknown
	"0xC",  // Policy violation
	"0x12", // Expired
	"0x17", // Password expired
	"0x18", // Pre-authentication failed
	"0x1F", // Integrity check failed
	"0x25", // Clock skew too great
}

// RandomKerberosStatus returns a random Kerberos status code.
func RandomKerberosStatus(r *rand.Rand) string {
	return KerberosStatusCodes[r.Intn(len(KerberosStatusCodes))] // #nosec G404
}
