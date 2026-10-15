package paloalto

import (
	"encoding/csv"
	"net"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// wantFieldCount is each type's record width on a PAN-OS 11.0.4-h6 firewall
// (testdata capture); CORRELATION, never captured, keeps its 11.0 field page's.
var wantFieldCount = map[string]int{
	"TRAFFIC":       117,
	"THREAT":        123,
	"SYSTEM":        26,
	"CONFIG":        26,
	"AUTH":          47,
	"CORRELATION":   22,
	"DECRYPTION":    108,
	"GLOBALPROTECT": 50,
	"GTP":           106,
	"HIPMATCH":      32,
	"IPTAG":         28,
	"SCTP":          73,
	"USERID":        37,
}

func indexOf(names []string, target string) int {
	for i, n := range names {
		if n == target {
			return i
		}
	}
	return -1
}

func TestAllThirteenLogTypesPresent(t *testing.T) {
	require.Len(t, logTypeCatalog, 13)
	for lt := range wantFieldCount {
		assert.Contains(t, logTypeCatalog, lt, "catalog missing %s", lt)
		assert.Contains(t, logFieldNames, lt, "field-name list missing %s", lt)
	}
}

func TestFieldNameListLengthsMatchSpec(t *testing.T) {
	for _, lt := range logTypeCatalog {
		assert.Equal(t, wantFieldCount[lt], len(logFieldNames[lt]),
			"%s spec field-name list must have %d entries", lt, wantFieldCount[lt])
	}
}

func TestBuildMatchesNameListLength(t *testing.T) {
	for _, lt := range logTypeCatalog {
		lt := lt
		t.Run(lt, func(t *testing.T) {
			for i := 0; i < 20; i++ {
				f := buildLogFields(lt, fallbackDevice())
				require.Equal(t, len(logFieldNames[lt]), len(f),
					"%s value count must equal its spec field-name count", lt)
			}
		})
	}
}

// TestExactFieldOrder pins the spec order at named anchor positions across the
// full width of every type. A field inserted, dropped, or reordered shifts an
// anchor and fails here.
func TestExactFieldOrder(t *testing.T) {
	// The shared 7-field header (position, name) is identical for all types
	// except field 5, whose meaning is type-specific (kept out of this check).
	for _, lt := range logTypeCatalog {
		names := logFieldNames[lt]
		assert.Equal(t, "FUTURE_USE", names[0], "%s field 1", lt)
		assert.Equal(t, "Receive Time", names[1], "%s field 2", lt)
		assert.Equal(t, "Type", names[3], "%s field 4 must be Type", lt)
		// Field 6 carries the config version on the device; CORRELATION keeps
		// its page's FUTURE_USE.
		if lt != "CORRELATION" {
			assert.Equal(t, "Config Version", names[5], "%s field 6", lt)
		} else {
			assert.Equal(t, "FUTURE_USE", names[5], "CORRELATION field 6")
		}
	}

	// Per-type anchors deep in the record — these lock tail ordering.
	anchors := map[string]map[string]int{
		"TRAFFIC":    {"Rule Name": 11, "Protocol": 29, "Source Country": 41, "Session End Reason": 46, "Action Source": 53, "Offloaded": 114, "Flow Type": 115},
		"THREAT":     {"Rule Name": 11, "Severity": 34, "Report ID": 53, "Cloud Report ID": 120, "Flow Type": 122},
		"DECRYPTION": {"Config Version": 5, "TLS Version": 39, "Server Name Indication": 63, "Tunneled Application": 104, "Application Sanctioned State": 106},
		"GTP":        {"MSISDN": 32, "Sequence Number": 47, "Tunnel": 62, "Tunnel Inspection Rule": 79, "Application Sanctioned State": 104},
		"SCTP":       {"SCTP Association ID": 39, "UUID for rule": 63, "High Resolution Timestamp": 71},
		"USERID":     {"Origin Data Source": 34},
	}
	for lt, want := range anchors {
		names := logFieldNames[lt]
		for field, pos := range want {
			assert.Equal(t, pos, indexOf(names, field),
				"%s: %q must be at index %d (position %d)", lt, field, pos, pos+1)
		}
	}
}

func TestCommonHeaderValues(t *testing.T) {
	for _, lt := range logTypeCatalog {
		f := buildLogFields(lt, fallbackDevice())
		assert.Equal(t, "1", f[0], "%s field 1 FUTURE_USE=1", lt)
		assert.NotEmpty(t, f[1], "%s field 2 Receive Time", lt)
		assert.NotEmpty(t, f[2], "%s field 3 Serial", lt)
		assert.Equal(t, lt, f[3], "%s field 4 Type", lt)
		assert.NotEmpty(t, f[6], "%s field 7 Generated Time", lt)
	}
}

// TestFutureUseFieldsEmpty asserts every FUTURE_USE position (except field 1,
// which PAN-OS emits as "1") renders empty.
func TestFutureUseFieldsEmpty(t *testing.T) {
	for _, lt := range logTypeCatalog {
		names := logFieldNames[lt]
		f := buildLogFields(lt, fallbackDevice())
		for i, name := range names {
			if i == 0 || name != "FUTURE_USE" {
				continue
			}
			assert.Empty(t, f[i], "%s FUTURE_USE at position %d must be empty", lt, i+1)
		}
	}
}

// TestKeyFieldsPopulated checks that value-bearing fields carry realistic
// values at their spec position (order-linked: value read via field name index).
func TestKeyFieldsPopulated(t *testing.T) {
	f := buildLogFields("TRAFFIC", fallbackDevice())
	names := logFieldNames["TRAFFIC"]
	assert.NotEmpty(t, f[indexOf(names, "Rule Name")], "TRAFFIC Rule Name (pos 12)")
	assert.NotNil(t, net.ParseIP(f[indexOf(names, "Source Address")]), "TRAFFIC Source Address must be an IP")
	assert.NotNil(t, net.ParseIP(f[indexOf(names, "Destination Address")]), "TRAFFIC Destination Address must be an IP")

	f = buildLogFields("USERID", fallbackDevice())
	assert.Equal(t, "1", f[0], "USERID field 1 must be FUTURE_USE=1 (blueprint FUTURE_USER is a bug)")
}

func TestFormatLogLineFieldCount(t *testing.T) {
	for _, lt := range logTypeCatalog {
		line := formatLogLine(lt, fallbackDevice())
		idx := strings.Index(line, " 1,")
		require.GreaterOrEqual(t, idx, 0, "%s line must contain the CSV start", lt)
		fields, err := csv.NewReader(strings.NewReader(line[idx+1:])).Read()
		require.NoError(t, err, "%s line must be valid CSV", lt)
		assert.Equal(t, wantFieldCount[lt], len(fields), "%s CSV field count", lt)
	}
}

// gpEventStages maps each GlobalProtect Event ID to its stage.
var gpEventStages = func() map[string]string {
	m := map[string]string{}
	for _, e := range gpEvents {
		m[e.id] = e.stage
	}
	return m
}()

// buildLogFields returns the ordered CSV field slice for a log type, one value
// per field in logFieldNames order.
func buildLogFields(logType string, dev device) []string {
	_, fields := buildRecord(logType, dev)
	return fields
}

// formatLogLine renders the syslog header plus the comma-joined CSV fields.
func formatLogLine(logType string, dev device) string {
	line, _ := renderLogLine(logType, dev)
	return line
}
