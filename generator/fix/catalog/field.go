package catalog

import (
	"math/rand"
	"strconv"
)

// Tag is a FIX tag number (e.g. 35 for MsgType, 49 for SenderCompID).
type Tag int

// Field is one tag/value pair on the wire. Value is the textual form
// that will be emitted; FIX is a text-on-the-wire protocol with one
// fixed encoding for each tag.
type Field struct {
	Tag   Tag
	Value string
}

// FieldGenerator produces a Field given the supplied RNG and
// build-time context. Generators MUST source ALL randomness from the
// provided *rand.Rand — never from rand.Intn or rand.New(...) inline —
// so the FIX generator's "same seed → identical output" guarantee
// holds.
type FieldGenerator func(r *rand.Rand, ctx *GenerateCtx) Field

// GenerateCtx carries shared inputs available to every FieldGenerator
// during a message build. Fields here are read-only from a generator's
// perspective; mutation is the StateTracker's responsibility and lives
// behind its own locking.
type GenerateCtx struct {
	// Version is the FIX protocol version being emitted.
	Version Version
	// AssetCategory identifies which per-category modeling code is active.
	AssetCategory AssetCategory
	// SenderCompID is the FIX SenderCompID (tag 49) for this session.
	SenderCompID string
	// TargetCompID is the FIX TargetCompID (tag 56) for this session.
	TargetCompID string
	// SeqNum is the outgoing sequence number for the message under
	// construction (tag 34, MsgSeqNum).
	SeqNum int
	// SendingTime is the value to emit for tag 52, SendingTime. The
	// caller is responsible for choosing this so it stays consistent
	// across a build session — never read from time.Now() inside a
	// generator.
	SendingTime string
	// Memo is per-message scratch space for FieldGenerators that need
	// to share derived state across multiple fields within one message
	// — e.g. an instrument pick that must stay consistent across
	// Symbol/SecurityType/SecurityID/price/etc. so the message is
	// internally coherent. Lazily initialized by the first writer in
	// each message; keys are conventionally typed values defined by
	// the package using them.
	Memo map[any]any
}

// LiteralField returns a FieldGenerator that always emits the given
// tag with the given fixed string value. Useful for header fields like
// MsgType where the wire form is constant for a message definition.
func LiteralField(tag Tag, value string) FieldGenerator {
	return func(_ *rand.Rand, _ *GenerateCtx) Field {
		return Field{Tag: tag, Value: value}
	}
}

// IntField returns a FieldGenerator that emits a fixed-int tag/value
// pair. The value is rendered via strconv.Itoa.
func IntField(tag Tag, value int) FieldGenerator {
	return func(_ *rand.Rand, _ *GenerateCtx) Field {
		return Field{Tag: tag, Value: strconv.Itoa(value)}
	}
}
