package paloalto

import (
	"encoding/csv"
	"net"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
	_ "time/tzdata" // TestClockShiftWithin loads America/Detroit; embed the zone database for hosts without one

	"github.com/stretchr/testify/require"
)

// field returns the named field of a built record.
func field(logType string, f []string, name string) string {
	return f[indexOf(logFieldNames[logType], name)]
}

func atoi(t *testing.T, s string) int {
	t.Helper()
	n, err := strconv.Atoi(s)
	require.NoError(t, err, "%q is not a number", s)
	return n
}

// clockShiftWithin is how far, in seconds, now's zone moved its UTC offset in
// the last d (0 outside a DST change). Start and generated times are written
// as local wall-clock times, so across a fall-back a record's lag reads off by
// exactly that much.
func clockShiftWithin(now time.Time, d time.Duration) int {
	_, off := now.Zone()
	_, before := now.Add(-d).Zone()
	if off < before {
		return before - off
	}
	return off - before
}

func TestClockShiftWithin(t *testing.T) {
	detroit, err := time.LoadLocation("America/Detroit")
	require.NoError(t, err)
	fallBack := time.Date(2026, 11, 1, 1, 30, 0, 0, detroit).Add(time.Hour) // 01:30 EST, after the shift
	require.Equal(t, 3600, clockShiftWithin(fallBack, 24*time.Hour))
	require.Zero(t, clockShiftWithin(time.Date(2026, 10, 14, 12, 0, 0, 0, detroit), 24*time.Hour))
	require.Zero(t, clockShiftWithin(time.Date(2026, 11, 1, 1, 30, 0, 0, time.UTC), 24*time.Hour))
}

// TestTrafficRecordIsCoherent pins a TRAFFIC record to one session as the
// device logs it: Bytes and Packets are the two directions' totals; start
// sessions are allowed with end reason n/a; end sessions are allowed and end
// on a TCP reason, aged-out, or threat; a deny session is allowed by its rule
// and ends on policy-deny, or on auth-policy-redirect for web-browsing; a drop
// is denied or dropped with no session or egress interface; and the start
// time precedes the generated time by the elapsed time plus the lag the
// capture shows for the subtype.
func TestTrafficRecordIsCoherent(t *testing.T) {
	shift := clockShiftWithin(time.Now(), 24*time.Hour)
	tcpReasons := []string{"tcp-fin", "tcp-rst-from-client", "tcp-rst-from-server", "aged-out", "threat"}
	lags := map[string][2]int{"start": {0, 7}, "end": {3, 94}, "deny": {0, 4}, "drop": {1, 4}}
	for i := 0; i < 500; i++ {
		f := buildLogFields("TRAFFIC", fallbackDevice())
		get := func(name string) string { return field("TRAFFIC", f, name) }
		require.Equal(t, atoi(t, get("Bytes Sent"))+atoi(t, get("Bytes Received")), atoi(t, get("Bytes")), "bytes")
		require.Equal(t, atoi(t, get("Packets Sent"))+atoi(t, get("Packets Received")), atoi(t, get("Packets")), "packets")

		sub, action, reason, proto := get("Threat/Content Type"), get("Action"), get("Session End Reason"), get("Protocol")
		switch sub {
		case "start":
			require.Equal(t, "allow", action)
			require.Equal(t, "n/a", reason)
		case "end":
			require.Equal(t, "allow", action)
			if proto == "tcp" {
				require.Contains(t, tcpReasons, reason)
			} else {
				require.Contains(t, []string{"aged-out", "threat"}, reason)
			}
		case "deny":
			require.Equal(t, "allow", action)
			if get("Application") == "web-browsing" {
				require.Contains(t, []string{"policy-deny", "auth-policy-redirect"}, reason)
			} else {
				require.Equal(t, "policy-deny", reason)
			}
		case "drop":
			require.Contains(t, []string{"deny", "drop"}, action)
			require.Equal(t, "policy-deny", reason)
			require.Equal(t, "0", get("Session ID"))
			require.Empty(t, get("Outbound Interface"))
		default:
			t.Fatalf("unknown subtype %q", sub)
		}

		gen, err := time.ParseInLocation("2006/01/02 15:04:05", get("Generated Time"), time.Local)
		require.NoError(t, err)
		start, err := time.ParseInLocation("2006/01/02 15:04:05", get("Start Time"), time.Local)
		require.NoError(t, err)
		lag := int(gen.Sub(start)/time.Second) - atoi(t, get("Elapsed Time"))
		in := func(l int) bool { return l >= lags[sub][0] && l <= lags[sub][1] }
		require.True(t, in(lag) || (shift > 0 && (in(lag+shift) || in(lag-shift))), "%s start lag %d", sub, lag)
		require.Equal(t, "N/A", get("Tunnel Type"), "no tunnel")
	}
}

