package paloalto

import (
	"net"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// wantFieldCount is the authoritative PAN-OS 11.0 field count per log type,
// validated against Palo Alto's public "Syslog Field Descriptions" (11.0). For
// the four high-width types the count excludes fields the page marks 11.1+/
// 12.1.2+ (see logFieldNames doc comment).
var wantFieldCount = map[string]int{
	"TRAFFIC":        115,
	"THREAT":         121,
	"SYSTEM":         26,
	"CONFIG":         28,
	"AUTHENTICATION": 47,
	"CORRELATION":    22,
	"DECRYPTION":     106,
	"GLOBALPROTECT":  50,
	"GTP":            94,
	"HIP-MATCH":      32,
	"IPTAG":          27,
	"SCTP":           65,
	"USERID":         37,
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
				f := buildLogFields(lt)
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
		// Field 6 is FUTURE_USE for every type except DECRYPTION (Config Version).
		if lt != "DECRYPTION" {
			assert.Equal(t, "FUTURE_USE", names[5], "%s field 6", lt)
		} else {
			assert.Equal(t, "Config Version", names[5], "DECRYPTION field 6")
		}
	}

	// Per-type anchors deep in the record — these lock tail ordering.
	anchors := map[string]map[string]int{
		"TRAFFIC":    {"Rule Name": 11, "Protocol": 29, "Source Country": 41, "Session End Reason": 46, "Action Source": 53, "Offloaded": 114},
		"THREAT":     {"Rule Name": 11, "Severity": 34, "Report ID": 53, "Cloud Report ID": 120},
		"DECRYPTION": {"Config Version": 5, "TLS Version": 39, "Server Name Indication": 63, "Application Sanctioned State": 105},
		"GTP":        {"MSISDN": 32, "Access Point Name": 33, "Application Sanctioned State": 93},
		"SCTP":       {"SCTP Association ID": 40, "UUID for rule": 63, "High Resolution Timestamp": 64},
		"USERID":     {"Origin Data Source": 34, "Cluster Name": 36},
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
		f := buildLogFields(lt)
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
		f := buildLogFields(lt)
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
	f := buildLogFields("TRAFFIC")
	names := logFieldNames["TRAFFIC"]
	assert.NotEmpty(t, f[indexOf(names, "Rule Name")], "TRAFFIC Rule Name (pos 12)")
	assert.NotNil(t, net.ParseIP(f[indexOf(names, "Source Address")]), "TRAFFIC Source Address must be an IP")
	assert.NotNil(t, net.ParseIP(f[indexOf(names, "Destination Address")]), "TRAFFIC Destination Address must be an IP")

	f = buildLogFields("USERID")
	assert.Equal(t, "1", f[0], "USERID field 1 must be FUTURE_USE=1 (blueprint FUTURE_USER is a bug)")
}

func TestFormatLogLineFieldCount(t *testing.T) {
	for _, lt := range logTypeCatalog {
		line := formatLogLine(lt)
		idx := strings.Index(line, " 1,")
		require.GreaterOrEqual(t, idx, 0, "%s line must contain the CSV start", lt)
		fields := strings.Split(line[idx+1:], ",")
		assert.Equal(t, wantFieldCount[lt], len(fields), "%s CSV field count", lt)
	}
}
