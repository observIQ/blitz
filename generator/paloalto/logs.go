package paloalto

import (
	"fmt"
	"hash/fnv"
	"net"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

// logFieldNames is each log type's CSV field order as a PAN-OS 11.0.4-h6
// firewall sends it (testdata/panos-11.0.4-h6.log), named from Palo Alto's
// 11.0 "Syslog Field Descriptions". Field 1 is FUTURE_USE. buildRecord
// produces one value per name, so the names double as the order contract the
// tests assert.
//
// The device fills some slots its type's page calls FUTURE_USE; those take
// the name a sibling page gives the same slot (Config Version, Time Logged)
// or, unnamed anywhere, "=<value>" for the constant the device sends (System
// Event Flag names the one unnamed slot whose value varies). The
// device also sends fields the pages mark 11.1+ (Flow Type, Cluster Name).
var logFieldNames = map[string][]string{
	"TRAFFIC": {
		"FUTURE_USE", "Receive Time", "Serial Number", "Type", "Threat/Content Type", "Config Version",
		"Generated Time", "Source Address", "Destination Address", "NAT Source IP", "NAT Destination IP",
		"Rule Name", "Source User", "Destination User", "Application", "Virtual System", "Source Zone",
		"Destination Zone", "Inbound Interface", "Outbound Interface", "Log Action", "Time Logged",
		"Session ID", "Repeat Count", "Source Port", "Destination Port", "NAT Source Port",
		"NAT Destination Port", "Flags", "Protocol", "Action", "Bytes", "Bytes Sent", "Bytes Received",
		"Packets", "Start Time", "Elapsed Time", "Category", "FUTURE_USE", "Sequence Number",
		"Action Flags", "Source Country", "Destination Country", "FUTURE_USE", "Packets Sent",
		"Packets Received", "Session End Reason", "Device Group Hierarchy Level 1",
		"Device Group Hierarchy Level 2", "Device Group Hierarchy Level 3",
		"Device Group Hierarchy Level 4", "Virtual System Name", "Device Name", "Action Source",
		"Source VM UUID", "Destination VM UUID", "Tunnel ID/IMSI", "Monitor Tag/IMEI",
		"Parent Session ID", "Parent Start Time", "Tunnel Type", "SCTP Association ID", "SCTP Chunks",
		"SCTP Chunks Sent", "SCTP Chunks Received", "Rule UUID", "HTTP/2 Connection", "App Flap Count",
		"Policy ID", "Link Switches", "SD-WAN Cluster", "SD-WAN Device Type", "SD-WAN Cluster Type",
		"SD-WAN Site", "Dynamic User Group Name", "XFF Address", "Source Device Category",
		"Source Device Profile", "Source Device Model", "Source Device Vendor",
		"Source Device OS Family", "Source Device OS Version", "Source Hostname", "Source Mac Address",
		"Destination Device Category", "Destination Device Profile", "Destination Device Model",
		"Destination Device Vendor", "Destination Device OS Family", "Destination Device OS Version",
		"Destination Hostname", "Destination Mac Address", "Container ID", "POD Namespace", "POD Name",
		"Source External Dynamic List", "Destination External Dynamic List", "Host ID", "Serial Number",
		"Source Dynamic Address Group", "Destination Dynamic Address Group", "Session Owner",
		"High Resolution Timestamp", "A Slice Service Type", "A Slice Differentiator",
		"Application Subcategory", "Application Category", "Application Technology", "Application Risk",
		"Application Characteristic", "Application Container", "Tunneled Application",
		"Application SaaS", "Application Sanctioned State", "Offloaded", "Flow Type", "Cluster Name",
	},
	// THREAT: the device sends Cluster Name before Flow Type, the reverse of
	// the page.
	"THREAT": {
		"FUTURE_USE", "Receive Time", "Serial Number", "Type", "Threat/Content Type", "Config Version",
		"Generated Time", "Source Address", "Destination Address", "NAT Source IP", "NAT Destination IP",
		"Rule Name", "Source User", "Destination User", "Application", "Virtual System", "Source Zone",
		"Destination Zone", "Inbound Interface", "Outbound Interface", "Log Action", "Time Logged",
		"Session ID", "Repeat Count", "Source Port", "Destination Port", "NAT Source Port",
		"NAT Destination Port", "Flags", "IP Protocol", "Action", "URL/Filename", "Threat ID",
		"Category", "Severity", "Direction", "Sequence Number", "Action Flags", "Source Location",
		"Destination Location", "FUTURE_USE", "Content Type", "PCAP_ID", "File Digest", "Cloud",
		"URL Index", "User Agent", "File Type", "X-Forwarded-For", "Referer", "Sender", "Subject",
		"Recipient", "Report ID", "Device Group Hierarchy Level 1", "Device Group Hierarchy Level 2",
		"Device Group Hierarchy Level 3", "Device Group Hierarchy Level 4", "Virtual System Name",
		"Device Name", "FUTURE_USE", "Source VM UUID", "Destination VM UUID", "HTTP Method",
		"Tunnel ID/IMSI", "Monitor Tag/IMEI", "Parent Session ID", "Parent Start Time", "Tunnel Type",
		"Threat Category", "Content Version", "=0x0", "SCTP Association ID", "Payload Protocol ID",
		"HTTP Headers", "URL Category List", "Rule UUID", "HTTP/2 Connection", "Dynamic User Group Name",
		"XFF Address", "Source Device Category", "Source Device Profile", "Source Device Model",
		"Source Device Vendor", "Source Device OS Family", "Source Device OS Version", "Source Hostname",
		"Source MAC Address", "Destination Device Category", "Destination Device Profile",
		"Destination Device Model", "Destination Device Vendor", "Destination Device OS Family",
		"Destination Device OS Version", "Destination Hostname", "Destination MAC Address",
		"Container ID", "POD Namespace", "POD Name", "Source External Dynamic List",
		"Destination External Dynamic List", "Host ID", "Serial Number", "Domain EDL",
		"Source Dynamic Address Group", "Destination Dynamic Address Group", "Partial Hash",
		"High Resolution Timestamp", "Reason", "Justification", "A Slice Service Type",
		"Application Subcategory", "Application Category", "Application Technology", "Application Risk",
		"Application Characteristic", "Application Container", "Tunneled Application",
		"Application SaaS", "Application Sanctioned State", "Cloud Report ID", "Cluster Name", "Flow Type",
	},
	"SYSTEM": {
		"FUTURE_USE", "Receive Time", "Serial Number", "Type", "Content/Threat Type", "Config Version",
		"Generated Time", "Virtual System", "Event ID", "Object", "=0", "System Event Flag",
		"Module", "Severity", "Description", "Sequence Number", "Action Flags",
		"Device Group Hierarchy Level 1", "Device Group Hierarchy Level 2",
		"Device Group Hierarchy Level 3", "Device Group Hierarchy Level 4", "Virtual System Name",
		"Device Name", "=0", "=0", "High Resolution Timestamp",
	},
	// CONFIG: the device sends no Before or After Change Detail.
	"CONFIG": {
		"FUTURE_USE", "Receive Time", "Serial Number", "Type", "Subtype", "Config Version",
		"Generated Time", "Host", "Virtual System", "Command", "Admin", "Client", "Result",
		"Configuration Path", "Sequence Number", "Action Flags", "Device Group Hierarchy Level 1",
		"Device Group Hierarchy Level 2", "Device Group Hierarchy Level 3",
		"Device Group Hierarchy Level 4", "Virtual System Name", "Device Name", "Device Group",
		"Audit Comment", "=0", "High Resolution Timestamp",
	},
	"AUTH": {
		"FUTURE_USE", "Receive Time", "Serial Number", "Type", "Threat/Content Type", "Config Version",
		"Generated Time", "Virtual System", "Source IP", "User", "Normalize User", "Object",
		"Authentication Policy", "Repeat Count", "Authentication ID", "Vendor", "Log Action",
		"Server Profile", "Description", "Client Type", "Event Type", "Factor Number",
		"Sequence Number", "Action Flags", "Device Group Hierarchy Level 1",
		"Device Group Hierarchy Level 2", "Device Group Hierarchy Level 3",
		"Device Group Hierarchy Level 4", "Virtual System Name", "Device Name", "Virtual System ID",
		"Authentication Protocol", "UUID for rule", "High Resolution Timestamp",
		"Source Device Category", "Source Device Profile", "Source Device Model",
		"Source Device Vendor", "Source Device OS Family", "Source Device OS Version",
		"Source Hostname", "Source Mac Address", "Region", "FUTURE_USE", "User Agent", "Session ID",
		"Cluster Name",
	},
	// CORRELATION was not captured: its 11.0 page's order. The
	// device likely fills field 6 (Config Version) and appends Cluster Name
	// as on every captured type; capture one to confirm.
	"CORRELATION": {
		"FUTURE_USE", "Receive Time", "Serial Number", "Type", "Content/Threat Type", "FUTURE_USE",
		"Generated Time", "Source Address", "Source User", "Virtual System", "Category", "Severity",
		"Device Group Hierarchy Level 1", "Device Group Hierarchy Level 2",
		"Device Group Hierarchy Level 3", "Device Group Hierarchy Level 4", "Virtual System Name",
		"Device Name", "Virtual System ID", "Object Name", "Object ID", "Evidence",
	},
	"DECRYPTION": {
		"FUTURE_USE", "Receive Time", "Serial Number", "Type", "Threat/Content Type", "Config Version",
		"Generate Time", "Source Address", "Destination Address", "NAT Source IP", "NAT Destination IP",
		"Rule", "Source User", "Destination User", "Application", "Virtual System", "Source Zone",
		"Destination Zone", "Inbound Interface", "Outbound Interface", "Log Action", "Time Logged",
		"Session ID", "Repeat Count", "Source Port", "Destination Port", "NAT Source Port",
		"NAT Destination Port", "Flags", "IP Protocol", "Action", "Tunnel", "FUTURE_USE", "FUTURE_USE",
		"Source VM UUID", "Destination VM UUID", "UUID for rule", "Stage for Client to Firewall",
		"Stage for Firewall to Server", "TLS Version", "Key Exchange Algorithm", "Encryption Algorithm",
		"Hash Algorithm", "Policy Name", "Elliptic Curve", "Error Index", "Root Status", "Chain Status",
		"Proxy Type", "Certificate Serial Number", "Fingerprint", "Certificate Start Date",
		"Certificate End Date", "Certificate Version", "Certificate Size", "Common Name Length",
		"Issuer Common Name Length", "Root Common Name Length", "SNI Length", "Certificate Flags",
		"Subject Common Name", "Issuer Subject Common Name", "Root Subject Common Name",
		"Server Name Indication", "Error", "Container ID", "POD Namespace", "POD Name",
		"Source External Dynamic List", "Destination External Dynamic List",
		"Source Dynamic Address Group", "Destination Dynamic Address Group", "High Res Timestamp",
		"Source Device Category", "Source Device Profile", "Source Device Model", "Source Device Vendor",
		"Source Device OS Family", "Source Device OS Version", "Source Hostname", "Source Mac Address",
		"Destination Device Category", "Destination Device Profile", "Destination Device Model",
		"Destination Device Vendor", "Destination Device OS Family", "Destination Device OS Version",
		"Destination Hostname", "Destination Mac Address", "Sequence Number", "Action Flags",
		"Device Group Hierarchy Level 1", "Device Group Hierarchy Level 2",
		"Device Group Hierarchy Level 3", "Device Group Hierarchy Level 4", "Virtual System Name",
		"Device Name", "Virtual System ID", "Application Subcategory", "Application Category",
		"Application Technology", "Application Risk", "Application Characteristic",
		"Application Container", "Tunneled Application", "Application SaaS",
		"Application Sanctioned State", "Cluster Name",
	},
	"GLOBALPROTECT": {
		"FUTURE_USE", "Receive Time", "Serial Number", "Type", "Threat/Content Type", "Config Version",
		"Generated Time", "Virtual System", "Event ID", "Stage", "Authentication Method", "Tunnel Type",
		"Source User", "Source Region", "Machine Name", "Public IP", "Public IPv6", "Private IP",
		"Private IPv6", "Host ID", "Serial Number", "Client Version", "Client OS", "Client OS Version",
		"Repeat Count", "Reason", "Error", "Description", "Status", "Location", "Login Duration",
		"Connect Method", "Error Code", "Portal", "Sequence Number", "Action Flags",
		"High Res Timestamp", "Selection Type", "Response Time", "Priority", "Attempted Gateways",
		"Gateway", "Device Group Hierarchy Level 1", "Device Group Hierarchy Level 2",
		"Device Group Hierarchy Level 3", "Device Group Hierarchy Level 4", "Virtual System Name",
		"Device Name", "Virtual System ID", "Cluster Name",
	},
	// GTP: the device fills the page's FUTURE_USE slots with the Tunnel
	// Inspection page's fields, for tunnel and GTP-C sessions alike. The five
	// unnamed slots before the timestamp are empty on the device.
	"GTP": {
		"FUTURE_USE", "Receive Time", "Serial Number", "Type", "Threat/Content Type", "Config Version",
		"Generated Time", "Source Address", "Destination Address", "NAT Source IP", "NAT Destination IP",
		"Rule Name", "Source User", "Destination User", "Application", "Virtual System", "Source Zone",
		"Destination Zone", "Inbound Interface", "Outbound Interface", "Log Action", "Time Logged",
		"Session ID", "Repeat Count", "Source Port", "Destination Port", "NAT Source Port",
		"NAT Destination Port", "Flags", "Protocol", "Action", "GTP Event Type", "MSISDN",
		"Access Point Name", "Radio Access Technology", "GTP Message Type", "End User IP Address",
		"Tunnel Endpoint Identifier1", "Tunnel Endpoint Identifier2", "GTP Interface", "GTP Cause",
		"Severity", "Serving Country MCC", "Serving Network MNC", "Area Code", "Cell ID",
		"GTP Event Code", "Sequence Number", "Action Flags", "Source Location", "Destination Location",
		"FUTURE_USE", "Device Group Hierarchy Level 1", "Device Group Hierarchy Level 2",
		"Device Group Hierarchy Level 3", "Device Group Hierarchy Level 4", "Virtual System Name",
		"Device Name", "Tunnel ID/IMSI", "Monitor Tag/IMEI", "Parent Session ID", "Parent Start Time",
		"Tunnel", "Bytes", "Bytes Sent", "Bytes Received", "Packets", "Packets Sent", "Packets Received",
		"Maximum Encapsulation", "Unknown Protocol", "Strict Check", "Tunnel Fragment",
		"Sessions Created", "Sessions Closed", "Session End Reason", "Action Source", "Start Time",
		"Elapsed Time", "Tunnel Inspection Rule", "Remote User IP", "Remote User ID", "UUID for rule",
		"PCAP ID", "Dynamic User Group", "Source External Dynamic List",
		"Destination External Dynamic List", "FUTURE_USE", "FUTURE_USE", "FUTURE_USE", "FUTURE_USE",
		"FUTURE_USE", "High Resolution Timestamp", "A Slice Differentiator", "A Slice Service Type",
		"PDU Session ID", "Application Subcategory", "Application Category", "Application Technology",
		"Application Risk", "Application Characteristic", "Application Container",
		"Tunneled Application", "Application SaaS", "Application Sanctioned State", "Cluster Name",
	},
	"HIPMATCH": {
		"FUTURE_USE", "Receive Time", "Serial Number", "Type", "Threat/Content Type", "Config Version",
		"Generated Time", "Source User", "Virtual System", "Machine Name", "Operating System",
		"Source Address", "HIP", "Repeat Count", "HIP Type", "FUTURE_USE", "FUTURE_USE",
		"Sequence Number", "Action Flags", "Device Group Hierarchy Level 1",
		"Device Group Hierarchy Level 2", "Device Group Hierarchy Level 3",
		"Device Group Hierarchy Level 4", "Virtual System Name", "Device Name", "Virtual System ID",
		"IPv6 Source Address", "Host ID", "User Device Serial Number", "Device MAC Address",
		"High Resolution Timestamp", "Cluster Name",
	},
	// IPTAG: the device sends one empty field before the timestamp that the
	// page does not list.
	"IPTAG": {
		"FUTURE_USE", "Receive Time", "Serial", "Type", "Threat/Content Type", "Config Version",
		"Generate Time", "Virtual System", "Source IP", "Tag Name", "Event ID", "Repeat Count",
		"Timeout", "Data Source Name", "Data Source Type", "Data Source Subtype", "Sequence Number",
		"Action Flags", "DG Hierarchy Level 1", "DG Hierarchy Level 2", "DG Hierarchy Level 3",
		"DG Hierarchy Level 4", "Virtual System Name", "Device Name", "Virtual System ID",
		"FUTURE_USE", "High Resolution Timestamp", "Cluster Name",
	},
	// SCTP: the device sends a Traffic-like prefix where the page lists
	// FUTURE_USE, an event name after the chunk type, and seven empty fields
	// before the timestamp.
	"SCTP": {
		"FUTURE_USE", "Receive Time", "Serial Number", "Type", "Threat/Content Type", "Config Version",
		"Generated Time", "Source Address", "Destination Address", "NAT Source IP", "NAT Destination IP",
		"Rule Name", "Source User", "Destination User", "Application", "Virtual System", "Source Zone",
		"Destination Zone", "Inbound Interface", "Outbound Interface", "Log Action", "Time Logged",
		"Session ID", "Repeat Count", "Source Port", "Destination Port", "NAT Source Port",
		"NAT Destination Port", "Flags", "IP Protocol", "Action", "Device Group Hierarchy Level 1",
		"Device Group Hierarchy Level 2", "Device Group Hierarchy Level 3",
		"Device Group Hierarchy Level 4", "Virtual System Name", "Device Name", "Sequence Number",
		"Action Flags", "SCTP Association ID", "Payload Protocol ID", "Severity", "SCTP Chunk Type",
		"SCTP Event Type", "=0", "SCTP Verification Tag 1", "SCTP Verification Tag 2",
		"SCTP Cause Code", "Diameter App ID", "Diameter Command Code", "Diameter AVP Code",
		"SCTP Stream ID", "SCTP Association End Reason", "Op Code", "SCCP Calling Party SSN",
		"SCCP Calling Party Global Title", "SCTP Filter", "SCTP Chunks", "SCTP Chunks Sent",
		"SCTP Chunks Received", "Packets", "Packets Sent", "Packets Received", "UUID for rule",
		"FUTURE_USE", "FUTURE_USE", "FUTURE_USE", "FUTURE_USE", "FUTURE_USE", "FUTURE_USE", "FUTURE_USE",
		"High Resolution Timestamp", "Cluster Name",
	},
	"USERID": {
		"FUTURE_USE", "Receive Time", "Serial Number", "Type", "Threat/Content Type", "Config Version",
		"Generated Time", "Virtual System", "Source IP", "User", "Data Source Name", "Event ID",
		"Repeat Count", "Time Out Threshold", "Source Port", "Destination Port", "Data Source",
		"Data Source Type", "Sequence Number", "Action Flags", "Device Group Hierarchy Level 1",
		"Device Group Hierarchy Level 2", "Device Group Hierarchy Level 3",
		"Device Group Hierarchy Level 4", "Virtual System Name", "Device Name", "Virtual System ID",
		"Factor Type", "Factor Completion Time", "Factor Number", "User Group Flags", "User by Source",
		"Tag Name", "High Resolution Timestamp", "Origin Data Source", "FUTURE_USE", "Cluster Name",
	},
}

// logTypeCatalog is the full set of PAN-OS log types the generator emits, in a
// stable order, by the Type string the device sends.
var logTypeCatalog = []string{
	"TRAFFIC", "THREAT", "SYSTEM", "CONFIG", "AUTH", "CORRELATION",
	"DECRYPTION", "GLOBALPROTECT", "GTP", "HIPMATCH", "IPTAG", "SCTP", "USERID",
}

// recordCtx holds per-record values so fields that reference the same entity
// (e.g. the two timestamps, the 5-tuple, one session's counters) stay
// internally consistent.
type recordCtx struct {
	logType   string
	subtype   string
	now       time.Time
	received  time.Time
	src       string
	dst       string
	srcPort   string
	dstPort   string
	natSrc    string
	natDst    string
	natSport  string
	natDport  string
	outIf     string
	session   string
	serial    string
	device    string
	proto     string
	action    string
	endReason string
	flags     string
	category  string
	elapsed   int
	lag       int // seconds from session end to the log, beyond Elapsed Time
	bytesSent int
	bytesRecv int
	pktsSent  int
	pktsRecv  int
	carrier   carrier
	app       string
	dev       device
	rule      string
	seq       uint64
	client    client
	event     systemEvent
	gp        gpEvent
	threat    threatSig
	cert      certProfile
	config    configChange
	userSrc   userIDSource
	sctp      sctpChunk
	corr      correlationObject
	authOK    bool
	// gtpEvent is a GTP-C session's event and the message that triggered it.
	gtpEvent [2]string
	// serialSeen marks the firewall's Serial Number as written, so a second
	// Serial Number field (the user's machine) is told apart.
	serialSeen bool
}

// carrier is a mobile operator's MCC and MNC (GTP Serving Country MCC and
// Serving Network MNC).
type carrier struct{ mcc, mnc string }

// carriers are real operators: AT&T (US), Vodafone (GB), T-Mobile (DE).
var carriers = []carrier{{"310", "410"}, {"234", "15"}, {"262", "01"}}

// device is the one firewall a generator describes: its hostname (the CSV
// Device Name), serial number, per-log-type sequence counters, session
// counter, and its rules' permanent UUIDs, fixed for the generator's
// lifetime. Copies share the counters, which workers advance concurrently.
type device struct {
	name   string
	serial string
	// syslogHost is the name the syslog header carries: the device's FQDN,
	// PAN-OS's default Syslog HOSTNAME Format, or its hostname without a
	// domain.
	syslogHost string
	// seq is each log type's Sequence Number space: "a 64-bit log entry
	// identifier incremented sequentially; each log type has a unique number
	// space" (PAN-OS 11.0). Read-only map; the counters are atomic.
	seq map[string]*atomic.Uint64
	// sessions is the dataplane's session ID, which rises per session.
	sessions *atomic.Uint64
	// ruleUUIDs are the device's rules and the UUID that "permanently
	// identifies the rule".
	ruleUUIDs map[string]string
	// untrustIP is the untrust interface's address, the source of its
	// source-NATed sessions.
	untrustIP string
	// adminIP is the internal host the firewall's admins log in from (CLI,
	// auth, config changes), one per device as in the capture.
	adminIP string
	// userDomain is the NetBIOS domain of the device's users: the first
	// label of its FQDN's domain (blitz for pa-fw-01.blitz.local), else corp.
	userDomain string
	// panDBBuild is the URL database build, which rises per upgrade.
	panDBBuild *atomic.Uint64
}

// ruleNames are the security rules the simulated firewall matches.
var ruleNames = []string{"allow-web", "trust-untrust", "block-bad", "default"}

// authRule is the Authentication policy rule that sends users to the portal.
const authRule = "captive-auth"

// tunnelRule is the Tunnel Inspection policy rule.
const tunnelRule = "inspect-tunnels"

// newDevice returns the firewall name with serial, a sequence space per log
// type and a session counter starting at a random point, and one UUID per
// rule. The device's sequence numbers are 64-bit (7693931910711476224 on a
// real VM-Series) and its session IDs index a session table of up to ~800k.
func newDevice(name, serial string) device {
	d := device{
		name: name, serial: serial, syslogHost: name,
		seq: map[string]*atomic.Uint64{}, sessions: &atomic.Uint64{},
		ruleUUIDs: map[string]string{}, untrustIP: generatePrivateIP(),
		adminIP:    generatePrivateIP(),
		userDomain: "corp", panDBBuild: &atomic.Uint64{},
	}
	d.panDBBuild.Store(uint64(randInt(20000, 20900))) // #nosec G115 -- positive
	for _, lt := range logTypeCatalog {
		c := &atomic.Uint64{}
		c.Store(uint64(7_693_931_910_711_476_224 + randInt64(0, 1_000_000_000_000))) // #nosec G115 -- positive, below 2^63
		d.seq[lt] = c
	}
	d.sessions.Store(uint64(randInt(1, 100_000))) // #nosec G115 -- positive
	for _, r := range append(slices.Clone(ruleNames), authRule) {
		d.ruleUUIDs[r] = generateRuleUUID()
	}
	return d
}

// setFQDN names the device by fqdn in its syslog header and takes its users'
// domain from it.
func (d *device) setFQDN(fqdn string) {
	d.syslogHost = fqdn
	if _, domain, ok := strings.Cut(fqdn, "."); ok {
		d.userDomain, _, _ = strings.Cut(domain, ".")
	}
}

// userFor is the user the device's User-ID maps to a source address: a
// domain user, or a bare GlobalProtect-style one. It is a function of the
// address, so every record from one host names the same user, as on the
// device (the capture's records from a mapped host all carry its user).
func (d device) userFor(addr string) string {
	h := fnv.New32a()
	_, _ = h.Write([]byte(addr))
	switch h.Sum32() % 3 {
	case 0:
		return d.userDomain + `\jdoe`
	case 1:
		return d.userDomain + `\asmith`
	default:
		return "jdoe"
	}
}

// fallbackDevice is the firewall a generator describes with no environment
// appliance: the PAN-OS VM default hostname and one generated serial.
func fallbackDevice() device { return newDevice("PA-VM", generateSerial()) }

// client is the user's machine a GlobalProtect or HIP Match record describes:
// the Host ID GlobalProtect assigns, its name, and its public address.
type client struct{ hostID, machine, publicIP string }

func newClient() client {
	return client{
		hostID:   fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", randInt64(0, 0xffffffff), randInt(0, 0xffff), randInt(0, 0xffff), randInt(0, 0xffff), randInt64(0, 0xffffffffffff)),
		machine:  pick("LAPTOP-", "DESKTOP-") + strconv.Itoa(randInt(1000, 9999)),
		publicIP: generateRandomIP(),
	}
}

// systemEvent is one System log row the device sent: the subtype (daemon),
// Event ID, Object, severity, message, and the value of the second unnamed
// field after Object (1 for system-shutdown, else 0). %s in a message is an
// address, or for a PAN-DB upgrade the database version.
type systemEvent struct{ subtype, id, object, severity, description, flag string }

// systemEvents are rows from the capture.
var systemEvents = []systemEvent{
	{"general", "general", "", "informational", "User admin logged in via CLI from %s", "0"},
	{"general", "general", "", "informational", "User admin logged out via CLI from %s", "0"},
	{"general", "general", "", "informational", "Config installed", "0"},
	{"general", "general", "", "informational", "Captive Portal authentication failed for user: jdoe on %s, vsys1", "0"},
	{"general", "system-shutdown", "", "high", "The system is shutting down due to CLI Initiated.", "1"},
	{"auth", "auth-success", "local-auth", "informational", "authenticated for user 'jdoe'.   auth profile 'local-auth', vsys 'vsys1', From: %s.", "0"},
	{"vpn", "ike-config-p1-success", "", "informational", "IKE daemon configuration load phase-1 succeeded.", "0"},
	{"routing", "routed-config-p2-success", "", "informational", "Route daemon configuration load phase-2 succeeded.", "0"},
	{"url-filtering", "upgrade-url-database-success", "", "informational", "PAN-DB was upgraded to version %s.", "0"},
	{"port", "link-change", "ethernet1/1", "informational", "Port ethernet1/1: Up   10Gb/s-full duplex", "0"},
	{"dhcp", "if-clear", "ethernet1/2", "critical", "DHCP client cleared IP address on interface:ethernet1/2 due to: Configuration removed", "0"},
	{"wildfire", "wildfire-conn-success", "", "medium", "Successfully registered to Public Cloud wildfire.paloaltonetworks.com", "0"},
	{"ctd-agent", "ctd-agent-connection", "", "high", "Failed to establish GRPC connection to WifUpload service: fail to load local cert and key: tls: failed to find any PEM data in certificate input", "0"},
}

// gpEvent is one GlobalProtect event as the device logs it: the stage it
// belongs to, its authentication method, connect method, and whether the
// client's user and machine are known yet.
type gpEvent struct {
	id, stage, authMethod, connectMethod string
	portal, identified                   bool
}

// gpEvents are the events the capture logged for a portal and gateway login.
var gpEvents = []gpEvent{
	{"portal-prelogin", "before-login", "", "", true, false},
	{"portal-auth", "login", "local-database", "user-logon", true, true},
	{"portal-getconfig", "configuration", "local-database", "user-logon", true, true},
	{"gateway-prelogin", "before-login", "", "", false, false},
	{"gateway-auth", "login", "local-database", "", false, true},
	{"gateway-register", "login", "", "", false, true},
	{"gateway-hip-check", "host-info", "", "", false, true},
	{"gateway-hip-report", "host-info", "", "", false, true},
}

// clock is the time source, swapped by tests.
var clock = time.Now

// appID is an App-ID as the firewall's application database defines it: its
// default service and the metadata logs carry.
type appID struct {
	proto, port                                                         string
	subcategory, category, technology, risk, characteristics, container string
}

// knownCharacteristics is the characteristic list most web App-IDs share.
const knownCharacteristics = "used-by-malware,able-to-transfer-file,has-known-vulnerability,tunnel-other-application,pervasive-use"

// appIDs are the App-IDs the generator emits: TRAFFIC metadata from the
// capture, ssh and smtp-base from the firewall's predefined application
// database (show predefined xpath /predefined/application).
var appIDs = map[string]appID{
	"web-browsing":   {"tcp", "80", "internet-utility", "general-internet", "browser-based", "4", knownCharacteristics, ""},
	"ssl":            {"tcp", "443", "encrypted-tunnel", "networking", "browser-based", "4", knownCharacteristics, ""},
	"google-base":    {"tcp", "443", "internet-utility", "general-internet", "browser-based", "4", knownCharacteristics, ""},
	"dns-base":       {"udp", "53", "infrastructure", "networking", "network-protocol", "3", "used-by-malware,has-known-vulnerability,pervasive-use", "dns"},
	"ssh":            {"tcp", "22", "encrypted-tunnel", "networking", "client-server", "4", knownCharacteristics, ""},
	"smtp-base":      {"tcp", "25", "email", "collaboration", "client-server", "5", "used-by-malware,able-to-transfer-file,has-known-vulnerability,tunnel-other-application,prone-to-misuse,pervasive-use", "smtp"},
	"not-applicable": {"tcp", "80", "unknown", "unknown", "unknown", "1", "", ""},
	"gre":            {"gre", "0", "routing", "networking", "network-protocol", "2", "has-known-vulnerability,tunnel-other-application,pervasive-use", ""},
	"gtp-u":          {"udp", "2152", "infrastructure", "networking", "network-protocol", "1", "pervasive-use", "gtp"},
	"gtpv1-c":        {"udp", "2123", "infrastructure", "networking", "network-protocol", "1", "pervasive-use", "gtp"},
}

// apps are the session App-IDs, in a stable order.
var apps = []string{"web-browsing", "ssl", "google-base", "dns-base", "ssh", "smtp-base"}

// webApps are the App-IDs whose sessions the device gives a URL category.
var webApps = map[string]bool{"web-browsing": true, "ssl": true, "google-base": true}

// urlCategory is the URL category PAN-DB gives a destination: the predefined
// private-ip-addresses for RFC 1918, else one of the captured categories.
func urlCategory(addr string) string {
	if strings.Contains(location(addr), "-") {
		return "private-ip-addresses"
	}
	return pick("computer-and-internet-info", "search-engines", "business-and-economy",
		"content-delivery-networks", "reference-and-research", "shareware-and-freeware")
}

// threatSig is one THREAT detection as the device logged it. urlCategory is
// the Category the device gave it; empty means the destination's URL
// category.
type threatSig struct {
	app, action, severity, direction, target, id, category, flags, urlCategory string
}

// threatSigs are each THREAT subtype's detections, from the capture except
// virus and wildfire, which were not captured. Virus and wildfire
// keep their 11.0 Threat Log Fields ranges and real-sample names (a guide);
// capture them to replace.
var threatSigs = map[string][]threatSig{
	"url": {
		{"web-browsing", "block-url", "informational", "client-to-server", "urlfiltering.paloaltonetworks.com/test-malware", "(9999)", "N/A", "0x403000", "malware"},
		{"web-browsing", "block-url", "informational", "client-to-server", "urlfiltering.paloaltonetworks.com/test-phishing", "(9999)", "N/A", "0x403000", "phishing"},
		{"web-browsing", "block-url", "informational", "client-to-server", "urlfiltering.paloaltonetworks.com/test-command-and-control", "(9999)", "N/A", "0x403000", "command-and-control"},
		{"web-browsing", "block-url", "informational", "client-to-server", "www.rexswain.com/eicar.com", "(9999)", "N/A", "0x403000", "malware"},
	},
	"spyware": {
		{"dns-base", "drop-packet", "medium", "client-to-server", "test-malware.testpanw.com", "generic:test-malware.testpanw.com(247308615)", "dns-malware", "0x403000", "any"},
		{"dns-base", "drop", "high", "client-to-server", "test-c2.testpanw.com", "generic:test-c2.testpanw.com(247317591)", "dns-c2", "0x403000", "any"},
	},
	"file": {
		{"web-browsing", "deny", "low", "server-to-client", "wildfire-test-pe-file.exe", "Microsoft PE File(52060)", "N/A", "0x402000", ""},
		{"web-browsing", "alert", "low", "server-to-client", "wildfire-test-elf-file", "ELF File(52175)", "N/A", "0x402000", ""},
	},
	"vulnerability": {
		{"web-browsing", "reset-both", "medium", "server-to-client", "eicar.com", "Eicar File Detected(39040)", "code-execution", "0x2000", "any"},
		{"web-browsing", "reset-both", "medium", "client-to-server", "passwd", "HTTP Directory Traversal Request Attempt(30844)", "info-leak", "0x402000", ""},
	},
	"data": {
		{"web-browsing", "alert", "informational", "client-to-server", "upload", "cc(60000)", "N/A", "0x2000", "any"},
		{"web-browsing", "alert", "medium", "server-to-client", "secret.txt", "rx(60001)", "N/A", "0x2000", "any"},
	},
	"ml-virus": {
		{"web-browsing", "reset-both", "medium", "server-to-client", "wildfire-test-elf-file", "Machine Learning found virus(599805)", "malicious-elf", "0x402000", ""},
	},
	"virus": {
		{"web-browsing", "reset-both", "medium", "server-to-client", "invoice.exe", "Virus/Win32.WGeneric.ajbecg(340897548)", "N/A", "0x402000", ""},
	},
	"wildfire": {
		{"web-browsing", "alert", "high", "server-to-client", "invoice.exe", "Windows Executable (EXE)(52020)", "N/A", "0x402000", ""},
	},
}

// threatSubtypes are threatSigs' keys in a stable order.
var threatSubtypes = []string{"url", "spyware", "file", "vulnerability", "data", "ml-virus", "virus", "wildfire"}

// certProfile is a server certificate a decrypted session saw (capture).
type certProfile struct {
	subject, issuer, root, sni, size, flags, serial, fingerprint, start, end string
}

var certProfiles = []certProfile{
	{"secure.eicar.org", "YR2", "ISRG Root X1", "secure.eicar.org", "4096", ":::::RSA", "05a5daa92f7abdeae15", "b7e021a369971a6082bf1ee3589549c36da6fef1e1ed13350b6673265823b027", "2026/10/06 03:52:35", "2027/01/04 02:52:34"},
	{"*.wikipedia.org", "YE2", "ISRG Root X2", "www.wikipedia.org", "256", ":::::ECDSA", "062387c4d2b6e686767", "08e0b65df41fb075ec91c1dcd2cacc4e8c7dda53b754806525117b32f7b73253", "2026/08/05 12:15:41", "2026/11/03 11:15:40"},
	{"www.google.com", "WR2", "GTS Root R1", "www.google.com", "256", ":::::ECDSA", "f40bfac62b652ce00ac", "b440fda0a311e16b033bc009755b59e972f96d16c52069ef3189755919ac1f10", "2026/09/18 13:02:27", "2026/12/11 12:02:26"},
	{"httpbin.org", "Amazon RSA 2048 M01", "Amazon Root CA 1", "httpbin.org", "2048", ":::::RSA", "041adc5724dca60fb7e", "738e8af738b619ca9296866aa0f263122d4bbec797c092e861e14abfb0632ff8", "2026/06/18 17:00:00", "2027/01/02 15:59:59"},
}

// configChange is one CONFIG entry the device logged: the command, client,
// result, and configuration path (commits have none).
type configChange struct{ command, client, result, path string }

var configChanges = []configChange{
	{"set", "CLI", "Succeeded", " deviceconfig setting management quota-settings disk-quota"},
	{"commit", "CLI", "Submitted", ""},
	{"set", "CLI", "Succeeded", " vsys  vsys1 rulebase security rules  allow-web"},
	{"move", "CLI", "Succeeded", " vsys  vsys1 rulebase security rules  block-bad"},
	{"delete", "CLI", "Succeeded", " vsys  vsys1 address  web-servers"},
	{"set", "CLI", "Failed", " vsys  vsys1 global-protect global-protect-gateway  gw1"},
	{"request", "Web", "Succeeded", " config shared certificate"},
}

// userIDSource is a User-ID mapping source as the device logs it.
type userIDSource struct{ name, source, sourceType, timeout string }

var userIDSources = []userIDSource{
	{"XMLAPI", "xml-api", "", "3600"},
	{"", "vpn-client", "globalprotect", "300"},
	{"", "captive-portal", "authenticate", "3600"},
}

// sctpChunk is an SCTP log entry a client's association produces: a control
// chunk, or the association's start or end.
type sctpChunk struct{ chunk, event string }

var sctpChunks = []sctpChunk{
	{"INIT", "SCTP control chunk"},
	{"COOKIE ECHO", "SCTP control chunk"},
	{"", "SCTP association start"},
	{"", "SCTP association end"},
}

// location is how PAN-OS names an address's place in Source/Destination
// Country and Location: the predefined region range for an RFC 1918 address,
// a full country name for a public one (capture).
func location(addr string) string {
	if ip := net.ParseIP(addr).To4(); ip != nil {
		switch {
		case ip[0] == 10:
			return "10.0.0.0-10.255.255.255"
		case ip[0] == 172 && ip[1]&0xf0 == 16:
			return "172.16.0.0-172.31.255.255"
		case ip[0] == 192 && ip[1] == 168:
			return "192.168.0.0-192.168.255.255"
		}
	}
	return pick("United States", "United Kingdom", "Germany", "Australia", "Japan")
}

// correlationObject is a correlation object a CORRELATION record reports.
type correlationObject struct {
	name, id, category, severity, evidence string
}

// correlationObjects are the objects seen in a device's CORRELATION log (the
// Splunk add-on's issue 8 sample): Beacon Detection, ID 6005. Every
// correlation object is in the compromised-host category (the Correlation
// Objects page). One object; add others as device logs show them.
var correlationObjects = []correlationObject{
	{name: "Beacon Detection", id: "6005", category: "compromised-host", severity: "medium", evidence: "Host visited known malware URL"},
}

func newCtx(logType string, dev device) *recordCtx {
	c := &recordCtx{
		logType: logType,
		subtype: subtypeFor(logType),
		now:     clock(),
		src:     generatePrivateIP(),
		dst:     generateRandomIP(),
		srcPort: strconv.Itoa(randInt(32768, 60999)), // Linux ephemeral range
		serial:  dev.serial,
		device:  dev.name,
		carrier: carriers[randInt(0, len(carriers)-1)],
		dev:     dev,
		rule:    ruleNames[randInt(0, len(ruleNames)-1)],
		client:  newClient(),
		event:   systemEvents[randInt(0, len(systemEvents)-1)],
		gp:      gpEvents[randInt(0, len(gpEvents)-1)],
		cert:    certProfiles[randInt(0, len(certProfiles)-1)],
		config:  configChanges[randInt(0, len(configChanges)-1)],
		userSrc: userIDSources[randInt(0, len(userIDSources)-1)],
		sctp:    sctpChunks[randInt(0, len(sctpChunks)-1)],
		corr:    correlationObjects[randInt(0, len(correlationObjects)-1)],
		authOK:  randInt(0, 1) == 0,
		gtpEvent: [][2]string{
			{"create-session", "create-pdp-context-request"},
			{"delete-session", "delete-pdp-context-request"},
		}[randInt(0, 1)],
		app:   apps[randInt(0, len(apps)-1)],
		outIf: "ethernet1/1",
	}
	c.received = c.now
	switch logType {
	case "SYSTEM", "CONFIG":
		// The management plane can receive a System or Config log a second
		// before it is stamped (the capture shows both).
		if logType == "SYSTEM" {
			c.subtype = c.event.subtype
		}
		c.received = c.now.Add(-time.Duration(randInt(0, 1)) * time.Second)
	case "THREAT":
		sigs := threatSigs[c.subtype]
		c.threat = sigs[randInt(0, len(sigs)-1)]
		c.app = c.threat.app
		// The detection's captured flags say whether its session was
		// translated, so the destination follows them.
		c.dst = generatePrivateIP()
		if f, _ := strconv.ParseUint(c.threat.flags, 0, 64); f&natFlag != 0 {
			c.dst = generatePublicIP()
		}
	case "DECRYPTION":
		c.app = "web-browsing"
		c.dstPort = "443"
	case "GTP":
		c.app = pick("gre", "gtp-u", "gtpv1-c")
		c.dst = generatePrivateIP() // the tunnel peer
	case "SCTP":
		c.app, c.proto, c.dst = "", "sctp", generatePrivateIP()
		c.dstPort = pick("3868", "2905", "36412") // Diameter, M3UA, S1AP
	case "TRAFFIC":
		if c.subtype == "drop" {
			c.app = "not-applicable"
		}
	}
	if a, ok := appIDs[c.app]; ok {
		c.proto, c.dstPort = a.proto, a.port
		if logType == "DECRYPTION" {
			c.dstPort = "443"
		}
	}
	if c.proto == "gre" {
		c.srcPort = "0"
	}
	if c.app == "gtp-u" {
		c.srcPort = c.dstPort
	}
	if n, ok := dev.seq[logType]; ok {
		c.seq = n.Add(1)
	}
	c.setSession()
	c.setNAT()
	return c
}

// setSession sets the action, end reason, flags, URL category, duration, and
// per-direction counters of the record's one session so they agree with its
// subtype and protocol, as the device logs them.
func (c *recordCtx) setSession() {
	c.action, c.endReason, c.category, c.flags = "allow", "n/a", "any", "0x0"
	switch c.logType {
	case "TRAFFIC":
		// Flags here leave out the NAT bit; setNAT adds it. Lags, reasons,
		// and categories are the capture's.
		switch c.subtype {
		case "start":
			c.lag = randInt(0, 7)
			c.flags = pick("0x0", "0x100000", "0x200000")
			if c.app == "google-base" && randInt(0, 2) > 0 {
				c.category = urlCategory(c.dst)
			}
			c.pktsSent, c.pktsRecv = randInt(1, 3), randInt(0, 2)
		case "end":
			c.elapsed = randInt(0, 600)
			// The device mostly logs a closed session 17-20 s after it ends.
			c.lag = randInt(3, 94)
			if randInt(0, 9) < 7 {
				c.lag = randInt(17, 20)
			}
			c.setEnd()
			c.pktsSent, c.pktsRecv = randInt(2, 400), randInt(1, 400)
		case "deny":
			// Denied after App-ID by an allowing rule's profile: the device
			// logs the action as allow.
			c.lag = randInt(0, 4)
			c.endReason, c.flags = "policy-deny", pick("0x1000013", "0x1200013")
			if c.app == "web-browsing" && randInt(0, 1) == 0 {
				c.endReason, c.flags, c.category = "auth-policy-redirect", "0x20", urlCategory(c.dst)
			}
			c.pktsSent, c.pktsRecv = randInt(1, 3), randInt(0, 2)
		case "drop":
			c.lag = randInt(1, 4)
			c.action, c.endReason, c.session, c.outIf = pick("deny", "drop"), "policy-deny", "0", ""
			c.pktsSent = 1
		}
	case "THREAT":
		c.action, c.flags, c.category = c.threat.action, c.threat.flags, c.threat.urlCategory
		switch {
		case c.subtype == "wildfire":
			c.category = pick("malware", "phishing", "grayware", "benign")
		case c.category == "":
			c.category = urlCategory(c.dst)
		}
	case "DECRYPTION":
		c.flags = pick("0x1000000", "0x1200000") // setNAT adds the NAT bit
	case "GTP":
		if c.app == "gre" {
			c.flags = "0x100000"
		}
		c.pktsSent = 1
	case "SCTP":
		if c.sctp.event == "SCTP association end" {
			c.endReason = "shutdown-from-endpoint"
			c.pktsSent, c.pktsRecv = randInt(2, 10), randInt(2, 10)
		}
	}
	c.bytesSent = c.pktsSent * randInt(60, 1500)
	c.bytesRecv = c.pktsRecv * randInt(60, 1500)
}

// setEnd sets an end record's end reason, flags, and URL category.
func (c *recordCtx) setEnd() {
	c.endReason = pick("aged-out", "threat")
	if c.proto == "tcp" {
		c.endReason = pick("tcp-fin", "tcp-fin", "tcp-rst-from-client", "tcp-rst-from-server", "aged-out", "threat")
	}
	switch c.endReason {
	case "aged-out":
		c.flags = pick("0x19", "0x1000019")
	case "threat":
		c.flags = pick("0x10", "0x19")
		if c.proto == "udp" {
			c.flags = pick("0x11", "0x12")
		}
	default:
		c.flags = pick("0x7a", "0x1c", "0x41c", "0x100001c")
	}
	if !webApps[c.app] {
		return
	}
	c.category = urlCategory(c.dst)
	switch {
	case c.app == "web-browsing" && randInt(0, 3) == 0:
		c.category = "any"
	case c.app == "google-base" && randInt(0, 6) == 0:
		c.category = "not-resolved"
	case c.endReason == "threat" && c.category != "private-ip-addresses" && randInt(0, 1) == 0:
		c.category = pick("malware", "phishing", "command-and-control")
	}
}

// setNAT applies the device's outbound source NAT: a session to a public
// address leaves from the untrust interface's address on a new port, its
// destination unchanged; a private destination or a dropped session is not
// translated. A translated TRAFFIC or DECRYPTION session's Flags carry natFlag.
func (c *recordCtx) setNAT() {
	c.natSrc, c.natDst, c.natSport, c.natDport = "0.0.0.0", "0.0.0.0", "0", "0"
	if strings.Contains(location(c.dst), "-") || c.outIf == "" {
		return
	}
	c.natSrc, c.natDst = c.dev.untrustIP, c.dst
	c.natSport, c.natDport = strconv.Itoa(randInt(1024, 65535)), c.dstPort
	if c.logType == "TRAFFIC" || c.logType == "DECRYPTION" {
		f, _ := strconv.ParseUint(c.flags, 0, 64)
		c.flags = fmt.Sprintf("%#x", f|natFlag)
	}
}

// natFlag is the Flags bit the device sets on a translated session.
const natFlag = 0x400000

// buildRecord returns a log type's generated time, which the syslog header
// carries, and its CSV fields, one value per field in logFieldNames order.
func buildRecord(logType string, dev device) (time.Time, []string) {
	names, ok := logFieldNames[logType]
	if !ok {
		names = logFieldNames["SYSTEM"]
		logType = "SYSTEM"
	}
	c := newCtx(logType, dev)
	out := make([]string, len(names))
	for i, name := range names {
		out[i] = c.valueFor(i, name)
	}
	return c.now, out
}

// isGTPC reports a GTP-C session, which carries the GTP fields a tunnel
// session leaves zero.
func (c *recordCtx) isGTPC() bool { return c.app == "gtpv1-c" }

// valueFor maps a field name to the value the device sends. The first field
// (position 0) is always FUTURE_USE="1".
func (c *recordCtx) valueFor(pos int, name string) string {
	if pos == 0 {
		return "1"
	}
	if v, ok := strings.CutPrefix(name, "="); ok {
		return v
	}
	ts := func(t time.Time) string { return t.Format("2006/01/02 15:04:05") }
	app := appIDs[c.app]
	switch name {
	case "FUTURE_USE":
		return ""
	case "Receive Time":
		return ts(c.received)
	case "Generated Time", "Generate Time", "Time Logged":
		return ts(c.now)
	case "Start Time":
		if c.logType == "GTP" {
			return ts(c.now.Add(-time.Duration(randInt(1, 5)) * time.Second))
		}
		return ts(c.now.Add(-time.Duration(c.elapsed+c.lag) * time.Second))
	case "Factor Completion Time":
		return ts(c.now) // the factor completes in the second the record is written
	case "High Resolution Timestamp", "High Res Timestamp":
		return c.now.Format("2006-01-02T15:04:05.000-07:00")
	case "Config Version":
		return "2816" // PAN-OS 11.0
	case "Serial Number", "Serial":
		if !c.serialSeen {
			c.serialSeen = true
			return c.serial
		}
		return "" // the user's machine; the device leaves it empty
	case "Host ID":
		if c.logType == "GLOBALPROTECT" && c.gp.identified && !c.gp.portal {
			return pick("", c.client.hostID)
		}
		return ""
	case "Type":
		return c.logType
	case "Threat/Content Type", "Content/Threat Type", "Subtype":
		return c.subtype
	case "Source Address", "Source IP":
		if c.logType == "HIPMATCH" {
			return c.client.publicIP
		}
		return c.src
	case "Destination Address":
		return c.dst
	case "NAT Source IP":
		return c.natSrc
	case "NAT Destination IP":
		return c.natDst
	case "NAT Source Port":
		return c.natSport
	case "NAT Destination Port":
		return c.natDport
	case "Public IP":
		return c.client.publicIP
	case "Public IPv6", "Private IPv6", "IPv6 Source Address":
		return "0.0.0.0"
	case "Private IP":
		if c.gp.stage == "host-info" {
			return c.client.publicIP
		}
		return "0.0.0.0"
	case "Rule Name", "Rule":
		return c.rule
	case "UUID for rule", "Rule UUID":
		if c.logType == "AUTH" {
			return c.dev.ruleUUIDs[authRule]
		}
		return c.dev.ruleUUIDs[c.rule]
	case "Source User":
		switch c.logType {
		case "GLOBALPROTECT":
			if !c.gp.identified {
				return ""
			}
			return "jdoe"
		case "HIPMATCH":
			return "jdoe"
		case "GTP", "SCTP":
			return ""
		}
		return c.dev.userFor(c.src)
	case "User", "Normalize User", "User by Source":
		if c.logType == "AUTH" {
			return "jdoe"
		}
		return c.dev.userFor(c.src)
	case "Application":
		return c.app
	case "Tunneled Application":
		return c.app
	case "Application Subcategory":
		return app.subcategory
	case "Application Category":
		return app.category
	case "Application Technology":
		return app.technology
	case "Application Risk":
		return app.risk
	case "Application Characteristic":
		return app.characteristics
	case "Application Container":
		return app.container
	case "Application SaaS", "Application Sanctioned State":
		return "no"
	case "Virtual System":
		if c.logType == "SYSTEM" || c.logType == "CONFIG" || (c.logType == "GLOBALPROTECT" && c.gp.id == "gateway-prelogin") {
			return ""
		}
		return "vsys1"
	case "Virtual System ID":
		return "1"
	case "Source Zone":
		return "trust"
	case "Destination Zone":
		return "untrust"
	case "Inbound Interface":
		return "ethernet1/2"
	case "Outbound Interface":
		return c.outIf
	case "Log Action":
		return "log-forwarding-default"
	case "Session ID":
		if c.logType == "AUTH" {
			return "0"
		}
		// Only a session takes an ID, so records without one (and drops,
		// set to 0 above) leave the numbering consecutive, as the capture's is.
		if c.session == "" {
			c.session = strconv.FormatUint(c.dev.sessions.Add(1), 10)
		}
		return c.session
	case "Repeat Count", "Factor Number":
		return "1"
	case "Source Port":
		if c.logType == "USERID" {
			return "0"
		}
		return c.srcPort
	case "Destination Port":
		if c.logType == "USERID" {
			return "0"
		}
		return c.dstPort
	case "Flags":
		return c.flags
	case "IP Protocol", "Protocol":
		return c.proto
	case "Action":
		return c.action
	case "Bytes":
		return strconv.Itoa(c.bytesSent + c.bytesRecv)
	case "Bytes Sent":
		return strconv.Itoa(c.bytesSent)
	case "Bytes Received":
		return strconv.Itoa(c.bytesRecv)
	case "Packets":
		return strconv.Itoa(c.pktsSent + c.pktsRecv)
	case "Packets Sent":
		return strconv.Itoa(c.pktsSent)
	case "Packets Received":
		return strconv.Itoa(c.pktsRecv)
	case "Elapsed Time":
		return strconv.Itoa(c.elapsed)
	case "Category":
		if c.logType == "CORRELATION" {
			return c.corr.category
		}
		return c.category
	case "URL Category List":
		if c.subtype == "url" {
			return c.category
		}
		return ""
	case "Sequence Number":
		return strconv.FormatUint(c.seq, 10)
	case "Action Flags", "User Group Flags":
		return "0x0"
	case "Source Location", "Source Country":
		return location(c.src)
	case "Destination Location", "Destination Country":
		return location(c.dst)
	case "Source Region":
		return location(c.client.publicIP)
	case "Session End Reason", "SCTP Association End Reason":
		return c.endReason
	case "Device Name":
		return c.device
	case "Device Group Hierarchy Level 1", "Device Group Hierarchy Level 2",
		"Device Group Hierarchy Level 3", "Device Group Hierarchy Level 4",
		"DG Hierarchy Level 1", "DG Hierarchy Level 2", "DG Hierarchy Level 3", "DG Hierarchy Level 4",
		"Device Group":
		return "0"
	case "Severity":
		switch c.logType {
		case "SYSTEM":
			return c.event.severity
		case "THREAT":
			return c.threat.severity
		case "GTP":
			if c.isGTPC() {
				return pick("informational", "low", "medium")
			}
			return "Unknown"
		case "SCTP":
			return "informational"
		case "CORRELATION":
			return c.corr.severity
		}
		// Safety net, unreachable today: every log type with a Severity
		// field has a case above.
		return pick("critical", "high", "medium", "low", "informational")
	case "Direction":
		return c.threat.direction
	case "Action Source":
		return "from-policy"
	case "URL/Filename":
		return c.threat.target
	case "Threat ID":
		return c.threat.id
	case "Threat Category":
		return c.threat.category
	case "Content Version":
		if c.subtype == "url" || c.subtype == "data" {
			return "AppThreat-0-0" // URL and data filtering use no content signature
		}
		return "AppThreat-9155-10272"
	case "HTTP Method":
		if c.subtype == "url" {
			return "get"
		}
		return ""
	case "URL Index":
		if c.subtype == "spyware" {
			return "0"
		}
		return "1"
	case "Partial Hash":
		if c.subtype == "ml-virus" {
			return strconv.FormatInt(randInt64(1e18, 9e18), 10)
		}
		return "0"
	case "Payload Protocol ID":
		switch c.logType {
		case "THREAT":
			return "4294967295" // no SCTP payload
		case "SCTP":
			if c.sctp.chunk == "" {
				return "0"
			}
		}
		return ""
	case "PCAP_ID", "PCAP ID", "Report ID", "HTTP/2 Connection", "App Flap Count", "Offloaded",
		"Tunnel ID/IMSI", "Parent Session ID", "SCTP Chunks", "SCTP Chunks Sent", "SCTP Chunks Received",
		"Remote User ID", "A Slice Differentiator", "A Slice Service Type", "PDU Session ID",
		"Maximum Encapsulation", "Unknown Protocol", "Strict Check", "Tunnel Fragment",
		"Sessions Created", "Sessions Closed", "Login Duration", "Timeout", "SCTP Stream ID":
		if c.logType != "GTP" && (name == "A Slice Differentiator" || name == "A Slice Service Type") {
			return ""
		}
		if c.logType == "SCTP" && c.sctp.event == "SCTP association end" && strings.HasPrefix(name, "SCTP Chunks") {
			return strconv.Itoa(map[string]int{"SCTP Chunks": 10, "SCTP Chunks Sent": 4, "SCTP Chunks Received": 6}[name])
		}
		if c.logType == "GTP" && c.isGTPC() && name == "Tunnel ID/IMSI" {
			return strconv.FormatInt(randInt64(100000000000000, 999999999999999), 10)
		}
		return "0"
	case "SCTP Association ID":
		if c.logType == "SCTP" {
			return strconv.Itoa(randInt(1, 1000))
		}
		return "0"
	case "Tunnel Type":
		if c.logType == "GLOBALPROTECT" {
			return ""
		}
		return "N/A" // no tunnel inspection
	case "Tunnel":
		if c.logType == "GTP" {
			return "tunnel"
		}
		return pick("N/A", "HTTP2-CONNECTION")
	case "Tunnel Inspection Rule":
		return tunnelRule
	case "Flow Type":
		return "NonProxyTraffic"

	// GTP: zero on a tunnel session; a GTP-C session's from the 11.0 GTP
	// Log Fields page. The GTP-C values are page-sourced (no
	// capture); GTP Event Type and Message Type values are not listed there.
	case "GTP Event Type", "Radio Access Technology":
		if !c.isGTPC() {
			return "Unknown"
		}
		if name == "Radio Access Technology" {
			return pick("UTRAN", "GERAN")
		}
		return c.gtpEvent[0]
	case "GTP Message Type":
		if c.isGTPC() {
			return c.gtpEvent[1]
		}
		return ""
	case "MSISDN":
		if c.isGTPC() {
			return strconv.FormatInt(randInt64(10000000000, 999999999999999), 10)
		}
		return ""
	case "Access Point Name":
		if c.isGTPC() {
			return pick("internet.apn", "ims.apn")
		}
		return ""
	case "Serving Country MCC":
		if c.isGTPC() {
			return c.carrier.mcc
		}
		return "0"
	case "Serving Network MNC":
		if c.isGTPC() {
			return c.carrier.mnc
		}
		return "0"
	case "Tunnel Endpoint Identifier1", "Tunnel Endpoint Identifier2", "GTP Event Code", "Area Code", "Cell ID", "GTP Interface", "GTP Cause":
		if c.isGTPC() {
			return strconv.Itoa(randInt(1, 65535))
		}
		return "0"

	// SYSTEM
	case "Event ID":
		switch c.logType {
		case "SYSTEM":
			return c.event.id
		case "GLOBALPROTECT":
			return c.gp.id
		case "IPTAG":
			return pick("register", "unregister")
		}
		return "0"
	case "Object":
		if c.logType == "AUTH" {
			return "local-auth"
		}
		return c.event.object
	case "System Event Flag":
		return c.event.flag
	case "Module":
		return "general"
	case "Description":
		switch c.logType {
		case "SYSTEM":
			switch {
			case c.event.id == "upgrade-url-database-success":
				return fmt.Sprintf(c.event.description, fmt.Sprintf("%s.%05d", c.now.Format("20060102"), c.dev.panDBBuild.Add(1)))
			case strings.Contains(c.event.description, "Captive Portal"):
				return fmt.Sprintf(c.event.description, generatePrivateIP()) // an internal user
			case strings.Contains(c.event.description, "%s"):
				return fmt.Sprintf(c.event.description, c.dev.adminIP)
			}
			return c.event.description
		case "AUTH":
			if !c.authOK {
				return "Invalid username/password."
			}
		case "GLOBALPROTECT":
			switch c.gp.id {
			case "portal-getconfig":
				return "Config name: agent-config, Machine Certificate CN : "
			case "gateway-hip-check":
				return "HIP report is needed"
			}
		}
		return ""

	// CONFIG
	case "Host":
		return c.dev.adminIP
	case "Command":
		return c.config.command
	case "Admin":
		return pick("admin", "apiadmin")
	case "Client":
		return c.config.client
	case "Result":
		return c.config.result
	case "Configuration Path":
		return c.config.path

	// AUTH
	case "Authentication Policy":
		return authRule
	case "Authentication ID":
		return strconv.FormatUint(c.seq+38_654_705_724, 10) // its own 64-bit space, near the log's
	case "Client Type":
		return "Authentication Portal"
	case "Event Type":
		if c.authOK {
			return "Authentication Success"
		}
		return "User Password Failure"

	// DECRYPTION
	case "Stage for Client to Firewall":
		return "Finished"
	case "Stage for Firewall to Server":
		return "Finished"
	case "TLS Version":
		return "TLS1.2"
	case "Key Exchange Algorithm":
		return "ECDHE"
	case "Encryption Algorithm":
		return pick("AES_256_GCM", "AES_128_GCM")
	case "Hash Algorithm":
		return pick("SHA384", "SHA256")
	case "Policy Name":
		return "decrypt-outbound"
	case "Elliptic Curve":
		return "secp256r1"
	case "Error Index":
		return "None"
	case "Root Status":
		return "trusted"
	case "Chain Status":
		return "Trusted"
	case "Proxy Type":
		return "Forward"
	case "Certificate Serial Number":
		return c.cert.serial
	case "Fingerprint":
		return c.cert.fingerprint
	case "Certificate Start Date":
		return c.cert.start
	case "Certificate End Date":
		return c.cert.end
	case "Certificate Version":
		return "V3"
	case "Certificate Size":
		return c.cert.size
	case "Common Name Length":
		return strconv.Itoa(len(c.cert.subject))
	case "Issuer Common Name Length":
		return strconv.Itoa(len(c.cert.issuer))
	case "Root Common Name Length":
		return strconv.Itoa(len(c.cert.root))
	case "SNI Length":
		return strconv.Itoa(len(c.cert.sni))
	case "Certificate Flags":
		return c.cert.flags
	case "Subject Common Name":
		return c.cert.subject
	case "Issuer Subject Common Name":
		return c.cert.issuer
	case "Root Subject Common Name":
		return c.cert.root
	case "Server Name Indication":
		return c.cert.sni

	// GLOBALPROTECT
	case "Stage":
		return c.gp.stage
	case "Authentication Method":
		return c.gp.authMethod
	case "Machine Name":
		if c.logType == "GLOBALPROTECT" && !c.gp.identified {
			return ""
		}
		return c.client.machine
	case "Client Version":
		return "Browser"
	case "Client OS":
		if c.gp.stage == "host-info" {
			return "Browser"
		}
		if c.gp.identified { // matches the Windows Client OS Version
			return "Windows"
		}
		return pick("Windows", "Linux")
	case "Operating System":
		return "Windows"
	case "Client OS Version":
		if c.gp.identified && c.gp.stage != "host-info" {
			return "Microsoft Windows 10 Pro"
		}
		return ""
	case "Error":
		if c.logType == "GLOBALPROTECT" && c.gp.id == "gateway-prelogin" {
			return "Gateway does not exist"
		}
		return ""
	case "Status":
		if c.gp.id == "gateway-prelogin" {
			return "failure"
		}
		return "success"
	case "Connect Method":
		return c.gp.connectMethod
	case "Error Code":
		if c.gp.id == "gateway-prelogin" {
			return "1"
		}
		return "0"
	case "Portal":
		switch {
		case c.gp.id == "gateway-prelogin":
			return ""
		case c.gp.portal:
			return "portal1"
		}
		return "gw1"

	// HIPMATCH
	case "HIP":
		return pick("win-host", "any-host", "win-hp")
	case "HIP Type":
		return pick("object", "profile")

	// IPTAG, USERID
	case "Tag Name":
		if c.logType == "IPTAG" {
			return pick("web-servers", "quarantine")
		}
		return ""
	case "Data Source Name":
		if c.logType == "IPTAG" {
			return "XMLAPI"
		}
		return c.userSrc.name
	case "Data Source":
		return c.userSrc.source
	case "Data Source Type":
		if c.logType == "IPTAG" {
			return "xml-api"
		}
		return c.userSrc.sourceType
	case "Data Source Subtype":
		return "unknown"
	case "Time Out Threshold":
		if c.subtype == "logout" {
			return "0"
		}
		return c.userSrc.timeout

	// SCTP
	case "SCTP Chunk Type":
		return c.sctp.chunk
	case "SCTP Event Type":
		return c.sctp.event
	case "SCTP Verification Tag 1", "SCTP Verification Tag 2":
		if c.sctp.chunk == "INIT" {
			return ""
		}
		return fmt.Sprintf("0x%x", randInt64(1, 0xffffffff))

	// CORRELATION (page-sourced)
	case "Object Name":
		return c.corr.name
	case "Object ID":
		return c.corr.id
	case "Evidence":
		// The object's match count; medium severity is repeated matches.
		return fmt.Sprintf("%s (%d times).", c.corr.evidence, randInt(2, 50))
	default:
		// Feature-gated / optional fields a firewall emits empty when the
		// feature is inactive (Device-ID, EDL/DAG, SD-WAN, VM, container, etc.).
		return ""
	}
}

// subtypeFor returns a subtype (field 5) for a log type, from those the
// device sent.
func subtypeFor(logType string) string {
	switch logType {
	case "TRAFFIC":
		return pick("start", "end", "drop", "deny")
	case "THREAT":
		return threatSubtypes[randInt(0, len(threatSubtypes)-1)]
	case "SYSTEM":
		return "" // set from the system event
	case "AUTH":
		return "Local Database" // the authentication profile's type
	case "CORRELATION":
		return "" // left empty in the device's log
	case "GTP":
		return "start"
	case "USERID":
		return pick("login", "logout")
	case "SCTP":
		return "27"
	default: // CONFIG, DECRYPTION, GLOBALPROTECT, HIPMATCH, IPTAG: unused
		return "0"
	}
}

// generatePaloAltoLog builds one PAN-OS syslog line: the BSD syslog timestamp
// header followed by the comma-separated CSV record for a randomly chosen log
// type.
func generatePaloAltoLog(dev device) (line, severity string) {
	logType := logTypeCatalog[randInt(0, len(logTypeCatalog)-1)]
	return renderLogLine(logType, dev)
}

// renderLogLine renders the syslog header plus the comma-joined CSV fields,
// and returns the line's syslog severity.
func renderLogLine(logType string, dev device) (line, severity string) {
	// BSD syslog (PAN-OS's default format): an RFC 3164 timestamp with a
	// space-padded day, then the firewall's hostname field.
	now, fields := buildRecord(logType, dev)
	severity = "INFO"
	if logType == "THREAT" || logType == "SYSTEM" {
		severity = syslogSeverity[fields[slices.Index(logFieldNames[logType], "Severity")]]
	}
	timestamp := now.Format("Jan _2 15:04:05") + " " + dev.syslogHost
	names := logFieldNames[logType]
	for i, f := range fields {
		if f != "" && alwaysQuoted[logType][names[i]] {
			fields[i] = `"` + strings.ReplaceAll(f, `"`, `""`) + `"`
			continue
		}
		fields[i] = csvField(f)
	}
	return timestamp + " " + strings.Join(fields, ","), severity
}

// syslogSeverity maps a Threat or System log's Severity to the syslog
// severity the device sends (capture); every other type is Info.
var syslogSeverity = map[string]string{
	"informational": "INFO",
	"low":           "NOTICE",
	"medium":        "WARN",
	"high":          "ERROR",
	"critical":      "CRITICAL",
}

// alwaysQuoted are the free-text fields the device quotes whenever they hold
// a value, comma or not (capture).
var alwaysQuoted = map[string]map[string]bool{
	"SYSTEM":        {"Description": true},
	"THREAT":        {"URL/Filename": true, "URL Category List": true},
	"GLOBALPROTECT": {"Client OS Version": true, "Description": true},
}

// csvField quotes a field that holds a comma or quote, doubling any quote, as
// PAN-OS writes a message such as "Connection to Update server closed: ...,
// source: ...".
func csvField(f string) string {
	if !strings.ContainsAny(f, ",\"") {
		return f
	}
	return `"` + strings.ReplaceAll(f, `"`, `""`) + `"`
}

// ---- shared value helpers ----

// pick returns one of the provided options at random.
func pick(opts ...string) string {
	return opts[randInt(0, len(opts)-1)]
}

// generateSerial returns a VM-Series serial: 15 uppercase hex digits (a
// hardware model's is 12 decimal digits).
func generateSerial() string {
	return fmt.Sprintf("%015X", randInt64(0, 1<<60-1))
}

// generateRuleUUID returns a random UUIDv4-shaped rule identifier.
func generateRuleUUID() string {
	return fmt.Sprintf("%08x-%04x-4%03x-%04x-%012x",
		randInt64(0, 0xffffffff), randInt(0, 0xffff), randInt(0, 0xfff),
		randInt(0x8000, 0xbfff), randInt64(0, 0xffffffffffff))
}
