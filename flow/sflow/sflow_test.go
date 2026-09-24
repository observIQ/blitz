package sflow

import (
	"encoding/binary"
	"net"
	"testing"
	"time"

	"github.com/observiq/blitz/embed"
	"github.com/stretchr/testify/require"
)

func testFlow() embed.FlowRecord {
	return embed.FlowRecord{
		SrcIP: net.IPv4(10, 0, 0, 1), DstIP: net.IPv4(93, 184, 216, 34),
		SrcPort: 54321, DstPort: 443, Protocol: 6, TCPFlags: 0x1b,
		InputIface: 2, OutputIface: 3, Packets: 42, Bytes: 4200,
		SamplingRate: 1024,
	}
}

func be(b []byte, o int) uint32 { return binary.BigEndian.Uint32(b[o : o+4]) }

func TestSFlowDatagramHeader(t *testing.T) {
	enc := NewEncoder(net.IPv4(192, 0, 2, 1), 0)
	pkts := enc.Encode(time.Unix(1700000010, 0), time.Unix(1699999000, 0), []embed.FlowRecord{testFlow()})
	require.Len(t, pkts, 1)
	d := pkts[0]

	require.Equal(t, uint32(5), be(d, 0)) // version
	require.Equal(t, uint32(1), be(d, 4)) // agent addr type IPv4
	require.Equal(t, []byte{192, 0, 2, 1}, d[8:12])
	require.Equal(t, uint32(0), be(d, 12))         // sub_agent_id
	require.Equal(t, uint32(0), be(d, 16))         // datagram seq starts 0
	require.Equal(t, uint32(1010*1000), be(d, 20)) // uptime ms
	require.Equal(t, uint32(1), be(d, 24))         // num_samples
}

func TestSFlowSampledIPv4(t *testing.T) {
	enc := NewEncoder(net.IPv4(192, 0, 2, 1), 0)
	d := enc.Encode(time.Unix(1700000010, 0), time.Unix(1699999000, 0), []embed.FlowRecord{testFlow()})[0]

	s := d[28:]                           // first sample begins after 28-byte datagram header
	require.Equal(t, uint32(1), be(s, 0)) // sample_data_format = flow sample
	// sample header: format,len,seq,source_id,rate,pool,drops,input,output,num_records
	require.Equal(t, uint32(1024), be(s, 16)) // sampling_rate
	require.Equal(t, uint32(2), be(s, 28))    // input
	require.Equal(t, uint32(3), be(s, 32))    // output
	require.Equal(t, uint32(1), be(s, 36))    // num_flow_records

	fr := s[40:]
	require.Equal(t, uint32(3), be(fr, 0))                // flow_format = sampled ipv4
	d0 := fr[8:]                                          // sampled_ipv4 data
	require.Equal(t, uint32(4200), be(d0, 0))             // length (bytes)
	require.Equal(t, uint32(6), be(d0, 4))                // protocol
	require.Equal(t, []byte{10, 0, 0, 1}, d0[8:12])       // src ip
	require.Equal(t, []byte{93, 184, 216, 34}, d0[12:16]) // dst ip
	require.Equal(t, uint32(54321), be(d0, 16))           // src port
	require.Equal(t, uint32(443), be(d0, 20))             // dst port
}
