package paloalto

import (
	"bufio"
	"encoding/csv"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode"

	"github.com/stretchr/testify/require"
)

// captureFile is real syslog from a VM-Series firewall on PAN-OS 11.0.4-h6
// (BSD format), every log type but CORRELATION. Scrubbed: the admin's public
// address is 203.0.113.10, the cloud project is example-project, and the
// serial is replaced with one of the same shape.
const captureFile = "testdata/panos-11.0.4-h6.log"

var captureLineRe = regexp.MustCompile(`^<(\d+)>\w{3} [ \d]\d \d\d:\d\d:\d\d \S+ (.*)$`)

// capturedLine is one captured record: its syslog PRI and CSV fields.
type capturedLine struct {
	pri    int
	fields []string
}

// loadCapture returns the captured records by log type.
func loadCapture(t *testing.T) map[string][]capturedLine {
	t.Helper()
	f, err := os.Open(captureFile)
	require.NoError(t, err)
	defer f.Close()
	out := map[string][]capturedLine{}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		m := captureLineRe.FindStringSubmatch(sc.Text())
		require.NotNil(t, m, sc.Text())
		rec, err := csv.NewReader(strings.NewReader(m[2])).Read()
		require.NoError(t, err, sc.Text())
		pri, _ := strconv.Atoi(m[1])
		out[rec[3]] = append(out[rec[3]], capturedLine{pri, rec})
	}
	require.NoError(t, sc.Err())
	return out
}

// docOnly reports a generated record the capture has no example of, whose
// values come from the PAN-OS 11.0 field pages instead: THREAT virus and
// wildfire, and GTP-C sessions (GTP event logs).
func docOnly(logType string, f []string) bool {
	switch logType {
	case "THREAT":
		sub := field(logType, f, "Threat/Content Type")
		return sub == "virus" || sub == "wildfire"
	case "GTP":
		return field(logType, f, "Application") == "gtpv1-c"
	}
	return false
}

// shape abstracts a value to its character classes: digit runs become 9,
// lowercase runs a, uppercase runs A, and punctuation stays, so "10.250.2.20"
// and "172.217.119.4" share a shape and "0x400000" and "allow" do not.
func shape(v string) string {
	var b strings.Builder
	var last rune
	for _, r := range v {
		c := r
		switch {
		case unicode.IsDigit(r):
			c = '9'
		case unicode.IsLower(r):
			c = 'a'
		case unicode.IsUpper(r):
			c = 'A'
		}
		if c == last && (c == '9' || c == 'a' || c == 'A') {
			continue
		}
		b.WriteRune(c)
		last = c
	}
	return b.String()
}