// TestFieldValuesMatchTheirSpec pins values the device or the 11.0 field
// pages define: a GTP-C session's Serving Network MCC and MNC are a numeric
// MCC and MNC, while a tunnel session's GTP fields are zero; DECRYPTION's
// subtype is 0; and an SCTP association ends on one of its own reasons with
// Packets the two directions' total.
func TestFieldValuesMatchTheirSpec(t *testing.T) {
	for i := 0; i < 200; i++ {
		g := buildLogFields("GTP", fallbackDevice())
		mcc, mnc := field("GTP", g, "Serving Country MCC"), field("GTP", g, "Serving Network MNC")
		if field("GTP", g, "Application") == "gtpv1-c" {
			require.Len(t, mcc, 3, "MCC %q", mcc)
			atoi(t, mcc)
			require.True(t, len(mnc) == 2 || len(mnc) == 3, "MNC %q", mnc)
			atoi(t, mnc)
		} else {
			require.Equal(t, "0", mcc)
			require.Equal(t, "0", mnc)
		}

		d := buildLogFields("DECRYPTION", fallbackDevice())
		require.Equal(t, "0", field("DECRYPTION", d, "Threat/Content Type"))

		s := buildLogFields("SCTP", fallbackDevice())
		require.Contains(t, []string{"n/a", "shutdown-from-endpoint"}, field("SCTP", s, "SCTP Association End Reason"))
		require.Equal(t, atoi(t, field("SCTP", s, "Packets Sent"))+atoi(t, field("SCTP", s, "Packets Received")), atoi(t, field("SCTP", s, "Packets")))
	}
}

// TestDeviceCountersAndRules pins per-device state the 11.0 pages define:
// Sequence Number "incremented sequentially; each log type has a unique
// number space", and Rule UUID "permanently identifies the rule".
func TestDeviceCountersAndRules(t *testing.T) {
	dev := fallbackDevice()
	prev := map[string]int{}
	uuids := map[string]string{}
	for i := 0; i < 50; i++ {
		for _, lt := range []string{"TRAFFIC", "THREAT", "SYSTEM"} {
			f := buildLogFields(lt, dev)
			seq := atoi(t, field(lt, f, "Sequence Number"))
			if p, ok := prev[lt]; ok {
				require.Equal(t, p+1, seq, "%s sequence", lt)
			}
			prev[lt] = seq
			if lt != "SYSTEM" {
				rule, uuid := field(lt, f, "Rule Name"), field(lt, f, "Rule UUID")
				if u, ok := uuids[rule]; ok {
					require.Equal(t, u, uuid, "rule %s keeps its UUID", rule)
				}
				uuids[rule] = uuid
			}
		}
	}
}

