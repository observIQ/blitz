// Package netflow encodes blitz FlowRecords into Cisco NetFlow export
// packets. V5 is the fixed-layout legacy format (IPv4 only, 30 flows per
// packet); V9 is the template-based format (see v9.go).
package netflow

import (
	"encoding/binary"
	"net"
	"sync/atomic"
	"time"

	"github.com/observiq/blitz/embed"
	"github.com/observiq/blitz/flow/wire"
)

const (
	v5HeaderLen = 24
	v5RecordLen = 48
	v5MaxFlows  = 30
)

// V5Encoder encodes FlowRecords as NetFlow v5 packets. It owns the export
// boot time (for switch-uptime timestamps) and a flow sequence counter.
// Safe for concurrent Encode calls.
type V5Encoder struct {
	boot       time.Time
	seq        atomic.Uint32
	engineType uint8
	engineID   uint8
	sampling   uint16
}

// NewV5Encoder returns a v5 encoder whose switch-uptime clock starts at boot.
func NewV5Encoder(boot time.Time) *V5Encoder {
	return &V5Encoder{boot: boot}
}

// Encode renders flows into one or more NetFlow v5 packets (max 30 flows
// each), stamped at now. Returns nil for an empty batch.
func (e *V5Encoder) Encode(now time.Time, flows []embed.FlowRecord) [][]byte {
	if len(flows) == 0 {
		return nil
	}
	var pkts [][]byte
	for start := 0; start < len(flows); start += v5MaxFlows {
		end := start + v5MaxFlows
		if end > len(flows) {
			end = len(flows)
		}
		pkts = append(pkts, e.packet(now, flows[start:end]))
	}
	return pkts
}

func (e *V5Encoder) packet(now time.Time, flows []embed.FlowRecord) []byte {
	count := len(flows)
	buf := make([]byte, v5HeaderLen+count*v5RecordLen)

	uptimeMS := wire.U32(now.Sub(e.boot).Milliseconds())
	seq := e.seq.Add(wire.U32(count)) - wire.U32(count) // sequence of first record

	binary.BigEndian.PutUint16(buf[0:2], 5)
	binary.BigEndian.PutUint16(buf[2:4], wire.U16(count))
	binary.BigEndian.PutUint32(buf[4:8], uptimeMS)
	binary.BigEndian.PutUint32(buf[8:12], wire.U32(now.Unix()))
	binary.BigEndian.PutUint32(buf[12:16], wire.U32(now.Nanosecond()))
	binary.BigEndian.PutUint32(buf[16:20], seq)
	buf[20] = e.engineType
	buf[21] = e.engineID
	binary.BigEndian.PutUint16(buf[22:24], e.sampling)

	for i, f := range flows {
		e.putRecord(buf[v5HeaderLen+i*v5RecordLen:], f)
	}
	return buf
}

func (e *V5Encoder) putRecord(b []byte, f embed.FlowRecord) {
	copy(b[0:4], ip4(f.SrcIP))
	copy(b[4:8], ip4(f.DstIP))
	copy(b[8:12], ip4(f.NextHop))
	binary.BigEndian.PutUint16(b[12:14], wire.U16(f.InputIface))
	binary.BigEndian.PutUint16(b[14:16], wire.U16(f.OutputIface))
	binary.BigEndian.PutUint32(b[16:20], wire.U32(f.Packets))
	binary.BigEndian.PutUint32(b[20:24], wire.U32(f.Bytes))
	binary.BigEndian.PutUint32(b[24:28], e.uptimeMS(f.StartTime))
	binary.BigEndian.PutUint32(b[28:32], e.uptimeMS(f.EndTime))
	binary.BigEndian.PutUint16(b[32:34], f.SrcPort)
	binary.BigEndian.PutUint16(b[34:36], f.DstPort)
	// b[36] pad1
	b[37] = f.TCPFlags
	b[38] = f.Protocol
	b[39] = f.TOS
	binary.BigEndian.PutUint16(b[40:42], wire.U16(f.SrcAS))
	binary.BigEndian.PutUint16(b[42:44], wire.U16(f.DstAS))
	b[44] = f.SrcMask
	b[45] = f.DstMask
	// b[46:48] pad2
}

func (e *V5Encoder) uptimeMS(t time.Time) uint32 {
	if t.IsZero() {
		return 0
	}
	return wire.U32(t.Sub(e.boot).Milliseconds())
}

// ip4 returns the 4-byte IPv4 form of ip, or zeros if ip is nil/not v4.
func ip4(ip net.IP) []byte {
	if v4 := ip.To4(); v4 != nil {
		return v4
	}
	return []byte{0, 0, 0, 0}
}