// fieldPatterns replace the captured shapes for fields whose values belong to
// one deployment (names, IDs, free text) or vary in shape (hex, mixed case).
// Every captured value must still match its pattern.
var fieldPatterns = map[string]string{
	"Serial Number":                `^([0-9A-F]{15}|\d{12})?$`,
	"Serial":                       `^([0-9A-F]{15}|\d{12})$`,
	"Device Name":                  `^[\w.-]+$`,
	"Rule Name":                    `^[\w.-]+$`,
	"Rule":                         `^[\w.-]+$`,
	"Log Action":                   `^[\w.-]+$`,
	"Policy Name":                  `^[\w.-]+$`,
	"Tunnel Inspection Rule":       `^[\w.-]+$`,
	"Authentication Policy":        `^[\w.-]+$`,
	"Object":                       `^[\w./-]*$`,
	"Portal":                       `^[\w.-]*$`,
	"HIP":                          `^[\w.-]+$`,
	"Tag Name":                     `^[\w.-]*$`,
	"Machine Name":                 `^[\w.-]*$`,
	"Admin":                        `^\w+$`,
	"Source User":                  `^([\w.-]+\\)?[\w.-]*$`,
	"User":                         `^([\w.-]+\\)?[\w.-]+$`,
	"Normalize User":               `^([\w.-]+\\)?[\w.-]+$`,
	"User by Source":               `^([\w.-]+\\)?[\w.-]+$`,
	"Rule UUID":                    `^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`,
	"UUID for rule":                `^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`,
	"Host ID":                      `^[0-9a-z-]*$`,
	"Sequence Number":              `^\d{19}$`,
	"Authentication ID":            `^\d{19}$`,
	"Session ID":                   `^\d+$`,
	"Partial Hash":                 `^\d+$`,
	"Flags":                        `^0x[0-9a-f]+$`,
	"Action Flags":                 `^0x[0-9a-f]+$`,
	"User Group Flags":             `^0x[0-9a-f]+$`,
	"SCTP Verification Tag 1":      `^(0x[0-9a-f]+)?$`,
	"SCTP Verification Tag 2":      `^(0x[0-9a-f]+)?$`,
	"Certificate Serial Number":    `^[0-9a-f]+$`,
	"Fingerprint":                  `^[0-9a-f]{64}$`,
	"Certificate Flags":            `^(:+sni)?:+[A-Z]+$`,
	"Application":                  `^[a-z0-9-]*$`,
	"Tunneled Application":         `^[a-z0-9-]*$`,
	"Application Container":        `^[a-z0-9-]*$`,
	"Application Characteristic":   `^[a-z,-]*$`,
	"Application Subcategory":      `^[a-z-]*$`,
	"Category":                     `^[a-z-]+$`,
	"URL Category List":            `^[a-z,-]*$`,
	"Threat Category":              `^([a-z0-9-]+|N/A)$`,
	"Threat ID":                    `^.*\(\d+\)$`,
	"Content Version":              `^AppThreat-\d+-\d+$`,
	"High Resolution Timestamp":    `^\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d\.\d{3}[-+]\d\d:\d\d$`,
	"High Res Timestamp":           `^\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d\.\d{3}[-+]\d\d:\d\d$`,
	"Source Location":              `^(\d+\.\d+\.\d+\.\d+-\d+\.\d+\.\d+\.\d+|[A-Z][a-z]+( [A-Z][a-z]+)*)$`,
	"Destination Location":         `^(\d+\.\d+\.\d+\.\d+-\d+\.\d+\.\d+\.\d+|[A-Z][a-z]+( [A-Z][a-z]+)*)$`,
	"Source Country":               `^(\d+\.\d+\.\d+\.\d+-\d+\.\d+\.\d+\.\d+|[A-Z][a-z]+( [A-Z][a-z]+)*)$`,
	"Destination Country":          `^(\d+\.\d+\.\d+\.\d+-\d+\.\d+\.\d+\.\d+|[A-Z][a-z]+( [A-Z][a-z]+)*)$`,
	"Source Region":                `^(\d+\.\d+\.\d+\.\d+-\d+\.\d+\.\d+\.\d+|[A-Z][a-z]+( [A-Z][a-z]+)*)$`,
	"Event ID":                     `^[a-z0-9-]+$`,
	"Description":                  `.*`,
	"Configuration Path":           `.*`,
	"Error":                        `.*`,
	"URL/Filename":                 `.*`,
	"Client OS Version":            `.*`,
	"Subject Common Name":          `^[\w.*-]+$`,
	"Server Name Indication":       `^[\w.-]+$`,
	"Issuer Subject Common Name":   `.+`,
	"Root Subject Common Name":     `.+`,
	"SCTP Chunk Type":              `^[A-Z ]*$`,
	"Stage for Client to Firewall": `^[A-Za-z_]+$`,
	"Stage for Firewall to Server": `^[A-Za-z_]+$`,
}

