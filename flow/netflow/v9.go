package netflow

import (
	"encoding/binary"
	"sync"
	"time"

	"github.com/observiq/blitz/embed"
	"github.com/observiq/blitz/flow/wire"
)

const (
	v9HeaderLen        = 20
	templateID  uint16 = 256

	// DefaultRefreshInterval and DefaultRefreshRecords are the NetFlow v9 /
	// IPFIX template resend cadence: whichever comes first.
	DefaultRefreshInterval = 60 * time.Second
	DefaultRefreshRecords  = 256
)

// v9Field is one template field: NetFlow field type and its byte length.
type v9Field struct {
	typ uint16
	len uint16
}

// v9Template is the fixed IPv4 template both the v9 and IPFIX encoders share
// (IPFIX reuses the same field type IDs). Order defines the data-record layout.
var v9Template = []v9Field{
	{1, 4},  // IN_BYTES
	{2, 4},  // IN_PKTS
	{4, 1},  // PROTOCOL
	{5, 1},  // TOS
	{6, 1},  // TCP_FLAGS
	{7, 2},  // L4_SRC_PORT
	{8, 4},  // IPV4_SRC_ADDR
	{9, 1},  // SRC_MASK
	{10, 2}, // INPUT_SNMP
	{11, 2}, // L4_DST_PORT
	{12, 4}, // IPV4_DST_ADDR
	{13, 1}, // DST_MASK
	{14, 2}, // OUTPUT_SNMP
	{15, 4}, // IPV4_NEXT_HOP
	{16, 2}, // SRC_AS
	{17, 2}, // DST_AS
	{21, 4}, // LAST_SWITCHED
	{22, 4}, // FIRST_SWITCHED
}

func recordLen(tmpl []v9Field) int {
	n := 0
	for _, f := range tmpl {
		n += int(f.len)
	}
	return n
}

// V9Encoder encodes FlowRecords as NetFlow v9 packets with periodic template
// resends. Safe for concurrent Encode calls.
//
// NetFlow v9 has no enterprise-PEN mechanism (that is IPFIX-only), so a v9
// vendor flavor is distinguished by a proprietary field type in the template:
// vendorField, when non-zero, appends one vendor-specific 4-byte field that a
// decoder surfaces by its type id. This is the genuine wire difference for
// v9-based flavors such as NetStream (Huawei) and rFlow (Redback).
type V9Encoder struct {
	boot        time.Time
	sourceID    uint32
	vendorField uint16

	refreshInterval time.Duration
	refreshRecords  int

	mu           sync.Mutex
	pktSeq       uint32
	totalRecords uint64
	lastTemplate time.Time
}

// NewV9Encoder returns a v9 encoder with the default template refresh cadence.
// A non-zero vendorField appends that vendor-specific field to every template
// and record.
func NewV9Encoder(boot time.Time, sourceID uint32, vendorField uint16) *V9Encoder {
	return &V9Encoder{
		boot:            boot,
		sourceID:        sourceID,
		vendorField:     vendorField,
		refreshInterval: DefaultRefreshInterval,
		refreshRecords:  DefaultRefreshRecords,
	}
}

// vendorLen is the byte width of the v9 vendor-specific field value.
const vendorLen = 4

func (e *V9Encoder) recLen() int {
	n := recordLen(v9Template)
	if e.vendorField != 0 {
		n += vendorLen
	}
	return n
}

func (e *V9Encoder) templateLen() int {
	n := 4 + 4 + len(v9Template)*4
	if e.vendorField != 0 {
		n += 4
	}
	return n
}