// TestEventFieldsMatchTheSpec pins event and result fields to the device: a
// SYSTEM event is a captured (subtype, Event ID, severity) row with Module
// general; a GlobalProtect event names its stage and a success or failure
// status; CONFIG Result, Client, and Command use captured values; User-ID logs
// logins and logouts; the IP-Tag subtype is unused and its events register or
// unregister a tag.
func TestEventFieldsMatchTheSpec(t *testing.T) {
	dev := fallbackDevice()
	for i := 0; i < 300; i++ {
		s := buildLogFields("SYSTEM", dev)
		ev := systemEvent{subtype: field("SYSTEM", s, "Content/Threat Type"), id: field("SYSTEM", s, "Event ID"), severity: field("SYSTEM", s, "Severity")}
		found := false
		for _, e := range systemEvents {
			if e.subtype == ev.subtype && e.id == ev.id && e.severity == ev.severity {
				found = true
			}
		}
		require.True(t, found, "unknown SYSTEM event %+v", ev)
		require.Equal(t, "general", field("SYSTEM", s, "Module"))

		gp := buildLogFields("GLOBALPROTECT", dev)
		require.Equal(t, gpEventStages[field("GLOBALPROTECT", gp, "Event ID")], field("GLOBALPROTECT", gp, "Stage"))
		require.NotEmpty(t, field("GLOBALPROTECT", gp, "Stage"))
		require.Contains(t, []string{"success", "failure"}, field("GLOBALPROTECT", gp, "Status"))

		c := buildLogFields("CONFIG", dev)
		require.Contains(t, []string{"Submitted", "Succeeded", "Failed"}, field("CONFIG", c, "Result"))
		require.Contains(t, []string{"Web", "CLI"}, field("CONFIG", c, "Client"))
		require.Contains(t, []string{"commit", "delete", "move", "request", "set"}, field("CONFIG", c, "Command"))
		if field("CONFIG", c, "Command") == "commit" {
			require.Empty(t, field("CONFIG", c, "Configuration Path"))
		}

		u := buildLogFields("USERID", dev)
		require.Contains(t, []string{"login", "logout"}, field("USERID", u, "Threat/Content Type"))

		ip := buildLogFields("IPTAG", dev)
		require.Equal(t, "0", field("IPTAG", ip, "Threat/Content Type"))
		require.Contains(t, []string{"register", "unregister"}, field("IPTAG", ip, "Event ID"))
	}
}

// TestCommaFieldsAreQuoted pins PAN-OS CSV quoting: a description with commas
// is one quoted field, so the record still parses to its full width.
func TestCommaFieldsAreQuoted(t *testing.T) {
	for i := 0; i < 500; i++ {
		line := formatLogLine("SYSTEM", fallbackDevice())
		rec, err := csv.NewReader(strings.NewReader(line[strings.Index(line, " 1,")+1:])).Read()
		require.NoError(t, err)
		require.Len(t, rec, wantFieldCount["SYSTEM"])
		if strings.Contains(field("SYSTEM", rec, "Description"), ",") {
			return
		}
	}
	t.Fatal("no SYSTEM description with a comma in 500 records")
}

// TestAddressesAreRoutable pins generated addresses to real traffic: RFC 1918
// private or public unicast, never loopback, multicast, Class E, CGNAT, or
// documentation space.
func TestAddressesAreRoutable(t *testing.T) {
	reserved := []string{"0.0.0.0/8", "100.64.0.0/10", "127.0.0.0/8", "169.254.0.0/16", "192.0.2.0/24", "198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24", "224.0.0.0/4", "240.0.0.0/4"}
	for i := 0; i < 5000; i++ {
		ip := net.ParseIP(generateRandomIP())
		require.NotNil(t, ip)
		for _, c := range reserved {
			_, n, _ := net.ParseCIDR(c)
			require.False(t, n.Contains(ip), "%s is in reserved %s", ip, c)
		}
	}
}

