package paloalto

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// PAN-OS 11.0 CSV field ORDER per log type, validated against Palo Alto's public
// "Syslog Field Descriptions" (PAN-OS 11.0). Each list is the exact ordered set
// of field names in the record body; field 1 is FUTURE_USE. buildLogFields
// produces a value slice of the same length, so len(values) == len(names) is the
// field count and the names double as the order contract the tests assert.
//
// The authoritative page is unified across 11.0-and-later and appends/interleaves
// later-version fields. For the four high-width types we exclude ONLY the fields
// the page explicitly marks as introduced after 11.0:
//   - 11.1+: Flow Type, Cluster Name, AI Traffic, AI Forward Error, K8S Cluster ID
//   - 12.1.2+: Source Adv DevID, Destination Adv DevID
//
// Everything at or before 11.0 is retained. Excluded fields per type are noted
// on each list below.
var logFieldNames = map[string][]string{
	// TRAFFIC 11.0 = 115. From the rendered spec format string (headless Chrome);
	// full page = 130, excluded post-11.0 (marked on the page): Flow Type,
	// Cluster Name, AI Traffic, AI Forward Error, K8S Cluster ID (11.1+), the TCP
	// telemetry block, and Source/Destination Adv DevID (12.1.2+). Kept through
	// Offloaded.
	"TRAFFIC": {
		"FUTURE_USE", "Receive Time", "Serial Number", "Type", "Threat/Content Type", "FUTURE_USE",
		"Generated Time", "Source Address", "Destination Address", "NAT Source IP", "NAT Destination IP",
		"Rule Name", "Source User", "Destination User", "Application", "Virtual System", "Source Zone",
		"Destination Zone", "Inbound Interface", "Outbound Interface", "Log Action", "FUTURE_USE",
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
		"Application SaaS", "Application Sanctioned State", "Offloaded",
	},
	// THREAT 11.0 = 121. From the rendered spec format string (headless Chrome);
	// full page = 123, excluded post-11.0 (marked on the page): Flow Type and
	// Cluster Name (both 11.1+). Kept through Cloud Report ID.
	"THREAT": {
		"FUTURE_USE", "Receive Time", "Serial Number", "Type", "Threat/Content Type", "FUTURE_USE",
		"Generated Time", "Source Address", "Destination Address", "NAT Source IP", "NAT Destination IP",
		"Rule Name", "Source User", "Destination User", "Application", "Virtual System", "Source Zone",
		"Destination Zone", "Inbound Interface", "Outbound Interface", "Log Action", "FUTURE_USE",
		"Session ID", "Repeat Count", "Source Port", "Destination Port", "NAT Source Port",
		"NAT Destination Port", "Flags", "IP Protocol", "Action", "URL/Filename", "Threat ID",
		"Category", "Severity", "Direction", "Sequence Number", "Action Flags", "Source Location",
		"Destination Location", "FUTURE_USE", "Content Type", "PCAP_ID", "File Digest", "Cloud",
		"URL Index", "User Agent", "File Type", "X-Forwarded-For", "Referer", "Sender", "Subject",
		"Recipient", "Report ID", "Device Group Hierarchy Level 1", "Device Group Hierarchy Level 2",
		"Device Group Hierarchy Level 3", "Device Group Hierarchy Level 4", "Virtual System Name",
		"Device Name", "FUTURE_USE", "Source VM UUID", "Destination VM UUID", "HTTP Method",
		"Tunnel ID/IMSI", "Monitor Tag/IMEI", "Parent Session ID", "Parent Start Time", "Tunnel Type",
		"Threat Category", "Content Version", "FUTURE_USE", "SCTP Association ID", "Payload Protocol ID",
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
		"Application SaaS", "Application Sanctioned State", "Cloud Report ID",
	},
	// SYSTEM 11.0 = 26 (matches published count exactly).
	"SYSTEM": {
		"FUTURE_USE", "Receive Time", "Serial Number", "Type", "Content/Threat Type", "FUTURE_USE",
		"Generated Time", "Virtual System", "Event ID", "Object", "FUTURE_USE", "FUTURE_USE",
		"Module", "Severity", "Description", "Sequence Number", "Action Flags",
		"Device Group Hierarchy Level 1", "Device Group Hierarchy Level 2",
		"Device Group Hierarchy Level 3", "Device Group Hierarchy Level 4", "Virtual System Name",
		"Device Name", "FUTURE_USE", "FUTURE_USE", "High Resolution Timestamp",
	},
	// CONFIG 11.0 = 28 (matches published count exactly).
	"CONFIG": {
		"FUTURE_USE", "Receive Time", "Serial Number", "Type", "Subtype", "FUTURE_USE",
		"Generated Time", "Host", "Virtual System", "Command", "Admin", "Client", "Result",
		"Configuration Path", "Before Change Detail", "After Change Detail", "Sequence Number",
		"Action Flags", "Device Group Hierarchy Level 1", "Device Group Hierarchy Level 2",
		"Device Group Hierarchy Level 3", "Device Group Hierarchy Level 4", "Virtual System Name",
		"Device Name", "Device Group", "Audit Comment", "FUTURE_USE", "High Resolution Timestamp",
	},
	// AUTHENTICATION 11.0 = 47 (matches published count exactly).
	"AUTHENTICATION": {
		"FUTURE_USE", "Receive Time", "Serial Number", "Type", "Threat/Content Type", "FUTURE_USE",
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
	// CORRELATION 11.0 = 22 (matches published count exactly).
	"CORRELATION": {
		"FUTURE_USE", "Receive Time", "Serial Number", "Type", "Content/Threat Type", "FUTURE_USE",
		"Generated Time", "Source Address", "Source User", "Virtual System", "Category", "Severity",
		"Device Group Hierarchy Level 1", "Device Group Hierarchy Level 2",
		"Device Group Hierarchy Level 3", "Device Group Hierarchy Level 4", "Virtual System Name",
		"Device Name", "Virtual System ID", "Object Name", "Object ID", "Evidence",
	},
	// DECRYPTION 11.0 = 106. From the rendered spec format string (headless
	// Chrome); full page = 107, excluded post-11.0 (marked on the page): Cluster
	// Name (11.1+). Kept through Application Sanctioned State.
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
		"Application Container", "Application SaaS", "Application Sanctioned State",
	},
	// GLOBALPROTECT 11.0 = 50 (matches published count exactly).
	"GLOBALPROTECT": {
		"FUTURE_USE", "Receive Time", "Serial Number", "Type", "Threat/Content Type", "FUTURE_USE",
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
	// GTP 11.0 = 94. From the rendered spec format string (headless Chrome); no
	// fields on the page are marked post-11.0.
	"GTP": {
		"FUTURE_USE", "Receive Time", "Serial Number", "Type", "Threat/Content Type", "FUTURE_USE",
		"Generated Time", "Source Address", "Destination Address", "FUTURE_USE", "FUTURE_USE",
		"Rule Name", "FUTURE_USE", "FUTURE_USE", "Application", "Virtual System", "Source Zone",
		"Destination Zone", "Inbound Interface", "Outbound Interface", "Log Action", "FUTURE_USE",
		"Session ID", "FUTURE_USE", "Source Port", "Destination Port", "FUTURE_USE", "FUTURE_USE",
		"FUTURE_USE", "Protocol", "Action", "GTP Event Type", "MSISDN", "Access Point Name",
		"Radio Access Technology", "GTP Message Type", "End User IP Address",
		"Tunnel Endpoint Identifier1", "Tunnel Endpoint Identifier2", "GTP Interface", "GTP Cause",
		"Severity", "Serving Country MCC", "Serving Network MNC", "Area Code", "Cell ID",
		"GTP Event Code", "FUTURE_USE", "FUTURE_USE", "Source Location", "Destination Location",
		"FUTURE_USE", "FUTURE_USE", "FUTURE_USE", "FUTURE_USE", "FUTURE_USE", "FUTURE_USE", "FUTURE_USE",
		"Tunnel ID/IMSI", "Monitor Tag/IMEI", "FUTURE_USE", "FUTURE_USE", "FUTURE_USE", "FUTURE_USE",
		"FUTURE_USE", "FUTURE_USE", "FUTURE_USE", "FUTURE_USE", "FUTURE_USE", "FUTURE_USE", "FUTURE_USE",
		"FUTURE_USE", "FUTURE_USE", "FUTURE_USE", "FUTURE_USE", "FUTURE_USE", "Start Time",
		"Elapsed Time", "Tunnel Inspection Rule", "Remote User IP", "Remote User ID", "UUID for rule",
		"PCAP ID", "High Resolution Timestamp", "A Slice Service Type", "A Slice Differentiator",
		"Application Subcategory", "Application Category", "Application Technology", "Application Risk",
		"Application Characteristic", "Application Container", "Application SaaS",
		"Application Sanctioned State",
	},
	// HIP-MATCH 11.0 = 32 (matches published count exactly).
	"HIP-MATCH": {
		"FUTURE_USE", "Receive Time", "Serial Number", "Type", "Threat/Content Type", "FUTURE_USE",
		"Generated Time", "Source User", "Virtual System", "Machine Name", "Operating System",
		"Source Address", "HIP", "Repeat Count", "HIP Type", "FUTURE_USE", "FUTURE_USE",
		"Sequence Number", "Action Flags", "Device Group Hierarchy Level 1",
		"Device Group Hierarchy Level 2", "Device Group Hierarchy Level 3",
		"Device Group Hierarchy Level 4", "Virtual System Name", "Device Name", "Virtual System ID",
		"IPv6 Source Address", "Host ID", "User Device Serial Number", "Device MAC Address",
		"High Resolution Timestamp", "Cluster Name",
	},
	// IPTAG 11.0 = 27 (matches published count exactly).
	"IPTAG": {
		"FUTURE_USE", "Receive Time", "Serial", "Type", "Threat/Content Type", "FUTURE_USE",
		"Generate Time", "Virtual System", "Source IP", "Tag Name", "Event ID", "Repeat Count",
		"Timeout", "Data Source Name", "Data Source Type", "Data Source Subtype", "Sequence Number",
		"Action Flags", "DG Hierarchy Level 1", "DG Hierarchy Level 2", "DG Hierarchy Level 3",
		"DG Hierarchy Level 4", "Virtual System Name", "Device Name", "Virtual System ID",
		"High Resolution Timestamp", "Cluster Name",
	},
	// SCTP 11.0 = 65 (matches published count exactly).
	"SCTP": {
		"FUTURE_USE", "Receive Time", "Serial Number", "Type", "FUTURE_USE", "FUTURE_USE",
		"Generated Time", "Source Address", "Destination Address", "FUTURE_USE", "FUTURE_USE",
		"Rule Name", "FUTURE_USE", "FUTURE_USE", "FUTURE_USE", "Virtual System", "Source Zone",
		"Destination Zone", "Inbound Interface", "Outbound Interface", "Log Action", "FUTURE_USE",
		"Session ID", "Repeat Count", "Source Port", "Destination Port", "FUTURE_USE", "FUTURE_USE",
		"FUTURE_USE", "FUTURE_USE", "IP Protocol", "Action", "Device Group Hierarchy Level 1",
		"Device Group Hierarchy Level 2", "Device Group Hierarchy Level 3",
		"Device Group Hierarchy Level 4", "Virtual System Name", "Device Name", "Sequence Number",
		"FUTURE_USE", "SCTP Association ID", "Payload Protocol ID", "Severity", "SCTP Chunk Type",
		"FUTURE_USE", "SCTP Verification Tag 1", "SCTP Verification Tag 2", "SCTP Cause Code",
		"Diameter App ID", "Diameter Command Code", "Diameter AVP Code", "SCTP Stream ID",
		"SCTP Association End Reason", "Op Code", "SCCP Calling Party SSN",
		"SCCP Calling Party Global Title", "SCTP Filter", "SCTP Chunks", "SCTP Chunks Sent",
		"SCTP Chunks Received", "Packets", "Packets Sent", "Packets Received", "UUID for rule",
		"High Resolution Timestamp",
	},
	// USERID 11.0 = 37 (matches published count exactly).
	"USERID": {
		"FUTURE_USE", "Receive Time", "Serial Number", "Type", "Threat/Content Type", "FUTURE_USE",
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
// stable order.
var logTypeCatalog = []string{
	"TRAFFIC", "THREAT", "SYSTEM", "CONFIG", "AUTHENTICATION", "CORRELATION",
	"DECRYPTION", "GLOBALPROTECT", "GTP", "HIP-MATCH", "IPTAG", "SCTP", "USERID",
}

// recordCtx holds per-record values so fields that reference the same entity
// (e.g. the two timestamps, the 5-tuple) stay internally consistent.
type recordCtx struct {
	logType string
	subtype string
	now     time.Time
	src     string
	dst     string
	srcPort string
	dstPort string
	session string
	serial  string
	proto   string
	action  string
	user    string
	app     string
}

func newCtx(logType string) *recordCtx {
	return &recordCtx{
		logType: logType,
		subtype: subtypeFor(logType),
		now:     time.Now(),
		src:     generateRandomIP(),
		dst:     generateRandomIP(),
		srcPort: generateRandomPort(),
		dstPort: generateRandomPort(),
		session: generateNumericSessionID(),
		serial:  generateSerial(),
		proto:   pick("tcp", "udp"),
		action:  pick("allow", "deny", "drop", "alert", "reset-both"),
		user:    pick("corp\\jdoe", "corp\\asmith", ""),
		app:     pick("web-browsing", "ssl", "dns", "ssh", "smtp"),
	}
}

// buildLogFields returns the ordered CSV field slice for a log type, one value
// per spec field in logFieldNames order. Field 0 is FUTURE_USE ("1"). Values
// are realistic where PAN-OS populates the field, and empty for FUTURE_USE and
// for feature-gated fields that a firewall genuinely emits empty when the
// feature is inactive (Device-ID, EDL/DAG, SD-WAN, SCTP-on-non-SCTP, etc.).
func buildLogFields(logType string) []string {
	names, ok := logFieldNames[logType]
	if !ok {
		names = logFieldNames["SYSTEM"]
		logType = "SYSTEM"
	}
	c := newCtx(logType)
	out := make([]string, len(names))
	dgh := 0 // rotate device-group-hierarchy fill so each of the 4 gets "0"
	for i, name := range names {
		out[i] = c.valueFor(i, name, &dgh)
	}
	return out
}

// valueFor maps a spec field name to a realistic value. The first field
// (position 0) is always FUTURE_USE="1", matching real PAN-OS output.
func (c *recordCtx) valueFor(pos int, name string, dgh *int) string {
	if pos == 0 {
		return "1"
	}
	switch name {
	case "FUTURE_USE":
		return ""
	case "Receive Time", "Generated Time", "Generate Time", "Time Logged":
		return c.now.Format("2006/01/02 15:04:05")
	case "Start Time", "Parent Start Time", "Parent Session Start Time", "Factor Completion Time":
		return c.now.Add(-time.Duration(randInt(1, 60)) * time.Second).Format("2006/01/02 15:04:05")
	case "High Resolution Timestamp", "High Res Timestamp":
		return c.now.Format("2006-01-02T15:04:05.000-07:00")
	case "Serial Number", "Serial":
		return c.serial
	case "Type":
		return c.logType
	case "Threat/Content Type", "Content/Threat Type", "Subtype":
		return c.subtype
	case "Source Address", "Source IP":
		return c.src
	case "Destination Address":
		return c.dst
	case "IPv6 Source Address", "Public IPv6", "Private IPv6", "End User IP Address":
		return ""
	case "Rule Name", "Rule", "Tunnel Inspection Rule":
		return pick("allow-web", "trust-untrust", "block-bad", "default")
	case "UUID for rule", "Rule UUID":
		return generateRuleUUID()
	case "Source User", "User", "Remote User ID", "User by Source", "Normalize User":
		return c.user
	case "Destination User":
		return ""
	case "Application":
		return c.app
	case "Virtual System", "Virtual System Name":
		return "vsys1"
	case "Virtual System ID":
		return "1"
	case "Source Zone":
		return "trust"
	case "Destination Zone":
		return "untrust"
	case "Inbound Interface":
		return "ethernet1/1"
	case "Outbound Interface":
		return "ethernet1/2"
	case "Log Action":
		return "log-forwarding-default"
	case "Session ID":
		return c.session
	case "Repeat Count":
		return "1"
	case "Source Port":
		return c.srcPort
	case "Destination Port":
		return c.dstPort
	case "NAT Source Port", "NAT Destination Port":
		return "0"
	case "NAT Source IP", "NAT Destination IP":
		return "0.0.0.0"
	case "Flags":
		return "0x0"
	case "IP Protocol", "Protocol":
		return c.proto
	case "Action":
		return c.action
	case "Bytes":
		return strconv.Itoa(randInt(200, 200000))
	case "Bytes Sent":
		return strconv.Itoa(randInt(100, 100000))
	case "Bytes Received":
		return strconv.Itoa(randInt(100, 100000))
	case "Packets":
		return strconv.Itoa(randInt(2, 800))
	case "Packets Sent", "Packets Received":
		return strconv.Itoa(randInt(1, 400))
	case "Elapsed Time", "Login Duration", "Response Time", "Time Out Threshold", "Timeout":
		return strconv.Itoa(randInt(0, 3600))
	case "Category", "Threat Category":
		return pick("any", "computer-and-internet-info", "malware", "phishing")
	case "Sequence Number":
		return strconv.Itoa(randInt(100000, 99999999))
	case "Action Flags":
		return "0x0"
	case "Source Location", "Source Country", "Source Region", "Serving Country MCC":
		return pick("US", "GB", "DE", "10.0.0.0-10.255.255.255")
	case "Destination Location", "Destination Country":
		return pick("US", "CN", "RU")
	case "Session End Reason", "SCTP Association End Reason":
		return pick("tcp-rst-from-client", "tcp-fin", "aged-out", "policy-deny")
	case "Device Name":
		return "PA-VM"
	case "Device Group Hierarchy Level 1", "Device Group Hierarchy Level 2",
		"Device Group Hierarchy Level 3", "Device Group Hierarchy Level 4",
		"DG Hierarchy Level 1", "DG Hierarchy Level 2", "DG Hierarchy Level 3", "DG Hierarchy Level 4":
		*dgh++
		return "0"
	case "Severity":
		return pick("critical", "high", "medium", "low", "informational")
	case "Direction":
		return pick("client-to-server", "server-to-client")
	case "Action Source":
		return pick("from-policy", "from-application")
	case "URL/Filename":
		return pick("http://example.com/x", "evil.example.net/mal.exe", "phish.test/login")
	case "Threat ID":
		return strconv.Itoa(randInt(10000, 99999))
	case "Content Type":
		return pick("", "text/html", "application/pdf")
	case "Content Version":
		return "AppThreat-" + strconv.Itoa(randInt(8000, 8999)) + "-" + strconv.Itoa(randInt(1000, 9999))
	case "TLS Version":
		return pick("TLS1.2", "TLS1.3")
	case "Key Exchange Algorithm":
		return pick("ECDHE", "RSA")
	case "Encryption Algorithm":
		return pick("AES-256-GCM", "AES-128-GCM")
	case "Hash Algorithm":
		return pick("SHA384", "SHA256")
	case "Event ID", "GTP Event Code":
		return strconv.Itoa(randInt(1, 9999))
	case "Object", "Object Name":
		return pick("", "general", "rule1")
	case "Object ID":
		return strconv.Itoa(randInt(1000, 9999))
	case "Module":
		return c.subtype
	case "Description", "Evidence", "Reason":
		return pick("event recorded", "policy matched", "threshold exceeded")
	case "Command":
		return pick("commit", "edit", "set", "delete")
	case "Admin":
		return pick("admin", "operator", "automation")
	case "Client", "Client Type":
		return pick("Web", "CLI", "API")
	case "Result", "Status":
		return pick("Submitted", "Succeeded", "success", "failure")
	case "Host":
		return c.src
	case "Machine Name":
		return "LAPTOP-" + strconv.Itoa(randInt(1000, 9999))
	case "Operating System", "Client OS":
		return pick("Windows 10", "Windows 11", "macOS 14")
	case "MSISDN":
		return strconv.FormatInt(randInt64(100000000000000, 999999999999999), 10)
	case "Access Point Name":
		return pick("internet.apn", "ims.apn")
	case "Radio Access Technology":
		return pick("EUTRAN", "UTRAN", "GERAN")
	case "GTP Event Type", "GTP Message Type":
		return pick("create-session", "delete-session", "update-bearer")
	case "Authentication Method", "Authentication Protocol":
		return pick("SAML", "LDAP", "RADIUS", "certificate")
	case "Vendor":
		return pick("PANW", "Okta", "Duo")
	case "Event Type":
		return pick("login", "logout")
	case "Tunnel Type":
		return pick("ipsec", "ssl", "GRE")
	case "SCTP Chunk Type":
		return pick("DATA", "INIT", "SACK", "HEARTBEAT")
	case "HIP":
		return pick("firewall-enabled", "disk-encrypted", "av-installed")
	case "HIP Type":
		return pick("object", "profile")
	case "Tag Name":
		return pick("malicious", "quarantine", "trusted")
	case "Data Source Name", "Data Source", "Origin Data Source":
		return pick("AD-agent", "syslog", "XML-API")
	case "Data Source Type":
		return pick("active-directory", "syslog")
	case "Data Source Subtype":
		return pick("unknown", "xml-api")
	default:
		// Feature-gated / optional fields a firewall emits empty when the
		// feature is inactive (Device-ID, EDL/DAG, SD-WAN, cert details,
		// SCTP-on-non-SCTP, container, etc.).
		return ""
	}
}

// subtypeFor returns a representative subtype (field 5) for a log type.
func subtypeFor(logType string) string {
	switch logType {
	case "TRAFFIC":
		return pick("start", "end", "drop", "deny")
	case "THREAT":
		return pick("url", "vulnerability", "spyware", "virus", "wildfire", "file")
	case "SYSTEM":
		return pick("general", "auth", "ha", "vpn", "routing")
	case "CONFIG":
		return "0"
	case "AUTHENTICATION":
		return "auth"
	case "CORRELATION":
		return pick("compromised-host", "recon")
	case "DECRYPTION":
		return pick("ssl", "ssh")
	case "GLOBALPROTECT":
		return pick("gateway", "portal")
	case "GTP":
		return pick("gtp-c", "gtp-u", "gtp-prime")
	case "HIP-MATCH":
		return "hip"
	case "IPTAG", "USERID":
		return "unknown"
	default:
		return ""
	}
}

// generatePaloAltoLog builds one PAN-OS syslog line: the BSD syslog timestamp
// header followed by the comma-separated CSV record for a randomly chosen log
// type. Every type in logTypeCatalog is emitted at its full PAN-OS 11.0 field
// width.
func generatePaloAltoLog() string {
	logType := logTypeCatalog[randInt(0, len(logTypeCatalog)-1)]
	return formatLogLine(logType)
}

// formatLogLine renders the syslog header plus the comma-joined CSV fields.
func formatLogLine(logType string) string {
	timestamp := time.Now().Format("Jan 02 15:04:05")
	return timestamp + " " + strings.Join(buildLogFields(logType), ",")
}

// ---- shared value helpers ----

// pick returns one of the provided options at random.
func pick(opts ...string) string {
	return opts[randInt(0, len(opts)-1)]
}

// generateSerial returns a PAN-OS-style 12-digit device serial number.
func generateSerial() string {
	var b strings.Builder
	for i := 0; i < 12; i++ {
		b.WriteString(strconv.Itoa(randInt(0, 9)))
	}
	return b.String()
}

// generateRuleUUID returns a random UUIDv4-shaped rule identifier.
func generateRuleUUID() string {
	return fmt.Sprintf("%08x-%04x-4%03x-%04x-%012x",
		randInt64(0, 0xffffffff), randInt(0, 0xffff), randInt(0, 0xfff),
		randInt(0x8000, 0xbfff), randInt64(0, 0xffffffffffff))
}
