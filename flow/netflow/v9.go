package netflow

import (
	"encoding/binary"
	"sync"
	"time"

	"github.com/observiq/blitz/embed"
	"github.com/observiq/blitz/flow/wire"
)

// NetFlow v9 wire constants (RFC 3954).
const (
	v9HeaderLen = 20

	flowSetTemplate        uint16 = 0 // template flowset id
	flowSetOptionsTemplate uint16 = 1 // options template flowset id

	templateIDv4      uint16 = 256
	templateIDv6      uint16 = 257
	templateIDOptions uint16 = 258

	scopeSystem uint16 = 1 // options scope field type "System"

	samplingDeterministic uint8 = 0x01 // SAMPLING_ALGORITHM deterministic

	v9TemplateCount = 2 // template records per resend: IPv4 + IPv6

	// Options template flowset (RFC 3954 §6.1): header, template id, scope
	// length, option length, one scope field (System, 4 bytes) and two option
	// fields (SAMPLING_INTERVAL 34/4, SAMPLING_ALGORITHM 35/1), padded to 4.
	v9OptionsTemplateLen = 24
	// Options data flowset: header + one record (scope value 4, interval 4,
	// algorithm 1), padded to 4.
	v9OptionsDataLen = 16
)

// v9Field is one template field: a NetFlow v9 field type, its byte length,
// and how to write it from a flow.
type v9Field struct {
	typ uint16
	len uint16
	put func(b []byte, e *V9Encoder, f embed.FlowRecord)
}

// Fields shared by the IPv4 and IPv6 templates.
var (
	v9InBytes  = v9Field{1, 4, func(b []byte, _ *V9Encoder, f embed.FlowRecord) { binary.BigEndian.PutUint32(b, wire.U32(f.Bytes)) }}
	v9InPkts   = v9Field{2, 4, func(b []byte, _ *V9Encoder, f embed.FlowRecord) { binary.BigEndian.PutUint32(b, wire.U32(f.Packets)) }}
	v9Protocol = v9Field{4, 1, func(b []byte, _ *V9Encoder, f embed.FlowRecord) { b[0] = f.Protocol }}
	v9TOS      = v9Field{5, 1, func(b []byte, _ *V9Encoder, f embed.FlowRecord) { b[0] = f.TOS }}
	v9TCPFlags = v9Field{6, 1, func(b []byte, _ *V9Encoder, f embed.FlowRecord) { b[0] = f.TCPFlags }}
	v9SrcPort  = v9Field{7, 2, func(b []byte, _ *V9Encoder, f embed.FlowRecord) { binary.BigEndian.PutUint16(b, f.SrcPort) }}
	v9InSNMP   = v9Field{10, 2, func(b []byte, _ *V9Encoder, f embed.FlowRecord) {
		binary.BigEndian.PutUint16(b, wire.U16(f.InputIface))
	}}
	v9DstPort = v9Field{11, 2, func(b []byte, _ *V9Encoder, f embed.FlowRecord) { binary.BigEndian.PutUint16(b, f.DstPort) }}
	v9OutSNMP = v9Field{14, 2, func(b []byte, _ *V9Encoder, f embed.FlowRecord) {
		binary.BigEndian.PutUint16(b, wire.U16(f.OutputIface))
	}}
	// SRC_AS/DST_AS use the 4-byte width RFC 3954 allows, so 32-bit ASNs survive.
	v9SrcAS  = v9Field{16, 4, func(b []byte, _ *V9Encoder, f embed.FlowRecord) { binary.BigEndian.PutUint32(b, f.SrcAS) }}
	v9DstAS  = v9Field{17, 4, func(b []byte, _ *V9Encoder, f embed.FlowRecord) { binary.BigEndian.PutUint32(b, f.DstAS) }}
	v9LastSw = v9Field{21, 4, func(b []byte, e *V9Encoder, f embed.FlowRecord) {
		binary.BigEndian.PutUint32(b, wire.UptimeMS(e.boot, f.EndTime))
	}}
	v9FirstSw = v9Field{22, 4, func(b []byte, e *V9Encoder, f embed.FlowRecord) {
		binary.BigEndian.PutUint32(b, wire.UptimeMS(e.boot, f.StartTime))
	}}
	// ICMP_TYPE: type*256+code for ICMP/ICMPv6, 0 otherwise.
	v9ICMPType = v9Field{32, 2, func(b []byte, _ *V9Encoder, f embed.FlowRecord) {
		binary.BigEndian.PutUint16(b, wire.ICMPTypeCode(f.Protocol, f.DstPort))
	}}
)