// privateRegion is PAN-OS's predefined region for an RFC 1918 address, which
// it writes in place of a country.
func privateRegion(ip net.IP) string {
	for _, r := range []struct{ cidr, name string }{
		{"10.0.0.0/8", "10.0.0.0-10.255.255.255"},
		{"172.16.0.0/12", "172.16.0.0-172.31.255.255"},
		{"192.168.0.0/16", "192.168.0.0-192.168.255.255"},
	} {
		if _, n, _ := net.ParseCIDR(r.cidr); n.Contains(ip) {
			return r.name
		}
	}
	return ""
}

// TestLocationFollowsAddress pins that a location field describes its record's
// address, as the device writes it: a private address gets its predefined
// region range, a public one a country name. GlobalProtect's Source Region
// describes the client's public address the same way.
func TestLocationFollowsAddress(t *testing.T) {
	for _, tc := range []struct{ logType, srcAddr, dstAddr, src, dst string }{
		{"TRAFFIC", "Source Address", "Destination Address", "Source Country", "Destination Country"},
		{"THREAT", "Source Address", "Destination Address", "Source Location", "Destination Location"},
		{"GLOBALPROTECT", "Public IP", "Public IP", "Source Region", "Source Region"},
	} {
		for i := 0; i < 300; i++ {
			f := buildLogFields(tc.logType, fallbackDevice())
			for addr, loc := range map[string]string{tc.srcAddr: tc.src, tc.dstAddr: tc.dst} {
				ip, got := net.ParseIP(field(tc.logType, f, addr)), field(tc.logType, f, loc)
				if want := privateRegion(ip); want != "" {
					require.Equal(t, want, got, "%s %s %s", tc.logType, loc, ip)
					continue
				}
				require.NotContains(t, got, "-", "%s %s %s: a public address has a country, not a range", tc.logType, loc, ip)
				require.Greater(t, len(got), 2, "%s %s %s: a country name, not a code", tc.logType, loc, ip)
			}
		}
	}
}

// TestApplicationMatchesProtocolAndPort pins that a session's App-ID, IP
// protocol, and destination port describe one service, that its App-ID
// metadata is the device's for that App-ID (TRAFFIC from the capture, ssh and
// smtp-base from the firewall's predefined application database), and that the
// client connects from an ephemeral port. A dropped session has no App-ID
// (not-applicable).
func TestApplicationMatchesProtocolAndPort(t *testing.T) {
	services := map[string]struct{ proto, port, meta string }{
		"web-browsing": {"tcp", "80", "internet-utility|general-internet|browser-based|4|used-by-malware,able-to-transfer-file,has-known-vulnerability,tunnel-other-application,pervasive-use|"},
		"ssl":          {"tcp", "443", "encrypted-tunnel|networking|browser-based|4|used-by-malware,able-to-transfer-file,has-known-vulnerability,tunnel-other-application,pervasive-use|"},
		"google-base":  {"tcp", "443", "internet-utility|general-internet|browser-based|4|used-by-malware,able-to-transfer-file,has-known-vulnerability,tunnel-other-application,pervasive-use|"},
		"dns-base":     {"udp", "53", "infrastructure|networking|network-protocol|3|used-by-malware,has-known-vulnerability,pervasive-use|dns"},
		"ssh":          {"tcp", "22", "encrypted-tunnel|networking|client-server|4|used-by-malware,able-to-transfer-file,has-known-vulnerability,tunnel-other-application,pervasive-use|"},
		"smtp-base":    {"tcp", "25", "email|collaboration|client-server|5|used-by-malware,able-to-transfer-file,has-known-vulnerability,tunnel-other-application,prone-to-misuse,pervasive-use|smtp"},
	}
	for _, tc := range []struct{ logType, proto string }{{"TRAFFIC", "Protocol"}, {"THREAT", "IP Protocol"}} {
		for i := 0; i < 300; i++ {
			f := buildLogFields(tc.logType, fallbackDevice())
			get := func(n string) string { return field(tc.logType, f, n) }
			app := get("Application")
			if app == "not-applicable" {
				require.Equal(t, "drop", get("Threat/Content Type"))
				continue
			}
			want, ok := services[app]
			require.True(t, ok, "unknown application %q", app)
			require.Equal(t, want.proto, get(tc.proto), "%s %s protocol", tc.logType, app)
			require.Equal(t, want.port, get("Destination Port"), "%s %s port", tc.logType, app)
			meta := strings.Join([]string{get("Application Subcategory"), get("Application Category"), get("Application Technology"),
				get("Application Risk"), get("Application Characteristic"), get("Application Container")}, "|")
			require.Equal(t, want.meta, meta, "%s %s metadata", tc.logType, app)
			require.Equal(t, app, get("Tunneled Application"))
			require.GreaterOrEqual(t, atoi(t, get("Source Port")), 1024, "a client port")
		}
	}
}

