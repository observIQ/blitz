// Package sflow encodes blitz FlowRecords into sFlow v5 datagrams
// (sflow_version_5.txt). Unlike the NetFlow/IPFIX family (which export flow
// summaries), sFlow exports sampled packets; here each of a FlowRecord's
// sampled packets becomes one flow sample carrying a sampled_ipv4 (format 3)
// or sampled_ipv6 (format 4) flow record, the summary forms that map a 5-tuple
// without synthesizing a raw packet header.
package sflow

import (
	"context"
	"encoding/binary"
	"net"
	"slices"
	"sync"
	"time"

	"github.com/observiq/blitz/embed"
	"github.com/observiq/blitz/flow/wire"
)

const (
	sampledIPv4Len = 32 // length, protocol, src/dst IPv4, ports, tcp_flags, tos
	sampledIPv6Len = 56 // length, protocol, src/dst IPv6, ports, tcp_flags, priority

	flowFormatSampledIPv4 = 3
	flowFormatSampledIPv6 = 4

	// sampleHdrLen is a flow sample's fields before its flow record:
	// sequence_number, source_id, sampling_rate, sample_pool, drops, input,
	// output, and the flow record count.
	sampleHdrLen = 8 * 4

	addressTypeIPv4 = 1
	addressTypeIPv6 = 2

	// interfaceMask keeps an interface value in the 30-bit ifIndex of format
	// 0 (single interface); sourceIndexMask keeps a source_id index in its
	// lower 3 bytes (type 0 = ifIndex in the top byte).
	interfaceMask   = 0x3FFFFFFF
	sourceIndexMask = 0x00FFFFFF
)

// sampleLen is one flow sample's size on the wire: data format and length,
// the sample header, and one flow record (format, length, data).
func sampleLen(v6 bool) int {
	if v6 {
		return 8 + sampleHdrLen + 8 + sampledIPv6Len
	}
	return 8 + sampleHdrLen + 8 + sampledIPv4Len
}

// sourceKey names a data source: sFlow v5 scopes source_id to its agent (the
// sub-agent is fixed per Encoder).
type sourceKey struct {
	agent string
	id    uint32
}

// source is a data source's counters.
type source struct {
	seq  uint32 // flow samples generated
	pool uint32 // packets that could have been sampled
}

// Encoder encodes FlowRecords as sFlow v5 datagrams. Safe for concurrent Encode.
type Encoder struct {
	agentIP    net.IP
	subAgentID uint32
	boot       time.Time

	// Each agent's sequence numbers and counters must persist across
	// datagrams, so these maps are never pruned: they grow with the distinct
	// agent addresses and sources seen (the built-in generator uses one agent).
	mu        sync.Mutex
	datagrams map[string]uint32 // datagram sequence per agent
	sources   map[sourceKey]*source
}

// NewEncoder returns an sFlow encoder reporting agentIP (IPv4 or IPv6) as the
// exporter address, with its uptime clock starting at boot. A nil agentIP
// reports each record's ExporterIP (127.0.0.1 when a record has none).
func NewEncoder(agentIP net.IP, subAgentID uint32, boot time.Time) *Encoder {
	return &Encoder{agentIP: agentIP, subAgentID: subAgentID, boot: boot, sources: map[sourceKey]*source{}, datagrams: map[string]uint32{}}
}

// agentFor is the agent address a record's samples are reported under.
func (e *Encoder) agentFor(f embed.FlowRecord) net.IP {
	switch {
	case e.agentIP != nil:
		return e.agentIP
	case f.ExporterIP != nil:
		return f.ExporterIP
	default:
		return net.IPv4(127, 0, 0, 1)
	}
}

func headerLen(agent net.IP) int {
	if agent.To4() == nil {
		return 28 + 12
	}
	return 28
}

// packet is one sampled packet of a FlowRecord: the record and the packet's
// TCP flags.
type packet struct {
	f     embed.FlowRecord
	flags uint8
}

// Encode renders flows as sFlow v5 datagrams, one flow sample per sampled
// packet (a record's Packets, at least one), each datagram within
// wire.MaxPayload, stamped with uptime = now-boot. A record's Packets are its
// sampled packets, as NetFlow/IPFIX count them, so a collector scaling samples
// by sampling_rate totals what it totals from the same record over NetFlow.
// Returns nil for an empty batch. EncodeTo streams the same datagrams without
// holding them all.
func (e *Encoder) Encode(now time.Time, flows []embed.FlowRecord) [][]byte {
	var out [][]byte
	_ = e.EncodeTo(context.Background(), now, flows, func(d []byte) error {
		out = append(out, slices.Clone(d))
		return nil
	})
	return out
}

