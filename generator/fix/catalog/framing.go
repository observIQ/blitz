package catalog

import (
	"strconv"
	"strings"
)

// SOH is the FIX field delimiter — ASCII 0x01.
const SOH = '\x01'

// Standard tag numbers used by the framing layer. Application tags
// live in version-specific files (or per-asset files) — only the
// session-framing tags belong here.
const (
	TagBeginString  Tag = 8
	TagBodyLength   Tag = 9
	TagMsgType      Tag = 35
	TagSenderCompID Tag = 49
	TagTargetCompID Tag = 56
	TagMsgSeqNum    Tag = 34
	TagSendingTime  Tag = 52
	TagCheckSum     Tag = 10
)

// EncodeField writes one SOH-terminated `tag=value` field to b.
// Returns b with the field appended.
func EncodeField(b []byte, f Field) []byte {
	b = strconv.AppendInt(b, int64(f.Tag), 10)
	b = append(b, '=')
	b = append(b, f.Value...)
	b = append(b, SOH)
	return b
}

// EncodeFields writes a sequence of fields in order. Equivalent to
// repeated EncodeField but reads cleaner at the call site.
func EncodeFields(b []byte, fields []Field) []byte {
	for _, f := range fields {
		b = EncodeField(b, f)
	}
	return b
}

// BodyLength returns the FIX BodyLength of body — the byte count from
// the first byte AFTER the BodyLength field's terminating SOH up to and
// including the SOH before the CheckSum field. In practice this is
// "length of the entire body slice as supplied," because callers build
// the body exactly to that definition.
//
// Per the FIX spec (4.2 section "Message Format" and identical in 4.4 /
// 5.0): BodyLength counts MsgType through and including the SOH before
// CheckSum.
func BodyLength(body []byte) int {
	return len(body)
}

// CheckSum computes the FIX CheckSum (tag 10) value for the byte stream
// from the start of the message through the SOH that immediately
// precedes the CheckSum field. CheckSum is the unsigned sum of all
// bytes modulo 256, formatted as a 3-digit zero-padded decimal string.
func CheckSum(everythingBeforeCheckSum []byte) string {
	var sum uint32
	for _, c := range everythingBeforeCheckSum {
		sum += uint32(c)
	}
	return checksum3(sum % 256)
}

// checksum3 formats a checksum byte as a 3-digit zero-padded string
// without allocating via fmt.
func checksum3(v uint32) string {
	var b [3]byte
	b[0] = byte('0' + (v/100)%10)
	b[1] = byte('0' + (v/10)%10)
	b[2] = byte('0' + v%10)
	return string(b[:])
}

// BuildMessage assembles a complete FIX message on the wire from the
// header fields (everything before BodyLength), the body fields
// (MsgType + remaining payload), and computes BodyLength + CheckSum
// per spec.
//
// The shape is: BeginString | BodyLength | <body> | CheckSum.
// Callers supply BeginString in beginString (a Version's BeginString())
// and the full ordered body in body. BuildMessage does the rest.
//
// Returns the SOH-framed message as a single byte slice.
func BuildMessage(beginString string, body []Field) []byte {
	// First serialize the body so we can compute BodyLength.
	bodyBytes := EncodeFields(nil, body)

	// Now build header into a buffer.
	out := make([]byte, 0, len(bodyBytes)+32)
	out = EncodeField(out, Field{Tag: TagBeginString, Value: beginString})
	out = EncodeField(out, Field{Tag: TagBodyLength, Value: strconv.Itoa(BodyLength(bodyBytes))})
	out = append(out, bodyBytes...)

	// CheckSum covers everything emitted so far (header + body),
	// including SOHs.
	out = EncodeField(out, Field{Tag: TagCheckSum, Value: CheckSum(out)})
	return out
}

// SplitFields parses an on-the-wire FIX message back into its ordered
// (tag, value) field list. Intended for tests, NOT for production
// consumption — there is no validation of BodyLength or CheckSum here.
// Returns nil on malformed input.
func SplitFields(msg []byte) []Field {
	parts := strings.Split(string(msg), string(SOH))
	out := make([]Field, 0, len(parts))
	for _, p := range parts {
		if p == "" {
			continue
		}
		eq := strings.IndexByte(p, '=')
		if eq <= 0 {
			return nil
		}
		tag, err := strconv.Atoi(p[:eq])
		if err != nil {
			return nil
		}
		out = append(out, Field{Tag: Tag(tag), Value: p[eq+1:]})
	}
	return out
}