// TestLogTypesMatchTheCapture pins the generator's type strings to the
// device's: AUTH and HIPMATCH, and no type the device does not send.
// CORRELATION was not captured.
func TestLogTypesMatchTheCapture(t *testing.T) {
	captured := loadCapture(t)
	for lt := range captured {
		require.Contains(t, logTypeCatalog, lt)
	}
	for _, lt := range logTypeCatalog {
		if lt != "CORRELATION" {
			require.Contains(t, captured, lt)
		}
	}
}

// TestFieldListsMatchTheCaptureWidth pins each type's field list to the
// device's record width.
func TestFieldListsMatchTheCaptureWidth(t *testing.T) {
	for lt, lines := range loadCapture(t) {
		for _, l := range lines {
			require.Len(t, logFieldNames[lt], len(l.fields), lt)
		}
	}
}

// TestFieldPatternsMatchTheCapture pins every fieldPatterns entry to the
// device: each captured value of that field matches.
func TestFieldPatternsMatchTheCapture(t *testing.T) {
	for lt, lines := range loadCapture(t) {
		names := logFieldNames[lt]
		if len(names) != len(lines[0].fields) {
			continue // TestFieldListsMatchTheCaptureWidth reports it
		}
		for i, name := range names {
			p, ok := fieldPatterns[name]
			if !ok {
				continue
			}
			re := regexp.MustCompile(p)
			for _, l := range lines {
				require.Regexp(t, re, l.fields[i], "%s %s (field %d)", lt, name, i+1)
			}
		}
	}
}

// TestValuesMatchTheCaptureShapes pins every generated value to what the
// device sends in that field: its fieldPatterns entry, or else one of the
// captured values' shapes, so a field the device leaves empty stays empty and
// one it fills is filled alike.
func TestValuesMatchTheCaptureShapes(t *testing.T) {
	for lt, lines := range loadCapture(t) {
		names := logFieldNames[lt]
		if len(names) != len(lines[0].fields) {
			continue
		}
		shapes := make([]map[string]bool, len(names))
		for i := range names {
			shapes[i] = map[string]bool{}
			for _, l := range lines {
				shapes[i][shape(l.fields[i])] = true
			}
		}
		dev := fallbackDevice()
		for range 300 {
			f := buildLogFields(lt, dev)
			if docOnly(lt, f) {
				continue
			}
			for i, name := range names {
				if p, ok := fieldPatterns[name]; ok {
					require.Regexp(t, p, f[i], "%s %s (field %d)", lt, name, i+1)
					continue
				}
				require.True(t, shapes[i][shape(f[i])], "%s %s (field %d) = %q; device sends %v", lt, name, i+1, f[i], shapes[i])
			}
		}
	}
}

// TestSubtypesWereCaptured pins every generated subtype to one the device
// sent, except the doc-only records.
func TestSubtypesWereCaptured(t *testing.T) {
	for lt, lines := range loadCapture(t) {
		subs := map[string]bool{}
		for _, l := range lines {
			subs[l.fields[4]] = true
		}
		dev := fallbackDevice()
		for range 300 {
			f := buildLogFields(lt, dev)
			if !docOnly(lt, f) {
				require.True(t, subs[f[4]], "%s subtype %q; device sends %v", lt, f[4], subs)
			}
		}
	}
}

// TestRecordSeverityFollowsTheDevice pins each record's syslog severity to the
// PRI the device sent for the same log type and Severity field: THREAT and
// SYSTEM by their Severity, every other type Info.
func TestRecordSeverityFollowsTheDevice(t *testing.T) {
	names := []string{"EMERG", "ALERT", "CRITICAL", "ERROR", "WARN", "NOTICE", "INFO", "DEBUG"}
	want := map[string]string{}
	for lt, lines := range loadCapture(t) {
		for _, l := range lines {
			key := lt
			if lt == "THREAT" || lt == "SYSTEM" {
				key += "/" + l.fields[indexOf(logFieldNames[lt], "Severity")]
			}
			want[key] = names[l.pri&7]
		}
	}
	dev := fallbackDevice()
	for range 3000 {
		lt := logTypeCatalog[randInt(0, len(logTypeCatalog)-1)]
		line, sev := renderLogLine(lt, dev)
		f, err := csv.NewReader(strings.NewReader(line[strings.Index(line, " 1,")+1:])).Read()
		require.NoError(t, err)
		key := lt
		if lt == "THREAT" || lt == "SYSTEM" {
			key += "/" + field(lt, f, "Severity")
		}
		if w, ok := want[key]; ok {
			require.Equal(t, w, sev, key)
		}
	}
	require.Equal(t, "ERROR", want["THREAT/high"], "the capture has a high THREAT")
}

