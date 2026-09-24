// Package ipfix encodes blitz FlowRecords into IPFIX (RFC 7011) export
// messages. IPFIX is the IETF standardization of NetFlow v9: template-based,
// but with a 16-byte header, absolute millisecond timestamps, and 64-bit
// counters.
//
// A Vendor makes the export genuinely vendor-distinct on the wire: IPFIX is the
// only format that carries an enterprise-specific Information Element (a field
// whose type has the 0x8000 bit set, followed by a 4-byte Private Enterprise
// Number). A decoder such as netsampler/goflow2 reads that PEN back
// (DataField.PenProvided / .Pen), so an AppFlow (Citrix, PEN 5951) packet
// decodes distinctly from a jFlow (Juniper, 2636) or cflowd (Nokia, 6527) one.
package ipfix

import (
	"encoding/binary"
	"net"
	"sync"
	"time"

	"github.com/observiq/blitz/embed"
	"github.com/observiq/blitz/flow/wire"
)

const (
	headerLen            = 16
	templateID    uint16 = 256
	setIDTemplate uint16 = 2

	// enterpriseBit marks a field type as enterprise-specific; the 4-byte PEN
	// follows the field in the template (RFC 7011 §3.2).
	enterpriseBit uint16 = 0x8000

	// DefaultRefreshInterval and DefaultRefreshRecords are the template resend
	// cadence: whichever comes first.
	DefaultRefreshInterval = 60 * time.Second
	DefaultRefreshRecords  = 256
)

// Vendor is an enterprise-specific IE that stamps the export as a vendor's
// IPFIX derivative. PEN is the IANA Private Enterprise Number; IEID is the
// vendor's element id (without the enterprise bit).
type Vendor struct {
	PEN  uint32
	IEID uint16
}

type ie struct {
	id  uint16
	len uint16
}

// template is the fixed IPv4 IPFIX template (IANA information elements).
var template = []ie{
	{1, 8},   // octetDeltaCount
	{2, 8},   // packetDeltaCount
	{4, 1},   // protocolIdentifier
	{5, 1},   // ipClassOfService
	{6, 1},   // tcpControlBits
	{7, 2},   // sourceTransportPort
	{8, 4},   // sourceIPv4Address
	{9, 1},   // sourceIPv4PrefixLength
	{10, 4},  // ingressInterface
	{11, 2},  // destinationTransportPort
	{12, 4},  // destinationIPv4Address
	{13, 1},  // destinationIPv4PrefixLength
	{14, 4},  // egressInterface
	{15, 4},  // ipNextHopIPv4Address
	{16, 4},  // bgpSourceAsNumber
	{17, 4},  // bgpDestinationAsNumber
	{152, 8}, // flowStartMilliseconds
	{153, 8}, // flowEndMilliseconds
}

// vendorIELen is the byte width of the vendor enterprise IE value.
const vendorIELen = 4

func recordLen(vendor *Vendor) int {
	n := 0
	for _, f := range template {
		n += int(f.len)
	}
	if vendor != nil {
		n += vendorIELen
	}
	return n
}

// Encoder encodes FlowRecords as IPFIX messages with periodic template
// resends. Safe for concurrent Encode calls.
type Encoder struct {
	domainID uint32
	vendor   *Vendor

	refreshInterval time.Duration
	refreshRecords  int

	mu           sync.Mutex
	seqRecords   uint32 // data records sent so far = next message's sequence number
	lastTemplate time.Time
	firstDone    bool
}

// NewEncoder returns an IPFIX encoder. A non-nil vendor adds that vendor's
// enterprise IE to every template and record.
func NewEncoder(domainID uint32, vendor *Vendor) *Encoder {
	return &Encoder{
		domainID:        domainID,
		vendor:          vendor,
		refreshInterval: DefaultRefreshInterval,
		refreshRecords:  DefaultRefreshRecords,
	}
}