// EncodeTo renders flows as Encode does, handing each datagram to send as it
// fills, so memory stays at one datagram however many packets a record has.
// The slice is reused after send returns; send must copy what it keeps. A
// send error or a cancelled ctx stops encoding and is returned.
func (e *Encoder) EncodeTo(ctx context.Context, now time.Time, flows []embed.FlowRecord, send func([]byte) error) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	buf := make([]byte, wire.MaxPayload)
	batch := make([]packet, 0, 32)
	var agent net.IP
	size := 0
	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		// A record's samples can fill many datagrams; stop between them.
		if err := ctx.Err(); err != nil {
			return err
		}
		d := e.datagramInto(buf[:size], now, agent, batch)
		batch, size = batch[:0], headerLen(agent)
		return send(d)
	}
	for _, f := range flows {
		// A datagram carries one agent's samples.
		if a := e.agentFor(f); !a.Equal(agent) {
			if err := flush(); err != nil {
				return err
			}
			agent, size = a, headerLen(a)
		}
		l := sampleLen(wire.IsIPv6(f))
		n := max(f.Packets, 1)
		for i := uint64(0); i < n; i++ {
			if len(batch) > 0 && size+l > wire.MaxPayload {
				if err := flush(); err != nil {
					return err
				}
			}
			batch = append(batch, packet{f: f, flags: packetFlags(f.TCPFlags, i, n)})
			size += l
		}
	}
	return flush()
}

// datagramInto writes one datagram of pkts into buf, which is exactly its
// size, and returns it.
func (e *Encoder) datagramInto(buf []byte, now time.Time, agent net.IP, pkts []packet) []byte {
	clear(buf)
	binary.BigEndian.PutUint32(buf[0:4], 5) // version
	var off int
	if v4 := agent.To4(); v4 != nil {
		binary.BigEndian.PutUint32(buf[4:8], addressTypeIPv4)
		copy(buf[8:12], v4)
		off = 12
	} else {
		binary.BigEndian.PutUint32(buf[4:8], addressTypeIPv6)
		copy(buf[8:24], wire.IP16(agent))
		off = 24
	}
	binary.BigEndian.PutUint32(buf[off:], e.subAgentID)
	// The datagram sequence number counts each agent's datagrams.
	key := agent.String()
	binary.BigEndian.PutUint32(buf[off+4:], e.datagrams[key])
	binary.BigEndian.PutUint32(buf[off+8:], wire.UptimeMS(e.boot, now))
	binary.BigEndian.PutUint32(buf[off+12:], wire.U32(len(pkts))) // num_samples
	e.datagrams[key]++

	off += 16
	for _, p := range pkts {
		off += e.putSample(buf[off:], key, p)
	}
	return buf
}