// rawFields splits a CSV record and reports which fields were quoted.
func rawFields(body string) (vals []string, quoted []bool) {
	for i := 0; i <= len(body); {
		if i < len(body) && body[i] == '"' {
			var v strings.Builder
			j := i + 1
			for ; j < len(body); j++ {
				if body[j] == '"' {
					if j+1 < len(body) && body[j+1] == '"' {
						v.WriteByte('"')
						j++
						continue
					}
					break
				}
				v.WriteByte(body[j])
			}
			vals, quoted = append(vals, v.String()), append(quoted, true)
			i = j + 2
			continue
		}
		j := strings.IndexByte(body[i:], ',')
		if j < 0 {
			j = len(body) - i
		}
		vals, quoted = append(vals, body[i:i+j]), append(quoted, false)
		i += j + 1
	}
	return vals, quoted
}

// TestQuotingMatchesTheCapture pins CSV quoting to the device's: a field the
// device quotes whenever it has a value is quoted whenever generated with
// one, and every other field is quoted only when it holds a comma or quote.
func TestQuotingMatchesTheCapture(t *testing.T) {
	f, err := os.Open(captureFile)
	require.NoError(t, err)
	defer f.Close()
	always := map[string]map[int]bool{} // type -> field -> quoted every non-empty time
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		vals, quoted := rawFields(captureLineRe.FindStringSubmatch(sc.Text())[2])
		lt := vals[3]
		if always[lt] == nil {
			always[lt] = map[int]bool{}
		}
		for i, v := range vals {
			if v == "" {
				continue
			}
			if prev, seen := always[lt][i]; !seen || prev {
				always[lt][i] = quoted[i]
			}
		}
	}
	require.NoError(t, sc.Err())

	dev := fallbackDevice()
	for lt, fields := range always {
		for range 300 {
			line := formatLogLine(lt, dev)
			vals, quoted := rawFields(line[strings.Index(line, " 1,")+1:])
			for i, v := range vals {
				if v == "" {
					continue
				}
				want := fields[i] || strings.ContainsAny(v, ",\"")
				require.Equal(t, want, quoted[i], "%s %s (field %d) %q", lt, logFieldNames[lt][i], i+1, v)
			}
		}
	}
}

// TestThreatDetectionsWereCaptured pins every THREAT detection to one the
// device logged: its subtype, target, Threat ID, action, severity,
// direction, threat category, and flags together. A url record's Category is
// the one the device gave that URL, and a detection the device always
// logged with Category any keeps it.
func TestThreatDetectionsWereCaptured(t *testing.T) {
	key := func(f []string) string {
		get := func(n string) string { return field("THREAT", f, n) }
		return strings.Join([]string{get("Threat/Content Type"), get("URL/Filename"), get("Threat ID"), get("Action"),
			get("Severity"), get("Direction"), get("Threat Category"), get("Flags")}, "|")
	}
	seen := map[string]bool{}
	categories := map[string]map[string]bool{} // subtype|target -> categories
	for _, l := range loadCapture(t)["THREAT"] {
		seen[key(l.fields)] = true
		target := field("THREAT", l.fields, "Threat/Content Type") + "|" + field("THREAT", l.fields, "URL/Filename")
		if categories[target] == nil {
			categories[target] = map[string]bool{}
		}
		categories[target][field("THREAT", l.fields, "Category")] = true
	}
	dev := fallbackDevice()
	for range 2000 {
		f := buildLogFields("THREAT", dev)
		if docOnly("THREAT", f) {
			continue
		}
		require.True(t, seen[key(f)], "THREAT detection not captured: %s", key(f))
		sub, cat := field("THREAT", f, "Threat/Content Type"), field("THREAT", f, "Category")
		cats := categories[sub+"|"+field("THREAT", f, "URL/Filename")]
		if sub == "url" || (len(cats) == 1 && cats["any"]) {
			require.True(t, cats[cat], "%s %s Category %q; device sent %v", sub, field("THREAT", f, "URL/Filename"), cat, cats)
		}
	}
}

