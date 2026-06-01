package catalog

// MessageDefinition is the recipe for one FIX message type at one
// specific protocol Version. Each MessageDefinition declares:
//
//   - the FIX MsgType code (tag 35 value, e.g. "D" for NewOrderSingle)
//   - the Version it applies to
//   - the ordered list of FieldGenerators that produce the body fields
//     (BeginString, BodyLength, MsgType, and CheckSum are added by the
//     framing layer in BuildMessage — definitions only supply the body)
//
// Per-asset-category and per-version subpackages register
// MessageDefinitions with the Registry at init time.
type MessageDefinition struct {
	// Version is the FIX protocol version this definition targets.
	Version Version
	// MsgType is the FIX MsgType code, e.g. "A" for Logon, "D" for
	// NewOrderSingle, "8" for ExecutionReport.
	MsgType string
	// AssetCategory is the asset category this definition belongs to.
	// Session-layer definitions (Logon, Heartbeat, etc.) use
	// AssetCategoryUnknown to signal "asset-agnostic."
	AssetCategory AssetCategory
	// Fields is the ordered list of body field generators. The framing
	// layer prepends the header (BeginString, BodyLength, MsgType,
	// SenderCompID, TargetCompID, MsgSeqNum, SendingTime) and appends
	// CheckSum — definitions only supply the application body.
	Fields []FieldGenerator
}

// MessageKey uniquely identifies a MessageDefinition within the
// Registry by (Version, MsgType, AssetCategory). The AssetCategory
// component lets the same MsgType (e.g. "D" NewOrderSingle) coexist
// across multiple categories with different field generators.
type MessageKey struct {
	Version       Version
	MsgType       string
	AssetCategory AssetCategory
}

// Key returns the MessageKey for the definition.
func (d *MessageDefinition) Key() MessageKey {
	return MessageKey{
		Version:       d.Version,
		MsgType:       d.MsgType,
		AssetCategory: d.AssetCategory,
	}
}
