// Package ipfix encodes blitz FlowRecords into IPFIX (RFC 7011) export
// messages. IPFIX is the IETF standardization of NetFlow v9: template-based,
// but with a 16-byte header, absolute millisecond timestamps, and 64-bit
// counters. Field IDs and widths follow the IANA IPFIX Information Element
// registry.
//
// A Vendor adds one real enterprise-specific Information Element (a field
// whose ID has the 0x8000 bit set, followed by the vendor's 4-byte IANA
// Private Enterprise Number) to every record, so the export decodes as that
// vendor's (see JuniperCommonProperties and CitrixConnectionID).
package ipfix

import (
	"encoding/binary"
	"hash/fnv"
	"slices"
	"sync"
	"time"

	"github.com/observiq/blitz/embed"
	"github.com/observiq/blitz/flow/wire"
)

// IPFIX wire constants (RFC 7011).
const (
	headerLen = 16

	setIDTemplate        uint16 = 2
	setIDOptionsTemplate uint16 = 3

	templateIDv4      uint16 = 256
	templateIDv6      uint16 = 257
	templateIDOptions uint16 = 258

	// enterpriseBit marks a field as enterprise-specific; the 4-byte PEN
	// follows the field specifier (RFC 7011 §3.2).
	enterpriseBit uint16 = 0x8000

	samplingDeterministic uint8 = 1 // samplingAlgorithm: deterministic

	// Options template set: header, record header (template id, field count,
	// scope field count), scope observationDomainId (149/4), then
	// samplingInterval (34/4) and samplingAlgorithm (35/1), padded to 4.
	optionsTemplateSetLen = 4 + 6 + 3*4 + 2
	// Options data set: header + one record (domain 4, interval 4,
	// algorithm 1), padded to 4.
	optionsDataSetLen = 4 + 12
)

// Vendor is an enterprise-specific Information Element: the vendor's IANA
// PEN, its element ID (without the enterprise bit), the value's byte length,
// and how to derive the value from a flow.
type Vendor struct {
	PEN  uint32
	IEID uint16
	Len  uint16
	put  func(b []byte, f embed.FlowRecord)
}

// JuniperCommonProperties is Juniper's Common Properties IE (PEN 2636, IE
// 137, 4 bytes), as Wireshark's packet-netflow.c decodes it: the top 6 bits
// are the CPID and the rest its description. The value is CPID 1 (forwarding
// class and drop priority) with class 0, drop priority 0 (best effort).
var JuniperCommonProperties = &Vendor{PEN: 2636, IEID: 137, Len: 4, put: func(b []byte, _ embed.FlowRecord) {
	binary.BigEndian.PutUint32(b, 1<<26)
}}

// CitrixConnectionID is Citrix NetScaler's Connection ID IE (PEN 5951, IE 133,
// uint32), which Wireshark's packet-netflow.c describes as identifying "Two
// flows of a TCP connection": both directions of a conversation share it, so
// the value hashes the protocol and the endpoint pair order-independently.
var CitrixConnectionID = &Vendor{PEN: 5951, IEID: 133, Len: 4, put: func(b []byte, f embed.FlowRecord) {
	// Fresh slices: appending to IP16's result could write into the caller's
	// backing array.
	a := append(slices.Clone(wire.IP16(f.SrcIP)), byte(f.SrcPort>>8), byte(f.SrcPort))
	z := append(slices.Clone(wire.IP16(f.DstIP)), byte(f.DstPort>>8), byte(f.DstPort))
	if string(a) > string(z) {
		a, z = z, a
	}
	h := fnv.New32a()
	_, _ = h.Write(append(append([]byte{f.Protocol}, a...), z...)) // hash.Hash.Write never returns an error
	binary.BigEndian.PutUint32(b, h.Sum32())
}}

// ie is one template field: an IANA IE ID, its byte length, and how to write
// it from a flow.
type ie struct {
	id  uint16
	len uint16
	put func(b []byte, f embed.FlowRecord)
}

