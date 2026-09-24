// Package wire holds the fixed-width integer conversions the flow encoders use
// to pack values into NetFlow/IPFIX/sFlow wire fields. Each flow protocol
// defines its fields at a fixed width, so narrowing a wider in-memory value to
// that width is the protocol's defined behavior, not an accidental overflow —
// the gosec G115 suppression lives here, in one place, rather than on every
// call site.
package wire

// integer is any integer type an encoder narrows from.
type integer interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 | ~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64
}

// U32 narrows v to a 32-bit wire field.
func U32[T integer](v T) uint32 {
	return uint32(v) // #nosec G115 -- fixed-width wire field; truncation is per protocol spec
}

// U16 narrows v to a 16-bit wire field.
func U16[T integer](v T) uint16 {
	return uint16(v) // #nosec G115 -- fixed-width wire field; truncation is per protocol spec
}

// U8 narrows v to an 8-bit wire field.
func U8[T integer](v T) uint8 {
	return uint8(v) // #nosec G115 -- fixed-width wire field; truncation is per protocol spec
}

// U64 fits v into a 64-bit wire field (used for unsigned counters/timestamps).
func U64[T integer](v T) uint64 {
	return uint64(v) // #nosec G115 -- fixed-width wire field; sign reinterpretation is per protocol spec
}