// putSample writes one flow sample (enterprise 0, format 1). source_id is the
// ingress ifIndex (type 0); its sequence_number and sample_pool advance per
// source (agent plus source_id), the pool by the sampling rate (a 1-in-N sample stands for N
// packets). Drops are always 0: the simulated agent never loses samples.
func (e *Encoder) putSample(b []byte, agent string, p packet) int {
	f := p.f
	v6 := wire.IsIPv6(f)
	total := sampleLen(v6)
	rate := wire.SamplingRate(f.SamplingRate)
	srcID := f.InputIface & sourceIndexMask
	k := sourceKey{agent, srcID}
	src := e.sources[k]
	if src == nil {
		src = &source{}
		e.sources[k] = src
	}
	src.pool += rate

	binary.BigEndian.PutUint32(b[0:4], 1)                             // sample_data_format = flow sample
	binary.BigEndian.PutUint32(b[4:8], wire.U32(total-8))             // sample_length
	binary.BigEndian.PutUint32(b[8:12], src.seq)                      // sequence_number
	binary.BigEndian.PutUint32(b[12:16], srcID)                       // source_id: type 0 (ifIndex), index
	binary.BigEndian.PutUint32(b[16:20], rate)                        // sampling_rate
	binary.BigEndian.PutUint32(b[20:24], src.pool)                    // sample_pool
	binary.BigEndian.PutUint32(b[24:28], 0)                           // drops
	binary.BigEndian.PutUint32(b[28:32], f.InputIface&interfaceMask)  // input
	binary.BigEndian.PutUint32(b[32:36], f.OutputIface&interfaceMask) // output
	binary.BigEndian.PutUint32(b[36:40], 1)                           // num_flow_records
	src.seq++

	fr := b[40:]
	format, dataLen := flowFormatSampledIPv4, sampledIPv4Len
	if v6 {
		format, dataLen = flowFormatSampledIPv6, sampledIPv6Len
	}
	binary.BigEndian.PutUint32(fr[0:4], wire.U32(format))
	binary.BigEndian.PutUint32(fr[4:8], wire.U32(dataLen))
	d := fr[8:]
	binary.BigEndian.PutUint32(d[0:4], wire.U32(packetLen(f))) // length
	binary.BigEndian.PutUint32(d[4:8], wire.U32(f.Protocol))   // protocol
	var off int
	if v6 {
		copy(d[8:24], wire.IP16(f.SrcIP))
		copy(d[24:40], wire.IP16(f.DstIP))
		off = 40
	} else {
		copy(d[8:12], wire.IP4(f.SrcIP))
		copy(d[12:16], wire.IP4(f.DstIP))
		off = 16
	}
	srcPort, dstPort := ports(f)
	binary.BigEndian.PutUint32(d[off:], srcPort)
	binary.BigEndian.PutUint32(d[off+4:], dstPort)
	binary.BigEndian.PutUint32(d[off+8:], wire.U32(p.flags))
	binary.BigEndian.PutUint32(d[off+12:], wire.U32(f.TOS)) // tos (IPv4) / priority (IPv6)
	return total
}

// maxIPLen is the largest IP packet (the 16-bit total/payload length field).
const maxIPLen = 65535

// packetLen is one sampled packet's IP length, the flow's mean packet size.
// sampled_ipv4/ipv6.length describes a single packet (a collector scales it by
// sampling_rate), so it must never carry the flow's byte total. A zero packet
// count is treated as one packet, and the result is clamped to maxIPLen.
func packetLen(f embed.FlowRecord) uint64 {
	return min(f.Bytes/max(f.Packets, 1), maxIPLen)
}

// ports returns sampled_ipv4/ipv6 src_port/dst_port. The sFlow v5 spec defines
// them as "TCP/UDP port number or equivalent" with no ICMP encoding; the
// reference decoder (sflowtool readFlowSample_IPv4) reads an ICMP record's type
// from dst_port with src_port 0, so ICMP and ICMPv6 write that. The ICMP code
// has no sampled_ipv4/ipv6 field and is dropped.
func ports(f embed.FlowRecord) (src, dst uint32) {
	if wire.IsICMP(f.Protocol) {
		return 0, uint32(f.DstPort >> 8)
	}
	return uint32(f.SrcPort), uint32(f.DstPort)
}

// TCP flag bits.
const (
	tcpFIN = 0x01
	tcpSYN = 0x02
	tcpPSH = 0x08
	tcpACK = 0x10
)

// packetFlags is the flags of packet i of n in a flow whose cumulative flags
// (the OR over its lifetime, as NetFlow/IPFIX carry them) are cumulative. A
// connection opens with SYN, then ACK, carries data with PSH|ACK, and closes
// with FIN|ACK. A flow that never saw ACK (a SYN flood) repeats its flags on
// every packet, and non-TCP stays 0. The last packet also carries any
// cumulative flag its earlier packets did not (a short flow), so the packets
// always OR back to exactly the cumulative flags.
func packetFlags(cumulative uint8, i, n uint64) uint8 {
	f := connectionPacketFlags(cumulative, i, n)
	if i == n-1 {
		// Packets 2..n-2 repeat packet 2's flags, so the first three cover
		// every earlier packet.
		var earlier uint8
		for j := uint64(0); j < min(i, 3); j++ {
			earlier |= connectionPacketFlags(cumulative, j, n)
		}
		f |= cumulative &^ earlier
	}
	return f
}

// connectionPacketFlags is packet i of n's place in the connection alone.
func connectionPacketFlags(cumulative uint8, i, n uint64) uint8 {
	switch {
	case cumulative&tcpACK == 0:
		return cumulative
	case i == 0 && cumulative&tcpSYN != 0:
		return tcpSYN
	case i == n-1 && cumulative&tcpFIN != 0:
		return tcpFIN | tcpACK
	case i >= 2 && cumulative&tcpPSH != 0:
		return tcpPSH | tcpACK
	default:
		return tcpACK
	}
}