// Fields shared by the IPv4 and IPv6 templates.
var (
	ieOctets   = ie{1, 8, func(b []byte, f embed.FlowRecord) { binary.BigEndian.PutUint64(b, f.Bytes) }}            // octetDeltaCount
	iePackets  = ie{2, 8, func(b []byte, f embed.FlowRecord) { binary.BigEndian.PutUint64(b, f.Packets) }}          // packetDeltaCount
	ieProto    = ie{4, 1, func(b []byte, f embed.FlowRecord) { b[0] = f.Protocol }}                                 // protocolIdentifier
	ieTOS      = ie{5, 1, func(b []byte, f embed.FlowRecord) { b[0] = f.TOS }}                                      // ipClassOfService
	ieTCPFlags = ie{6, 2, func(b []byte, f embed.FlowRecord) { binary.BigEndian.PutUint16(b, uint16(f.TCPFlags)) }} // tcpControlBits (unsigned16)
	ieSrcPort  = ie{7, 2, func(b []byte, f embed.FlowRecord) {
		binary.BigEndian.PutUint16(b, transportPort(f.Protocol, f.SrcPort))
	}} // sourceTransportPort
	ieIngress = ie{10, 4, func(b []byte, f embed.FlowRecord) { binary.BigEndian.PutUint32(b, f.InputIface) }} // ingressInterface
	ieDstPort = ie{11, 2, func(b []byte, f embed.FlowRecord) {
		binary.BigEndian.PutUint16(b, transportPort(f.Protocol, f.DstPort))
	}} // destinationTransportPort
	ieEgress = ie{14, 4, func(b []byte, f embed.FlowRecord) { binary.BigEndian.PutUint32(b, f.OutputIface) }}              // egressInterface
	ieSrcAS  = ie{16, 4, func(b []byte, f embed.FlowRecord) { binary.BigEndian.PutUint32(b, f.SrcAS) }}                    // bgpSourceAsNumber
	ieDstAS  = ie{17, 4, func(b []byte, f embed.FlowRecord) { binary.BigEndian.PutUint32(b, f.DstAS) }}                    // bgpDestinationAsNumber
	ieStart  = ie{152, 8, func(b []byte, f embed.FlowRecord) { binary.BigEndian.PutUint64(b, msSinceEpoch(f.StartTime)) }} // flowStartMilliseconds
	ieEnd    = ie{153, 8, func(b []byte, f embed.FlowRecord) { binary.BigEndian.PutUint64(b, msSinceEpoch(f.EndTime)) }}   // flowEndMilliseconds
	icmpTC   = func(b []byte, f embed.FlowRecord) {
		binary.BigEndian.PutUint16(b, wire.ICMPTypeCode(f.Protocol, f.DstPort))
	}
)

// templateV4 is the IPv4 template (id 256). Order defines the record layout.
var templateV4 = []ie{
	ieOctets, iePackets, ieProto, ieTOS, ieTCPFlags, ieSrcPort,
	{8, 4, func(b []byte, f embed.FlowRecord) { copy(b, wire.IP4(f.SrcIP)) }}, // sourceIPv4Address
	{9, 1, func(b []byte, f embed.FlowRecord) { b[0] = f.SrcMask }},           // sourceIPv4PrefixLength
	ieIngress, ieDstPort,
	{12, 4, func(b []byte, f embed.FlowRecord) { copy(b, wire.IP4(f.DstIP)) }}, // destinationIPv4Address
	{13, 1, func(b []byte, f embed.FlowRecord) { b[0] = f.DstMask }},           // destinationIPv4PrefixLength
	ieEgress,
	{15, 4, func(b []byte, f embed.FlowRecord) { copy(b, wire.IP4(f.NextHop)) }}, // ipNextHopIPv4Address
	ieSrcAS, ieDstAS, ieStart, ieEnd,
	{32, 2, icmpTC}, // icmpTypeCodeIPv4
}

// templateV6 is the IPv6 template (id 257).
var templateV6 = []ie{
	ieOctets, iePackets, ieProto, ieTOS, ieTCPFlags, ieSrcPort,
	{27, 16, func(b []byte, f embed.FlowRecord) { copy(b, wire.IP16(f.SrcIP)) }}, // sourceIPv6Address
	{29, 1, func(b []byte, f embed.FlowRecord) { b[0] = f.SrcMask }},             // sourceIPv6PrefixLength
	ieIngress, ieDstPort,
	{28, 16, func(b []byte, f embed.FlowRecord) { copy(b, wire.IP16(f.DstIP)) }}, // destinationIPv6Address
	{30, 1, func(b []byte, f embed.FlowRecord) { b[0] = f.DstMask }},             // destinationIPv6PrefixLength
	ieEgress,
	{62, 16, func(b []byte, f embed.FlowRecord) { copy(b, wire.IP16(f.NextHop)) }}, // ipNextHopIPv6Address
	ieSrcAS, ieDstAS, ieStart, ieEnd,
	{139, 2, icmpTC}, // icmpTypeCodeIPv6
}

