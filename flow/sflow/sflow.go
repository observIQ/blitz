// Package sflow encodes blitz FlowRecords into sFlow v5 datagrams. Unlike the
// NetFlow/IPFIX family (which export flow summaries), sFlow exports sampled
// packets; here each FlowRecord becomes one flow sample carrying a sampled-IPv4
// flow record (enterprise 0, format 3), the summary form that maps a 5-tuple
// without synthesizing a raw packet header.
package sflow

import (
	"encoding/binary"
	"net"
	"sync/atomic"
	"time"

	"github.com/observiq/blitz/embed"
	"github.com/observiq/blitz/flow/wire"
)

const (
	sampledIPv4Len = 32 // 8 uint32 fields
	flowRecordLen  = 8 + sampledIPv4Len
	// sample body after the sample_length field: 8 uint32 header fields +
	// one flow record.
	sampleBodyLen = 8*4 + flowRecordLen
	datagramHdr   = 28
)

// Encoder encodes FlowRecords as sFlow v5 datagrams. Safe for concurrent Encode.
type Encoder struct {
	agentIP    net.IP
	subAgentID uint32
	datagram   atomic.Uint32 // datagram sequence
	sampleSeq  atomic.Uint32 // per-sample sequence
}

// NewEncoder returns an sFlow encoder reporting agentIP as the exporter address.
func NewEncoder(agentIP net.IP, subAgentID uint32) *Encoder {
	return &Encoder{agentIP: agentIP, subAgentID: subAgentID}
}

// Encode renders flows into a single sFlow v5 datagram, one flow sample per
// record, stamped with uptime = now-boot. Returns nil for an empty batch.
func (e *Encoder) Encode(now, boot time.Time, flows []embed.FlowRecord) [][]byte {
	if len(flows) == 0 {
		return nil
	}
	buf := make([]byte, datagramHdr+len(flows)*(8+sampleBodyLen))

	binary.BigEndian.PutUint32(buf[0:4], 5)                                        // version
	binary.BigEndian.PutUint32(buf[4:8], 1)                                        // agent address type IPv4
	copy(buf[8:12], ip4(e.agentIP))                                                // agent address
	binary.BigEndian.PutUint32(buf[12:16], e.subAgentID)                           // sub agent id
	binary.BigEndian.PutUint32(buf[16:20], e.datagram.Add(1)-1)                    // datagram sequence
	binary.BigEndian.PutUint32(buf[20:24], wire.U32(now.Sub(boot).Milliseconds())) // uptime
	binary.BigEndian.PutUint32(buf[24:28], wire.U32(len(flows)))                   // num_samples

	off := datagramHdr
	for _, f := range flows {
		off += e.putSample(buf[off:], f)
	}
	return [][]byte{buf}
}

func (e *Encoder) putSample(b []byte, f embed.FlowRecord) int {
	binary.BigEndian.PutUint32(b[0:4], 1)                       // sample_data_format = flow sample
	binary.BigEndian.PutUint32(b[4:8], wire.U32(sampleBodyLen)) // sample_length
	binary.BigEndian.PutUint32(b[8:12], e.sampleSeq.Add(1)-1)   // sample_sequence_number
	binary.BigEndian.PutUint32(b[12:16], e.subAgentID)          // source_id
	binary.BigEndian.PutUint32(b[16:20], f.SamplingRate)        // sampling_rate
	binary.BigEndian.PutUint32(b[20:24], 0)                     // sample_pool
	binary.BigEndian.PutUint32(b[24:28], 0)                     // drops
	binary.BigEndian.PutUint32(b[28:32], f.InputIface)          // input
	binary.BigEndian.PutUint32(b[32:36], f.OutputIface)         // output
	binary.BigEndian.PutUint32(b[36:40], 1)                     // num_flow_records

	fr := b[40:]
	binary.BigEndian.PutUint32(fr[0:4], 3)                        // flow_format = sampled ipv4
	binary.BigEndian.PutUint32(fr[4:8], wire.U32(sampledIPv4Len)) // flow_length
	d := fr[8:]
	binary.BigEndian.PutUint32(d[0:4], wire.U32(f.Bytes))    // length
	binary.BigEndian.PutUint32(d[4:8], wire.U32(f.Protocol)) // protocol
	copy(d[8:12], ip4(f.SrcIP))
	copy(d[12:16], ip4(f.DstIP))
	binary.BigEndian.PutUint32(d[16:20], wire.U32(f.SrcPort))
	binary.BigEndian.PutUint32(d[20:24], wire.U32(f.DstPort))
	binary.BigEndian.PutUint32(d[24:28], wire.U32(f.TCPFlags))
	binary.BigEndian.PutUint32(d[28:32], wire.U32(f.TOS))

	return 8 + sampleBodyLen
}

func ip4(ip net.IP) []byte {
	if v4 := ip.To4(); v4 != nil {
		return v4
	}
	return []byte{0, 0, 0, 0}
}
