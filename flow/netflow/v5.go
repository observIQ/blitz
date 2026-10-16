// Package netflow encodes blitz FlowRecords into Cisco NetFlow export
// packets. V5 is the fixed-layout legacy format (IPv4 only, at most 30 flows
// per packet, of which blitz sends up to 29; see v5MaxFlows); V9 is the
// template-based format (see v9.go).
package netflow

import (
	"encoding/binary"
	"errors"
	"sync/atomic"
	"time"

	"github.com/observiq/blitz/embed"
	"github.com/observiq/blitz/flow/wire"
)

const (
	v5HeaderLen = 24
	v5RecordLen = 48
	// v5MaxFlows is the protocol's 30 records per packet, capped so a packet
	// fits wire.MaxPayload (30 records need 1464 bytes, over the IPv6 budget).
	v5MaxFlows = min(30, (wire.MaxPayload-v5HeaderLen)/v5RecordLen)

	// v5SamplingPacketInterval is sampling mode 01 (packet interval) in the
	// sampling_interval field's top 2 bits; the low 14 bits carry the rate.
	v5SamplingPacketInterval = 1 << 14
	v5MaxSamplingRate        = 1<<14 - 1
)

// ErrV5IPv6 reports an IPv6 flow sent to the IPv4-only NetFlow v5 format.
var ErrV5IPv6 = errors.New("netflow v5 carries IPv4 flows only; use netflow-v9, ipfix, or sflow for IPv6")

// V5Encoder encodes FlowRecords as NetFlow v5 packets. It owns the export
// boot time (for switch-uptime timestamps) and a flow sequence counter.
// Safe for concurrent Encode calls.
type V5Encoder struct {
	boot       time.Time
	seq        atomic.Uint32
	engineType uint8
	engineID   uint8
}

// NewV5Encoder returns a v5 encoder whose switch-uptime clock starts at boot.
func NewV5Encoder(boot time.Time) *V5Encoder {
	return &V5Encoder{boot: boot}
}

// Encode renders flows into NetFlow v5 packets stamped at now, at most
// v5MaxFlows each. The sampling rate is a header field, so records at
// different rates go in separate packets. An IPv6 flow fails the whole batch
// with ErrV5IPv6. Returns nil for an empty batch.
func (e *V5Encoder) Encode(now time.Time, flows []embed.FlowRecord) ([][]byte, error) {
	for _, f := range flows {
		if wire.IsIPv6(f) {
			return nil, ErrV5IPv6
		}
	}
	var pkts [][]byte
	for len(flows) > 0 {
		n := 1
		for n < len(flows) && n < v5MaxFlows && v5Sampling(flows[n].SamplingRate) == v5Sampling(flows[0].SamplingRate) {
			n++
		}
		pkts = append(pkts, e.packet(now, flows[:n]))
		flows = flows[n:]
	}
	return pkts, nil
}

// v5Sampling is the header sampling_interval for a record: mode 01 (packet
// interval) and the 1-in-N rate, with an unsampled record as 1 in 1 (the rate
// the other formats report for it). The rate saturates at the 14-bit
// maximum, 16383.
func v5Sampling(rate uint32) uint16 {
	return v5SamplingPacketInterval | wire.U16(min(wire.SamplingRate(rate), v5MaxSamplingRate))
}

func (e *V5Encoder) packet(now time.Time, flows []embed.FlowRecord) []byte {
	count := len(flows)
	buf := make([]byte, v5HeaderLen+count*v5RecordLen)

	uptimeMS := wire.UptimeMS(e.boot, now)
	seq := e.seq.Add(wire.U32(count)) - wire.U32(count) // sequence of first record

	binary.BigEndian.PutUint16(buf[0:2], 5)
	binary.BigEndian.PutUint16(buf[2:4], wire.U16(count))
	binary.BigEndian.PutUint32(buf[4:8], uptimeMS)
	binary.BigEndian.PutUint32(buf[8:12], wire.U32(now.Unix()))
	binary.BigEndian.PutUint32(buf[12:16], wire.U32(now.Nanosecond()))
	binary.BigEndian.PutUint32(buf[16:20], seq)
	buf[20] = e.engineType
	buf[21] = e.engineID
	binary.BigEndian.PutUint16(buf[22:24], v5Sampling(flows[0].SamplingRate))

	for i, f := range flows {
		e.putRecord(buf[v5HeaderLen+i*v5RecordLen:], f)
	}
	return buf
}

func (e *V5Encoder) putRecord(b []byte, f embed.FlowRecord) {
	copy(b[0:4], wire.IP4(f.SrcIP))
	copy(b[4:8], wire.IP4(f.DstIP))
	copy(b[8:12], wire.IP4(f.NextHop))
	binary.BigEndian.PutUint16(b[12:14], wire.U16(f.InputIface))
	binary.BigEndian.PutUint16(b[14:16], wire.U16(f.OutputIface))
	binary.BigEndian.PutUint32(b[16:20], wire.U32(f.Packets))
	binary.BigEndian.PutUint32(b[20:24], wire.U32(f.Bytes))
	binary.BigEndian.PutUint32(b[24:28], wire.UptimeMS(e.boot, f.StartTime))
	binary.BigEndian.PutUint32(b[28:32], wire.UptimeMS(e.boot, f.EndTime))
	binary.BigEndian.PutUint16(b[32:34], f.SrcPort)
	binary.BigEndian.PutUint16(b[34:36], f.DstPort)
	// b[36] pad1
	b[37] = f.TCPFlags
	b[38] = f.Protocol
	b[39] = f.TOS
	binary.BigEndian.PutUint16(b[40:42], wire.AS16(f.SrcAS))
	binary.BigEndian.PutUint16(b[42:44], wire.AS16(f.DstAS))
	b[44] = f.SrcMask
	b[45] = f.DstMask
	// b[46:48] pad2
}