// TestNATAndInterfacesFollowTheSession pins the device's source NAT and
// interfaces: a session to a public address leaves through the untrust
// interface with its source translated to that interface's address and its
// destination unchanged; a private destination is not translated (all four
// NAT fields zero); trust-to-untrust sessions arrive on the trust interface.
func TestNATAndInterfacesFollowTheSession(t *testing.T) {
	dev := fallbackDevice()
	for i := 0; i < 300; i++ {
		f := buildLogFields("TRAFFIC", dev)
		get := func(n string) string { return field("TRAFFIC", f, n) }
		require.Equal(t, "trust", get("Source Zone"))
		require.Equal(t, "untrust", get("Destination Zone"))
		require.Equal(t, "ethernet1/2", get("Inbound Interface"))
		if get("Threat/Content Type") != "drop" {
			require.Equal(t, "ethernet1/1", get("Outbound Interface"))
		}
		if privateRegion(net.ParseIP(get("Destination Address"))) != "" || get("Threat/Content Type") == "drop" {
			require.Equal(t, []string{"0.0.0.0", "0.0.0.0", "0", "0"}, []string{get("NAT Source IP"), get("NAT Destination IP"), get("NAT Source Port"), get("NAT Destination Port")})
			continue
		}
		require.Equal(t, dev.untrustIP, get("NAT Source IP"))
		require.Equal(t, get("Destination Address"), get("NAT Destination IP"))
		require.Equal(t, get("Destination Port"), get("NAT Destination Port"))
		require.NotEqual(t, "0", get("NAT Source Port"))
	}
}

// TestHeaderTimeIsTheRecordTime pins that the syslog header's timestamp is the
// record's Generated Time, as the device sends it, not a second clock read
// that can cross a second boundary.
func TestHeaderTimeIsTheRecordTime(t *testing.T) {
	orig := clock
	t.Cleanup(func() { clock = orig })
	t0 := time.Date(2026, 9, 30, 16, 25, 12, 0, time.Local)
	calls := 0
	fclock := func() time.Time {
		calls++
		return t0.Add(time.Duration(calls) * time.Second) // every read is a second later
	}
	clock = fclock

	line := formatLogLine("TRAFFIC", fallbackDevice())
	header, err := time.ParseInLocation("Jan _2 15:04:05", line[:15], time.Local)
	require.NoError(t, err)
	rec, err := csv.NewReader(strings.NewReader(line[strings.Index(line, " 1,")+1:])).Read()
	require.NoError(t, err)
	gen, err := time.ParseInLocation("2006/01/02 15:04:05", field("TRAFFIC", rec, "Generated Time"), time.Local)
	require.NoError(t, err)
	require.Equal(t, gen.Format("15:04:05"), header.Format("15:04:05"))
}

// TestSCTPRecordsAreSCTP pins that an SCTP association log carries the SCTP IP
// protocol and an SCTP service port (Diameter, M3UA, or S1AP), not a TCP or
// UDP App-ID's.
func TestSCTPRecordsAreSCTP(t *testing.T) {
	for i := 0; i < 200; i++ {
		f := buildLogFields("SCTP", fallbackDevice())
		require.Equal(t, "sctp", field("SCTP", f, "IP Protocol"))
		require.Contains(t, []string{"3868", "2905", "36412"}, field("SCTP", f, "Destination Port"))
	}
}