// TestSystemEventSlotsMatchTheCapture pins the two unnamed fields after a
// SYSTEM event's Object to the values the device sent for that Event ID.
func TestSystemEventSlotsMatchTheCapture(t *testing.T) {
	seen := map[string]bool{}
	for _, l := range loadCapture(t)["SYSTEM"] {
		seen[l.fields[8]+"|"+l.fields[10]+"|"+l.fields[11]] = true
	}
	dev := fallbackDevice()
	for range 1000 {
		f := buildLogFields("SYSTEM", dev)
		k := f[8] + "|" + f[10] + "|" + f[11]
		require.True(t, seen[k], "SYSTEM %s not captured", k)
	}
}

// TestPanDBVersionFollowsTheRecordDate pins the PAN-DB upgrade message to the
// device's form: version <record date>.<build>, the build rising per upgrade.
func TestPanDBVersionFollowsTheRecordDate(t *testing.T) {
	re := regexp.MustCompile(`^PAN-DB was upgraded to version (\d{8})\.(\d{5})\.$`)
	dev := fallbackDevice()
	last := -1
	for range 3000 {
		f := buildLogFields("SYSTEM", dev)
		if field("SYSTEM", f, "Event ID") != "upgrade-url-database-success" {
			continue
		}
		m := re.FindStringSubmatch(field("SYSTEM", f, "Description"))
		require.NotNil(t, m, field("SYSTEM", f, "Description"))
		gen, err := time.ParseInLocation("2006/01/02 15:04:05", field("SYSTEM", f, "Generated Time"), time.Local)
		require.NoError(t, err)
		require.Equal(t, gen.Format("20060102"), m[1])
		build, _ := strconv.Atoi(m[2])
		require.Greater(t, build, last)
		last = build
	}
	require.GreaterOrEqual(t, last, 0, "no PAN-DB upgrade in 3000 records")
}

// trafficOutcome abstracts a TRAFFIC record to the session outcome the device
// chose: subtype, App-ID, end reason, and Category, with the URL categories
// PAN-DB gives public destinations folded into one "url" class.
func trafficOutcome(f []string) string {
	get := func(n string) string { return field("TRAFFIC", f, n) }
	cat := get("Category")
	switch cat {
	case "computer-and-internet-info", "search-engines", "business-and-economy",
		"content-delivery-networks", "reference-and-research", "shareware-and-freeware":
		cat = "url"
	}
	return strings.Join([]string{get("Threat/Content Type"), get("Application"), get("Session End Reason"), cat}, "|")
}

