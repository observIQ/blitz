// Package catalog defines the FIX protocol message catalog used by the
// FIX generator: protocol versions, asset classes, field generators,
// message definitions, and SOH framing helpers.
//
// The catalog is intentionally version-aware: callers select a Version
// to drive which message definitions and tag conventions apply.
package catalog

// Version identifies a supported FIX protocol version. The three values
// here are the only ones blitz's FIX generator emits — 4.0 / 4.1 / 4.3
// are deliberately out of scope.
type Version int

const (
	// VersionUnknown is the zero value; not a valid emission target.
	VersionUnknown Version = iota
	// V42 is FIX 4.2. BeginString "FIX.4.2".
	V42
	// V44 is FIX 4.4. BeginString "FIX.4.4".
	V44
	// V50SP2 is FIX 5.0 SP2 over the FIXT.1.1 session layer.
	// BeginString "FIXT.1.1"; ApplVerID (tag 1128) = 9.
	V50SP2
)

// BeginString returns the wire BeginString (tag 8) value for the
// version. Returns the empty string for VersionUnknown.
func (v Version) BeginString() string {
	switch v {
	case V42:
		return "FIX.4.2"
	case V44:
		return "FIX.4.4"
	case V50SP2:
		return "FIXT.1.1"
	default:
		return ""
	}
}

// ApplVerID returns the ApplVerID (tag 1128) value used by FIXT-layer
// versions to identify the application-layer FIX version. Returns the
// empty string for versions that do not carry ApplVerID (4.2, 4.4 — they
// encode the version in BeginString itself).
func (v Version) ApplVerID() string {
	switch v {
	case V50SP2:
		return "9"
	default:
		return ""
	}
}

// String returns a short human label for the version, suitable for
// logging and config error messages.
func (v Version) String() string {
	switch v {
	case V42:
		return "FIX.4.2"
	case V44:
		return "FIX.4.4"
	case V50SP2:
		return "FIX.5.0SP2"
	default:
		return "unknown"
	}
}

// AllVersions returns the list of supported versions in declaration
// order. Useful for test matrices and config validation.
func AllVersions() []Version {
	return []Version{V42, V44, V50SP2}
}

// VersionFromString parses a config-friendly version token into a
// Version. Accepted tokens: "4.2", "4.4", "5.0sp2" (case-insensitive).
// Returns VersionUnknown and a non-nil error for any other input.
func VersionFromString(s string) (Version, error) {
	switch s {
	case "4.2", "FIX.4.2", "fix.4.2":
		return V42, nil
	case "4.4", "FIX.4.4", "fix.4.4":
		return V44, nil
	case "5.0sp2", "5.0SP2", "FIX.5.0SP2", "fix.5.0sp2", "FIXT.1.1", "fixt.1.1":
		return V50SP2, nil
	default:
		return VersionUnknown, &VersionParseError{Token: s}
	}
}

// VersionParseError reports an unrecognized version token.
type VersionParseError struct{ Token string }

func (e *VersionParseError) Error() string {
	return "fix: unknown version " + e.Token + " (want one of: 4.2, 4.4, 5.0sp2)"
}