// TestThreatFieldsFollowTheThreatPage pins THREAT values to the device: PCAP
// ID is 0 without a pcap; Threat ID is a name followed by its numeric ID in
// parentheses, in the subtype's range (9999 for url, 52000-52999 filetype,
// generic DNS signatures for spyware); DNS spyware and data have Category
// "any" (TestThreatDetectionsWereCaptured pins the rest); a URL is logged without a
// scheme; HTTP/2 Connection is 0; Application SaaS and Sanctioned State are
// "no".
func TestThreatFieldsFollowTheThreatPage(t *testing.T) {
	idRe := regexp.MustCompile(`^(.*)\((\d+)\)$`)
	ranges := map[string][2]int{"url": {9999, 9999}, "file": {52000, 52999}}
	for i := 0; i < 300; i++ {
		f := buildLogFields("THREAT", fallbackDevice())
		get := func(n string) string { return field("THREAT", f, n) }
		sub := get("Threat/Content Type")
		require.Equal(t, "0", get("PCAP_ID"), sub)

		m := idRe.FindStringSubmatch(get("Threat ID"))
		require.NotNil(t, m, "%s Threat ID %q is name(id)", sub, get("Threat ID"))
		if r, ok := ranges[sub]; ok {
			id := atoi(t, m[2])
			require.True(t, id >= r[0] && id <= r[1], "%s Threat ID %d outside %v", sub, id, r)
		}
		switch sub {
		case "spyware":
			require.True(t, strings.HasPrefix(m[1], "generic:"+get("URL/Filename")), "spyware names its domain: %q", m[1])
			require.Equal(t, "any", get("Category"))
		case "data":
			require.Equal(t, "any", get("Category"))
		}
		require.NotContains(t, get("URL/Filename"), "://", sub)
		require.Equal(t, "0", get("HTTP/2 Connection"))
		require.Equal(t, "no", get("Application SaaS"))
		require.Equal(t, "no", get("Application Sanctioned State"))
	}
}

// TestCorrelationRecordsMatchTheirObject pins a CORRELATION record to the one
// correlation object it reports, as a device logged it (the Splunk add-on's
// issue 8 sample): Beacon Detection, ID 6005, in the compromised-host
// category (every correlation object's, per the Correlation Objects page) at
// medium severity, with the match count in its evidence. The device left the
// Content/Threat Type empty.
func TestCorrelationRecordsMatchTheirObject(t *testing.T) {
	evidence := regexp.MustCompile(`^Host visited known malware URL \((\d+) times\)\.$`)
	for i := 0; i < 300; i++ {
		f := buildLogFields("CORRELATION", fallbackDevice())
		get := func(n string) string { return field("CORRELATION", f, n) }
		require.Empty(t, get("Content/Threat Type"))
		require.Equal(t, "compromised-host", get("Category"))
		require.Equal(t, "Beacon Detection", get("Object Name"))
		require.Equal(t, "6005", get("Object ID"))
		require.Equal(t, "medium", get("Severity"))
		m := evidence.FindStringSubmatch(get("Evidence"))
		require.NotNil(t, m, "evidence %q", get("Evidence"))
		require.GreaterOrEqual(t, atoi(t, m[1]), 2, "medium is repeated visits")
	}
}

// TestUnknownLogTypeBuildsASystemRecord pins buildRecord's fallback: a log
// type it has no field list for builds a full SYSTEM record.
func TestUnknownLogTypeBuildsASystemRecord(t *testing.T) {
	_, f := buildRecord("NO-SUCH-TYPE", fallbackDevice())
	require.Len(t, f, len(logFieldNames["SYSTEM"]))
	require.Equal(t, "SYSTEM", field("SYSTEM", f, "Type"))
}