// TestTrafficOutcomesCoverTheCapture pins TRAFFIC sessions to the device:
// every outcome the capture shows for a generated App-ID is generated, and an
// end record's start lag (Generated Time minus Start Time minus Elapsed Time)
// is mostly the 17-20 s the device holds a closed session before logging it.
func TestTrafficOutcomesCoverTheCapture(t *testing.T) {
	ours := map[string]bool{"not-applicable": true}
	for _, a := range apps {
		ours[a] = true
	}
	want := map[string]bool{}
	for _, l := range loadCapture(t)["TRAFFIC"] {
		if ours[field("TRAFFIC", l.fields, "Application")] {
			want[trafficOutcome(l.fields)] = true
		}
	}
	got := map[string]bool{}
	ends, held := 0, 0
	dev := fallbackDevice()
	for range 20000 {
		f := buildLogFields("TRAFFIC", dev)
		got[trafficOutcome(f)] = true
		if field("TRAFFIC", f, "Threat/Content Type") != "end" {
			continue
		}
		gen, err := time.ParseInLocation("2006/01/02 15:04:05", field("TRAFFIC", f, "Generated Time"), time.Local)
		require.NoError(t, err)
		start, err := time.ParseInLocation("2006/01/02 15:04:05", field("TRAFFIC", f, "Start Time"), time.Local)
		require.NoError(t, err)
		lag := int(gen.Sub(start)/time.Second) - atoi(t, field("TRAFFIC", f, "Elapsed Time"))
		ends++
		if lag >= 17 && lag <= 20 {
			held++
		}
	}
	for o := range want {
		require.True(t, got[o], "captured TRAFFIC outcome never generated: %s", o)
	}
	require.Greater(t, held*2, ends, "end start lag mostly 17-20 s: %d of %d", held, ends)
}

// TestNATFlagFollowsTranslation pins the session flag the device sets on a
// translated session: TRAFFIC, THREAT, and DECRYPTION Flags carry 0x400000
// exactly when NAT Source IP is set, in the capture and in generated records.
func TestNATFlagFollowsTranslation(t *testing.T) {
	check := func(logType string, f []string) {
		flags, err := strconv.ParseUint(field(logType, f, "Flags"), 0, 64)
		require.NoError(t, err)
		require.Equal(t, field(logType, f, "NAT Source IP") != "0.0.0.0", flags&0x400000 != 0,
			"%s Flags %s with NAT Source IP %s", logType, field(logType, f, "Flags"), field(logType, f, "NAT Source IP"))
	}
	capture := loadCapture(t)
	dev := fallbackDevice()
	for _, lt := range []string{"TRAFFIC", "THREAT", "DECRYPTION"} {
		for _, l := range capture[lt] {
			check(lt, l.fields)
		}
		for range 2000 {
			check(lt, buildLogFields(lt, dev))
		}
	}
}

// TestSessionIDsKeepRising pins the Session ID as the device's rising session
// counter: past the capture's six digits a long run keeps counting up, and
// the field pattern accepts it.
func TestSessionIDsKeepRising(t *testing.T) {
	dev := fallbackDevice()
	var f []string
	// A drop carries session 0, so draw until the record is not one.
	for f == nil || field("TRAFFIC", f, "Threat/Content Type") == "drop" {
		dev.sessions.Store(999_999)
		f = buildLogFields("TRAFFIC", dev)
	}
	got := field("TRAFFIC", f, "Session ID")
	require.Equal(t, "1000000", got)
	require.Regexp(t, fieldPatterns["Session ID"], got)
}

// TestOnlySessionsTakeSessionIDs pins the capture's session numbering: IDs
// rise by one per session, so records without a session (SYSTEM, CONFIG,
// User-ID, AUTH, and the rest) and drops take none.
func TestOnlySessionsTakeSessionIDs(t *testing.T) {
	dev := fallbackDevice()
	session := func() uint64 {
		for {
			f := buildLogFields("TRAFFIC", dev)
			if field("TRAFFIC", f, "Threat/Content Type") != "drop" {
				id, err := strconv.ParseUint(field("TRAFFIC", f, "Session ID"), 10, 64)
				require.NoError(t, err)
				return id
			}
		}
	}
	first := session()
	for _, lt := range []string{"SYSTEM", "CONFIG", "AUTH", "CORRELATION", "GLOBALPROTECT", "HIPMATCH", "IPTAG", "USERID"} {
		for range 5 {
			buildLogFields(lt, dev)
		}
	}
	require.Equal(t, first+1, session())
}