// Encoder encodes FlowRecords as IPFIX messages: an IPv4 and an IPv6 template,
// an options template announcing the sampling rate, and data sets split to
// fit wire.MaxPayload. Safe for concurrent Encode calls.
type Encoder struct {
	domainID uint32
	vendor   *Vendor

	refreshInterval time.Duration
	refreshRecords  int

	mu            sync.Mutex
	seq           uint32 // data records sent so far = next message's sequence number
	sinceTemplate int    // flow records since the last template resend
	lastTemplate  time.Time
	sentTemplate  bool
	rate          uint32 // sampling rate last announced
}

// NewEncoder returns an IPFIX encoder for observation domain domainID. A
// non-nil vendor adds that vendor's enterprise IE to both templates.
func NewEncoder(domainID uint32, vendor *Vendor) *Encoder {
	return &Encoder{
		domainID:        domainID,
		vendor:          vendor,
		refreshInterval: wire.TemplateRefreshInterval,
		refreshRecords:  wire.TemplateRefreshRecords,
	}
}

func (e *Encoder) recordLen(tmpl []ie) int {
	n := 0
	for _, f := range tmpl {
		n += int(f.len)
	}
	if e.vendor != nil {
		n += int(e.vendor.Len)
	}
	return n
}

// templateSetLen is the template set's length: header, then per template its
// id, field count, and field specifiers (8 bytes for the vendor's, with PEN).
func (e *Encoder) templateSetLen() int {
	n := 4 + 4 + 4*len(templateV4) + 4 + 4*len(templateV6)
	if e.vendor != nil {
		n += 2 * 8
	}
	return n
}

// ResendTemplates makes the next message lead with the templates and sampling
// option, for when a message that carried them may never have arrived.
func (e *Encoder) ResendTemplates() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.sentTemplate = false
}

// Encode renders flows into IPFIX messages, each within wire.MaxPayload. A
// message carries one address family at one sampling rate, so a batch splits
// wherever either changes, keeping record order. Templates lead a message when
// a resend is due (first message, interval elapsed, or refreshRecords flow
// records since the last resend); the sampling option follows the templates
// and is also resent whenever the rate changes. Returns nil for an empty batch.
func (e *Encoder) Encode(now time.Time, flows []embed.FlowRecord) [][]byte {
	e.mu.Lock()
	defer e.mu.Unlock()
	var msgs [][]byte
	for len(flows) > 0 {
		msg, n := e.message(now, flows)
		msgs = append(msgs, msg)
		flows = flows[n:]
	}
	return msgs
}

// message encodes the next message from the head of flows and returns it with
// the number of flows it consumed (at least one).
func (e *Encoder) message(now time.Time, flows []embed.FlowRecord) ([]byte, int) {
	v6 := wire.IsIPv6(flows[0])
	rate := wire.SamplingRate(flows[0].SamplingRate)
	tmpl, tmplID := templateV4, templateIDv4
	if v6 {
		tmpl, tmplID = templateV6, templateIDv6
	}
	recLen := e.recordLen(tmpl)

	withTemplate := !e.sentTemplate ||
		(e.refreshInterval > 0 && now.Sub(e.lastTemplate) >= e.refreshInterval) ||
		(e.refreshRecords > 0 && e.sinceTemplate >= e.refreshRecords)
	withOptions := withTemplate || rate != e.rate

	head := headerLen // everything before the data set
	if withTemplate {
		head += e.templateSetLen() + optionsTemplateSetLen
	}
	if withOptions {
		head += optionsDataSetLen
	}
	n := 1
	for n < len(flows) && wire.IsIPv6(flows[n]) == v6 &&
		wire.SamplingRate(flows[n].SamplingRate) == rate &&
		head+wire.Pad4(4+(n+1)*recLen) <= wire.MaxPayload {
		n++
	}
	dataSetLen := wire.Pad4(4 + n*recLen)
	buf := make([]byte, head+dataSetLen)

	binary.BigEndian.PutUint16(buf[0:2], 10)
	binary.BigEndian.PutUint16(buf[2:4], wire.U16(len(buf)))
	binary.BigEndian.PutUint32(buf[4:8], wire.U32(now.Unix()))
	binary.BigEndian.PutUint32(buf[8:12], e.seq)
	binary.BigEndian.PutUint32(buf[12:16], e.domainID)

	off := headerLen
	if withTemplate {
		off += e.putTemplateSet(buf[off:])
		off += putOptionsTemplateSet(buf[off:])
		e.lastTemplate, e.sentTemplate, e.sinceTemplate = now, true, 0
	}
	if withOptions {
		off += e.putOptionsDataSet(buf[off:], rate)
		e.rate = rate
		e.seq++ // an options data record is a Data Record (RFC 7011 §3.1)
	}
	binary.BigEndian.PutUint16(buf[off:off+2], tmplID)
	binary.BigEndian.PutUint16(buf[off+2:off+4], wire.U16(dataSetLen))
	rec := off + 4
	for _, f := range flows[:n] {
		for _, fl := range tmpl {
			fl.put(buf[rec:rec+int(fl.len)], f)
			rec += int(fl.len)
		}
		if e.vendor != nil {
			e.vendor.put(buf[rec:rec+int(e.vendor.Len)], f)
			rec += int(e.vendor.Len)
		}
	}
	// Remaining bytes stay zero: set padding.

	e.seq += wire.U32(n)
	e.sinceTemplate += n
	return buf, n
}