// TestGlobalProtectClientOSMatchesItsVersion pins that a record naming a
// Windows OS version also names Windows as the client OS.
func TestGlobalProtectClientOSMatchesItsVersion(t *testing.T) {
	dev := fallbackDevice()
	for range 500 {
		gp := buildLogFields("GLOBALPROTECT", dev)
		if strings.HasPrefix(field("GLOBALPROTECT", gp, "Client OS Version"), "Microsoft Windows") {
			require.Equal(t, "Windows", field("GLOBALPROTECT", gp, "Client OS"))
		}
	}
}

// TestVirusSignaturesMatchTheirFile pins that a virus detection names a real
// PAN-OS antivirus signature for its file's platform: a Windows .exe gets a
// Virus/Win32 signature, never a placeholder.
func TestVirusSignaturesMatchTheirFile(t *testing.T) {
	for _, s := range threatSigs["virus"] {
		require.NotContains(t, s.id, "example")
		if strings.HasSuffix(s.target, ".exe") {
			require.True(t, strings.HasPrefix(s.id, "Virus/Win32."), s.id)
		}
	}
}

// TestAdminAndPortalAddressesAreInternal pins the capture's addresses for the
// firewall's own users: admin CLI logins, auth successes, and CONFIG Host come
// from one internal admin host (10.250.0.20 in the capture), and a Captive
// Portal failure from an internal user, never an internet address.
func TestAdminAndPortalAddressesAreInternal(t *testing.T) {
	dev := fallbackDevice()
	addr := regexp.MustCompile(`(?:from|on|From:) (\d+\.\d+\.\d+\.\d+)`)
	admins := map[string]bool{}
	for range 3000 {
		s := buildLogFields("SYSTEM", dev)
		if m := addr.FindStringSubmatch(field("SYSTEM", s, "Description")); m != nil {
			require.True(t, net.ParseIP(m[1]).IsPrivate(), "%s", field("SYSTEM", s, "Description"))
			if !strings.HasPrefix(m[0], "on ") { // a Captive Portal user, not an admin
				admins[m[1]] = true
			}
		}
		c := buildLogFields("CONFIG", dev)
		require.True(t, net.ParseIP(field("CONFIG", c, "Host")).IsPrivate(), field("CONFIG", c, "Host"))
		admins[field("CONFIG", c, "Host")] = true
	}
	require.Len(t, admins, 1, "one admin host per firewall")
}

// TestRecordTimesFollowTheCapture pins two capture timings: a USERID factor
// completes in the second the record is written (its High Resolution
// Timestamp), and a CONFIG record, like a SYSTEM one, can be received up to a
// second before it is generated (110 of the capture's 246 CONFIG rows).
func TestRecordTimesFollowTheCapture(t *testing.T) {
	dev := fallbackDevice()
	for range 500 {
		u := buildLogFields("USERID", dev)
		hrt, err := time.Parse("2006-01-02T15:04:05.000-07:00", field("USERID", u, "High Resolution Timestamp"))
		require.NoError(t, err)
		require.Equal(t, hrt.Format("2006/01/02 15:04:05"), field("USERID", u, "Factor Completion Time"))
	}
	lags := map[time.Duration]bool{}
	for range 500 {
		c := buildLogFields("CONFIG", dev)
		gen, err := time.ParseInLocation("2006/01/02 15:04:05", field("CONFIG", c, "Generated Time"), time.Local)
		require.NoError(t, err)
		recv, err := time.ParseInLocation("2006/01/02 15:04:05", field("CONFIG", c, "Receive Time"), time.Local)
		require.NoError(t, err)
		lags[gen.Sub(recv)] = true
	}
	require.Equal(t, map[time.Duration]bool{0: true, time.Second: true}, lags)
}