// Encode renders flows into a single IPFIX message, prepending a template set
// when a resend is due. Returns nil for an empty batch.
func (e *Encoder) Encode(now time.Time, flows []embed.FlowRecord) [][]byte {
	if len(flows) == 0 {
		return nil
	}
	e.mu.Lock()
	defer e.mu.Unlock()

	cumBefore := wire.U64(e.seqRecords)
	cumAfter := cumBefore + wire.U64(len(flows))
	withTemplate := !e.firstDone ||
		(e.refreshInterval > 0 && !e.lastTemplate.IsZero() && now.Sub(e.lastTemplate) >= e.refreshInterval) ||
		(e.refreshRecords > 0 && cumBefore/wire.U64(e.refreshRecords) != cumAfter/wire.U64(e.refreshRecords))

	recLen := recordLen(e.vendor)
	dataSetLen := 4 + len(flows)*recLen
	if pad := dataSetLen % 4; pad != 0 {
		dataSetLen += 4 - pad
	}
	tmplSetLen := 0
	if withTemplate {
		tmplSetLen = e.templateSetLen()
		e.lastTemplate = now
	}

	buf := make([]byte, headerLen+tmplSetLen+dataSetLen)
	binary.BigEndian.PutUint16(buf[0:2], 10)
	binary.BigEndian.PutUint16(buf[2:4], wire.U16(len(buf)))
	binary.BigEndian.PutUint32(buf[4:8], wire.U32(now.Unix()))
	binary.BigEndian.PutUint32(buf[8:12], e.seqRecords)
	binary.BigEndian.PutUint32(buf[12:16], e.domainID)

	off := headerLen
	if withTemplate {
		off += e.putTemplateSet(buf[off:])
	}
	e.putDataSet(buf[off:], flows, recLen, dataSetLen)

	e.firstDone = true
	e.seqRecords = wire.U32(cumAfter)
	return [][]byte{buf}
}

// templateSetLen is the byte length of the template set including the optional
// vendor enterprise IE (which carries a trailing 4-byte PEN).
func (e *Encoder) templateSetLen() int {
	n := 4 + 4 + len(template)*4
	if e.vendor != nil {
		n += 4 + 4 // enterprise IE: (type,len) + PEN
	}
	return n
}

func (e *Encoder) putTemplateSet(b []byte) int {
	total := e.templateSetLen()
	fieldCount := len(template)
	if e.vendor != nil {
		fieldCount++
	}
	binary.BigEndian.PutUint16(b[0:2], setIDTemplate)
	binary.BigEndian.PutUint16(b[2:4], wire.U16(total))
	binary.BigEndian.PutUint16(b[4:6], templateID)
	binary.BigEndian.PutUint16(b[6:8], wire.U16(fieldCount))
	off := 8
	for _, f := range template {
		binary.BigEndian.PutUint16(b[off:off+2], f.id)
		binary.BigEndian.PutUint16(b[off+2:off+4], f.len)
		off += 4
	}
	if e.vendor != nil {
		binary.BigEndian.PutUint16(b[off:off+2], e.vendor.IEID|enterpriseBit)
		binary.BigEndian.PutUint16(b[off+2:off+4], vendorIELen)
		binary.BigEndian.PutUint32(b[off+4:off+8], e.vendor.PEN)
		off += 8
	}
	return total
}

func (e *Encoder) putDataSet(b []byte, flows []embed.FlowRecord, recLen, setLen int) {
	binary.BigEndian.PutUint16(b[0:2], templateID)
	binary.BigEndian.PutUint16(b[2:4], wire.U16(setLen))
	off := 4
	for _, f := range flows {
		e.putRecord(b[off:off+recLen], f)
		off += recLen
	}
}

func (e *Encoder) putRecord(b []byte, f embed.FlowRecord) {
	binary.BigEndian.PutUint64(b[0:8], f.Bytes)
	binary.BigEndian.PutUint64(b[8:16], f.Packets)
	b[16] = f.Protocol
	b[17] = f.TOS
	b[18] = f.TCPFlags
	binary.BigEndian.PutUint16(b[19:21], f.SrcPort)
	copy(b[21:25], ip4(f.SrcIP))
	b[25] = f.SrcMask
	binary.BigEndian.PutUint32(b[26:30], f.InputIface)
	binary.BigEndian.PutUint16(b[30:32], f.DstPort)
	copy(b[32:36], ip4(f.DstIP))
	b[36] = f.DstMask
	binary.BigEndian.PutUint32(b[37:41], f.OutputIface)
	copy(b[41:45], ip4(f.NextHop))
	binary.BigEndian.PutUint32(b[45:49], f.SrcAS)
	binary.BigEndian.PutUint32(b[49:53], f.DstAS)
	binary.BigEndian.PutUint64(b[53:61], msSinceEpoch(f.StartTime))
	binary.BigEndian.PutUint64(b[61:69], msSinceEpoch(f.EndTime))
	if e.vendor != nil {
		// Vendor IE value: the PEN echoed, so the decoded value is self-describing.
		binary.BigEndian.PutUint32(b[69:73], e.vendor.PEN)
	}
}

func msSinceEpoch(t time.Time) uint64 {
	if t.IsZero() {
		return 0
	}
	return wire.U64(t.UnixMilli())
}

func ip4(ip net.IP) []byte {
	if v4 := ip.To4(); v4 != nil {
		return v4
	}
	return []byte{0, 0, 0, 0}
}