// Encode renders flows into a single NetFlow v9 packet, prepending a template
// flowset when a resend is due (first packet, interval elapsed, or the record
// counter crossed a refresh boundary). Returns nil for an empty batch.
func (e *V9Encoder) Encode(now time.Time, flows []embed.FlowRecord) [][]byte {
	if len(flows) == 0 {
		return nil
	}
	e.mu.Lock()
	defer e.mu.Unlock()

	cumBefore := e.totalRecords
	cumAfter := cumBefore + wire.U64(len(flows))
	withTemplate := e.pktSeq == 0 ||
		(e.refreshInterval > 0 && !e.lastTemplate.IsZero() && now.Sub(e.lastTemplate) >= e.refreshInterval) ||
		(e.refreshRecords > 0 && cumBefore/wire.U64(e.refreshRecords) != cumAfter/wire.U64(e.refreshRecords))

	recLen := e.recLen()
	dataLen := 4 + len(flows)*recLen
	if pad := dataLen % 4; pad != 0 {
		dataLen += 4 - pad
	}
	tmplLen := 0
	if withTemplate {
		tmplLen = e.templateLen() // flowset hdr + template hdr + fields (+ vendor field)
		e.lastTemplate = now
	}

	buf := make([]byte, v9HeaderLen+tmplLen+dataLen)
	records := len(flows)
	if withTemplate {
		records++
	}

	binary.BigEndian.PutUint16(buf[0:2], 9)
	binary.BigEndian.PutUint16(buf[2:4], wire.U16(records))
	binary.BigEndian.PutUint32(buf[4:8], wire.U32(now.Sub(e.boot).Milliseconds()))
	binary.BigEndian.PutUint32(buf[8:12], wire.U32(now.Unix()))
	binary.BigEndian.PutUint32(buf[12:16], e.pktSeq)
	binary.BigEndian.PutUint32(buf[16:20], e.sourceID)

	off := v9HeaderLen
	if withTemplate {
		off += e.putV9Template(buf[off:])
	}
	e.putV9DataFlowSet(buf[off:], flows, recLen, dataLen)

	e.pktSeq++
	e.totalRecords = cumAfter
	return [][]byte{buf}
}

// putV9Template writes the template flowset and returns its length.
func (e *V9Encoder) putV9Template(b []byte) int {
	total := e.templateLen()
	fieldCount := len(v9Template)
	if e.vendorField != 0 {
		fieldCount++
	}
	binary.BigEndian.PutUint16(b[0:2], 0) // flowset_id 0 = template
	binary.BigEndian.PutUint16(b[2:4], wire.U16(total))
	binary.BigEndian.PutUint16(b[4:6], templateID)
	binary.BigEndian.PutUint16(b[6:8], wire.U16(fieldCount))
	off := 8
	for _, f := range v9Template {
		binary.BigEndian.PutUint16(b[off:off+2], f.typ)
		binary.BigEndian.PutUint16(b[off+2:off+4], f.len)
		off += 4
	}
	if e.vendorField != 0 {
		binary.BigEndian.PutUint16(b[off:off+2], e.vendorField)
		binary.BigEndian.PutUint16(b[off+2:off+4], vendorLen)
		off += 4
	}
	return total
}

func (e *V9Encoder) putV9DataFlowSet(b []byte, flows []embed.FlowRecord, recLen, dataLen int) {
	binary.BigEndian.PutUint16(b[0:2], templateID)
	binary.BigEndian.PutUint16(b[2:4], wire.U16(dataLen))
	off := 4
	for _, f := range flows {
		e.putV9Record(b[off:off+recLen], f)
		off += recLen
	}
	// remaining bytes stay zero = padding
}

func (e *V9Encoder) putV9Record(b []byte, f embed.FlowRecord) {
	boot := e.boot
	binary.BigEndian.PutUint32(b[0:4], wire.U32(f.Bytes))
	binary.BigEndian.PutUint32(b[4:8], wire.U32(f.Packets))
	b[8] = f.Protocol
	b[9] = f.TOS
	b[10] = f.TCPFlags
	binary.BigEndian.PutUint16(b[11:13], f.SrcPort)
	copy(b[13:17], ip4(f.SrcIP))
	b[17] = f.SrcMask
	binary.BigEndian.PutUint16(b[18:20], wire.U16(f.InputIface))
	binary.BigEndian.PutUint16(b[20:22], f.DstPort)
	copy(b[22:26], ip4(f.DstIP))
	b[26] = f.DstMask
	binary.BigEndian.PutUint16(b[27:29], wire.U16(f.OutputIface))
	copy(b[29:33], ip4(f.NextHop))
	binary.BigEndian.PutUint16(b[33:35], wire.U16(f.SrcAS))
	binary.BigEndian.PutUint16(b[35:37], wire.U16(f.DstAS))
	binary.BigEndian.PutUint32(b[37:41], uptime(boot, f.EndTime))
	binary.BigEndian.PutUint32(b[41:45], uptime(boot, f.StartTime))
	if e.vendorField != 0 {
		// Vendor field value: the vendor field id echoed, so the record is
		// self-describing and the field's presence is verifiable.
		binary.BigEndian.PutUint32(b[45:49], wire.U32(e.vendorField))
	}
}

func uptime(boot, t time.Time) uint32 {
	if t.IsZero() {
		return 0
	}
	return wire.U32(t.Sub(boot).Milliseconds())
}