// v9TemplateV4 is the IPv4 template (id 256). Order defines the record layout.
var v9TemplateV4 = []v9Field{
	v9InBytes, v9InPkts, v9Protocol, v9TOS, v9TCPFlags, v9SrcPort,
	{8, 4, func(b []byte, _ *V9Encoder, f embed.FlowRecord) { copy(b, wire.IP4(f.SrcIP)) }}, // IPV4_SRC_ADDR
	{9, 1, func(b []byte, _ *V9Encoder, f embed.FlowRecord) { b[0] = f.SrcMask }},           // SRC_MASK
	v9InSNMP, v9DstPort,
	{12, 4, func(b []byte, _ *V9Encoder, f embed.FlowRecord) { copy(b, wire.IP4(f.DstIP)) }}, // IPV4_DST_ADDR
	{13, 1, func(b []byte, _ *V9Encoder, f embed.FlowRecord) { b[0] = f.DstMask }},           // DST_MASK
	v9OutSNMP,
	{15, 4, func(b []byte, _ *V9Encoder, f embed.FlowRecord) { copy(b, wire.IP4(f.NextHop)) }}, // IPV4_NEXT_HOP
	v9SrcAS, v9DstAS, v9LastSw, v9FirstSw, v9ICMPType,
}

// v9TemplateV6 is the IPv6 template (id 257): the IPv4 layout with the RFC 3954
// IPv6 address, mask, and next-hop fields in place of the IPv4 ones.
var v9TemplateV6 = []v9Field{
	v9InBytes, v9InPkts, v9Protocol, v9TOS, v9TCPFlags, v9SrcPort,
	{27, 16, func(b []byte, _ *V9Encoder, f embed.FlowRecord) { copy(b, wire.IP16(f.SrcIP)) }}, // IPV6_SRC_ADDR
	{29, 1, func(b []byte, _ *V9Encoder, f embed.FlowRecord) { b[0] = f.SrcMask }},             // IPV6_SRC_MASK
	v9InSNMP, v9DstPort,
	{28, 16, func(b []byte, _ *V9Encoder, f embed.FlowRecord) { copy(b, wire.IP16(f.DstIP)) }}, // IPV6_DST_ADDR
	{30, 1, func(b []byte, _ *V9Encoder, f embed.FlowRecord) { b[0] = f.DstMask }},             // IPV6_DST_MASK
	v9OutSNMP,
	{62, 16, func(b []byte, _ *V9Encoder, f embed.FlowRecord) { copy(b, wire.IP16(f.NextHop)) }}, // IPV6_NEXT_HOP
	v9SrcAS, v9DstAS, v9LastSw, v9FirstSw, v9ICMPType,
}

func v9RecordLen(tmpl []v9Field) int {
	n := 0
	for _, f := range tmpl {
		n += int(f.len)
	}
	return n
}

var (
	v9RecLenV4 = v9RecordLen(v9TemplateV4)
	v9RecLenV6 = v9RecordLen(v9TemplateV6)

	// Template flowset: header, then per template its id, field count, and a
	// (type, length) pair per field.
	v9TemplateFlowSetLen = 4 + 4 + 4*len(v9TemplateV4) + 4 + 4*len(v9TemplateV6)
)

// V9Encoder encodes FlowRecords as NetFlow v9 packets: an IPv4 and an IPv6
// template, an options template announcing the sampling rate, and data
// flowsets split to fit wire.MaxPayload. Safe for concurrent Encode calls.
type V9Encoder struct {
	boot     time.Time
	sourceID uint32

	refreshInterval time.Duration
	refreshRecords  int

	mu            sync.Mutex
	pktSeq        uint32
	sinceTemplate int // data records since the last template resend
	lastTemplate  time.Time
	sentTemplate  bool
	rate          uint32 // sampling rate last announced
}

// NewV9Encoder returns a v9 encoder with the default template refresh cadence.
func NewV9Encoder(boot time.Time, sourceID uint32) *V9Encoder {
	return &V9Encoder{
		boot:            boot,
		sourceID:        sourceID,
		refreshInterval: wire.TemplateRefreshInterval,
		refreshRecords:  wire.TemplateRefreshRecords,
	}
}

// ResendTemplates makes the next packet lead with the templates and sampling
// option, for when a packet that carried them may never have arrived.
func (e *V9Encoder) ResendTemplates() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.sentTemplate = false
}

// Encode renders flows into NetFlow v9 packets, each within wire.MaxPayload.
// A packet carries one address family at one sampling rate, so a batch splits
// wherever either changes, keeping record order. Templates lead a packet when
// a resend is due (first packet, interval elapsed, or refreshRecords data
// records since the last resend); the sampling option follows the templates
// and is also resent whenever the rate changes. Returns nil for an empty batch.
func (e *V9Encoder) Encode(now time.Time, flows []embed.FlowRecord) [][]byte {
	e.mu.Lock()
	defer e.mu.Unlock()
	var pkts [][]byte
	for len(flows) > 0 {
		pkt, n := e.packet(now, flows)
		pkts = append(pkts, pkt)
		flows = flows[n:]
	}
	return pkts
}

