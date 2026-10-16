// Package wire holds the fixed-width integer conversions the flow encoders use
// to pack values into NetFlow/IPFIX/sFlow wire fields. Each flow protocol
// defines its fields at a fixed width, so narrowing a wider in-memory value to
// that width is the protocol's defined behavior, not an accidental overflow —
// the gosec G115 suppression lives here, in one place, rather than on every
// call site.
package wire

import (
	"net"
	"time"

	"github.com/observiq/blitz/embed"
)

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

// IP protocol numbers for ICMP and ICMPv6.
const (
	ProtoICMP   uint8 = 1
	ProtoICMPv6 uint8 = 58
)

// IsICMP reports whether proto is ICMP or ICMPv6.
func IsICMP(proto uint8) bool { return proto == ProtoICMP || proto == ProtoICMPv6 }

// ICMPTypeCode is an ICMP or ICMPv6 flow's type*256+code, which a FlowRecord
// carries in DstPort (the NetFlow v5 convention); 0 for every other protocol.
// It feeds the NetFlow v9 ICMP_TYPE field (32) and the IPFIX icmpTypeCodeIPv4
// (32) and icmpTypeCodeIPv6 (139) IEs.
func ICMPTypeCode(proto uint8, dstPort uint16) uint16 {
	if !IsICMP(proto) {
		return 0
	}
	return dstPort
}

// IsIPv6 reports whether f is an IPv6 flow: either endpoint is an address with
// no IPv4 form. A nil address counts as IPv4 (encoded as zeros).
func IsIPv6(f embed.FlowRecord) bool {
	return (f.SrcIP != nil && f.SrcIP.To4() == nil) || (f.DstIP != nil && f.DstIP.To4() == nil)
}

// IP4 returns the 4-byte form of ip, or zeros if ip is nil or not IPv4.
func IP4(ip net.IP) []byte {
	if v4 := ip.To4(); v4 != nil {
		return v4
	}
	return make([]byte, net.IPv4len)
}

// IP16 returns the 16-byte form of ip, or zeros if ip is nil.
func IP16(ip net.IP) []byte {
	if v6 := ip.To16(); v6 != nil {
		return v6
	}
	return make([]byte, net.IPv6len)
}

// SamplingRate is the 1-in-N packet sampling rate an encoder reports for a
// record: FlowRecord's 0 ("unsampled") is 1 in 1.
func SamplingRate(r uint32) uint32 { return max(r, 1) }

// Pad4 rounds n up to a multiple of 4, the flowset/set alignment NetFlow v9
// and IPFIX pad to.
func Pad4(n int) int { return (n + 3) &^ 3 }

// Template resend cadence shared by the NetFlow v9 and IPFIX encoders:
// whichever comes first.
const (
	TemplateRefreshInterval = 60 * time.Second
	TemplateRefreshRecords  = 256
)

// UptimeMS is t as milliseconds since boot, the switch-uptime clock NetFlow
// v5/v9 and sFlow timestamp with. A zero t is 0, and a t before boot clamps to
// 0 rather than wrapping the unsigned field to ~49 days. Past 2^32 ms the value
// wraps, as a real device's uptime counter does.
func UptimeMS(boot, t time.Time) uint32 {
	if t.IsZero() || t.Before(boot) {
		return 0
	}
	return U32(t.Sub(boot).Milliseconds())
}

// ExporterUptime is how long the simulated exporter has been up when a stream
// starts: a realistic uptime, far longer than any flow's back-dated start
// (the flow generator back-dates by at most its 60 s active timeout).
const ExporterUptime = 72 * time.Hour

// Boot is the simulated exporter's boot time for a stream starting at now.
func Boot(now time.Time) time.Time { return now.Add(-ExporterUptime) }

// MaxPayload is the largest UDP payload an encoder emits: a 1500-byte MTU less
// the IPv6 (40) and UDP (8) headers, so a packet fits the MTU over either IP
// version (RFC 7011 §10.3.3: messages must not exceed the path MTU).
const MaxPayload = 1500 - 40 - 8

// ASTrans is AS_TRANS (RFC 6793), the 2-byte stand-in for a 4-byte ASN.
const ASTrans = 23456

// AS16 fits an ASN into a 2-byte AS field: AS_TRANS when it needs 4 bytes.
func AS16(as uint32) uint16 {
	if as > 0xFFFF {
		return ASTrans
	}
	return uint16(as)
}