// putTemplateSet writes the template set (IPv4 and IPv6 templates, each with
// the vendor IE when set) and returns its length.
func (e *Encoder) putTemplateSet(b []byte) int {
	total := e.templateSetLen()
	binary.BigEndian.PutUint16(b[0:2], setIDTemplate)
	binary.BigEndian.PutUint16(b[2:4], wire.U16(total))
	off := 4
	for _, t := range []struct {
		id     uint16
		fields []ie
	}{{templateIDv4, templateV4}, {templateIDv6, templateV6}} {
		count := len(t.fields)
		if e.vendor != nil {
			count++
		}
		binary.BigEndian.PutUint16(b[off:off+2], t.id)
		binary.BigEndian.PutUint16(b[off+2:off+4], wire.U16(count))
		off += 4
		for _, f := range t.fields {
			binary.BigEndian.PutUint16(b[off:off+2], f.id)
			binary.BigEndian.PutUint16(b[off+2:off+4], f.len)
			off += 4
		}
		if e.vendor != nil {
			binary.BigEndian.PutUint16(b[off:off+2], e.vendor.IEID|enterpriseBit)
			binary.BigEndian.PutUint16(b[off+2:off+4], e.vendor.Len)
			binary.BigEndian.PutUint32(b[off+4:off+8], e.vendor.PEN)
			off += 8
		}
	}
	return total
}

// putOptionsTemplateSet writes the sampling options template set. The
// sampling IEs are samplingInterval (34) and samplingAlgorithm (35): IANA
// marks them deprecated in favor of the PSAMP samplingPacketInterval (305) /
// samplingPacketSpace (306) pair, but 34 is the 1-in-N rate collectors read
// (goflow2, and so netflowreceiver, takes 305 as the rate itself, which would
// misreport a 1-in-N systematic sampler as 1-in-1).
func putOptionsTemplateSet(b []byte) int {
	for i, v := range []uint16{
		setIDOptionsTemplate, optionsTemplateSetLen,
		templateIDOptions, 3, 1, // template id, field count, scope field count
		149, 4, // observationDomainId (scope)
		34, 4, // samplingInterval
		35, 1, // samplingAlgorithm
	} {
		binary.BigEndian.PutUint16(b[i*2:], v)
	}
	return optionsTemplateSetLen
}

// putOptionsDataSet writes the options data set announcing a deterministic
// 1-in-rate sampling interval for this observation domain.
func (e *Encoder) putOptionsDataSet(b []byte, rate uint32) int {
	binary.BigEndian.PutUint16(b[0:2], templateIDOptions)
	binary.BigEndian.PutUint16(b[2:4], optionsDataSetLen)
	binary.BigEndian.PutUint32(b[4:8], e.domainID)
	binary.BigEndian.PutUint32(b[8:12], rate)
	b[12] = samplingDeterministic
	return optionsDataSetLen
}

func msSinceEpoch(t time.Time) uint64 {
	if t.IsZero() {
		return 0
	}
	return wire.U64(t.UnixMilli())
}

// transportPort is the IE 7/11 value. IANA defines those IEs only for
// port-bearing transports (TCP, UDP, SCTP, DCCP), so ICMP and ICMPv6 write 0
// and carry type/code in icmpTypeCodeIPv4/IPv6 instead.
func transportPort(proto uint8, port uint16) uint16 {
	if wire.IsICMP(proto) {
		return 0
	}
	return port
}