// packet encodes the next packet from the head of flows and returns it with
// the number of flows it consumed (at least one).
func (e *V9Encoder) packet(now time.Time, flows []embed.FlowRecord) ([]byte, int) {
	v6 := wire.IsIPv6(flows[0])
	rate := wire.SamplingRate(flows[0].SamplingRate)
	tmpl, recLen, tmplID := v9TemplateV4, v9RecLenV4, templateIDv4
	if v6 {
		tmpl, recLen, tmplID = v9TemplateV6, v9RecLenV6, templateIDv6
	}

	withTemplate := !e.sentTemplate ||
		(e.refreshInterval > 0 && now.Sub(e.lastTemplate) >= e.refreshInterval) ||
		(e.refreshRecords > 0 && e.sinceTemplate >= e.refreshRecords)
	withOptions := withTemplate || rate != e.rate

	head := v9HeaderLen // everything before the data flowset
	count := 0          // non-data records (templates, options)
	if withTemplate {
		head += v9TemplateFlowSetLen + v9OptionsTemplateLen
		count += v9TemplateCount + 1
	}
	if withOptions {
		head += v9OptionsDataLen
		count++
	}
	n := 1
	for n < len(flows) && wire.IsIPv6(flows[n]) == v6 &&
		wire.SamplingRate(flows[n].SamplingRate) == rate &&
		head+wire.Pad4(4+(n+1)*recLen) <= wire.MaxPayload {
		n++
	}
	dataLen := wire.Pad4(4 + n*recLen)
	buf := make([]byte, head+dataLen)

	binary.BigEndian.PutUint16(buf[0:2], 9)
	binary.BigEndian.PutUint16(buf[2:4], wire.U16(count+n))
	binary.BigEndian.PutUint32(buf[4:8], wire.UptimeMS(e.boot, now))
	binary.BigEndian.PutUint32(buf[8:12], wire.U32(now.Unix()))
	binary.BigEndian.PutUint32(buf[12:16], e.pktSeq)
	binary.BigEndian.PutUint32(buf[16:20], e.sourceID)

	off := v9HeaderLen
	if withTemplate {
		off += putV9Templates(buf[off:])
		off += putV9OptionsTemplate(buf[off:])
		e.lastTemplate, e.sentTemplate, e.sinceTemplate = now, true, 0
	}
	if withOptions {
		off += e.putV9OptionsData(buf[off:], rate)
		e.rate = rate
	}
	binary.BigEndian.PutUint16(buf[off:off+2], tmplID)
	binary.BigEndian.PutUint16(buf[off+2:off+4], wire.U16(dataLen))
	rec := off + 4
	for _, f := range flows[:n] {
		for _, fl := range tmpl {
			fl.put(buf[rec:rec+int(fl.len)], e, f)
			rec += int(fl.len)
		}
	}
	// Remaining bytes stay zero: flowset padding.

	e.pktSeq++
	e.sinceTemplate += n
	return buf, n
}

// putV9Templates writes the template flowset (IPv4 and IPv6 templates) and
// returns its length.
func putV9Templates(b []byte) int {
	binary.BigEndian.PutUint16(b[0:2], flowSetTemplate)
	binary.BigEndian.PutUint16(b[2:4], wire.U16(v9TemplateFlowSetLen))
	off := 4
	for _, t := range []struct {
		id     uint16
		fields []v9Field
	}{{templateIDv4, v9TemplateV4}, {templateIDv6, v9TemplateV6}} {
		binary.BigEndian.PutUint16(b[off:off+2], t.id)
		binary.BigEndian.PutUint16(b[off+2:off+4], wire.U16(len(t.fields)))
		off += 4
		for _, f := range t.fields {
			binary.BigEndian.PutUint16(b[off:off+2], f.typ)
			binary.BigEndian.PutUint16(b[off+2:off+4], f.len)
			off += 4
		}
	}
	return v9TemplateFlowSetLen
}

// putV9OptionsTemplate writes the sampling options template flowset.
func putV9OptionsTemplate(b []byte) int {
	for i, v := range []uint16{
		flowSetOptionsTemplate, v9OptionsTemplateLen,
		templateIDOptions, 4, 8, // template id, scope length, option length
		scopeSystem, 4,
		34, 4, // SAMPLING_INTERVAL
		35, 1, // SAMPLING_ALGORITHM
	} {
		binary.BigEndian.PutUint16(b[i*2:], v)
	}
	return v9OptionsTemplateLen
}

// putV9OptionsData writes the options data flowset announcing a deterministic
// 1-in-rate sampling interval for this exporter (scope System = source id).
func (e *V9Encoder) putV9OptionsData(b []byte, rate uint32) int {
	binary.BigEndian.PutUint16(b[0:2], templateIDOptions)
	binary.BigEndian.PutUint16(b[2:4], v9OptionsDataLen)
	binary.BigEndian.PutUint32(b[4:8], e.sourceID)
	binary.BigEndian.PutUint32(b[8:12], rate)
	b[12] = samplingDeterministic
	return v9OptionsDataLen
}
