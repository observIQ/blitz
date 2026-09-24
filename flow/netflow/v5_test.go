package netflow

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
		SrcIP:       net.IPv4(10, 0, 0, 1),
		DstIP:       net.IPv4(93, 184, 216, 34),
		SrcPort:     54321,
		DstPort:     443,
		Protocol:    6,
		TOS:         0,
		TCPFlags:    0x1b,
		NextHop:     net.IPv4(10, 0, 0, 254),
		InputIface:  2,
		OutputIface: 3,
		Packets:     42,
		Bytes:       4200,
		SrcAS:       64500,
		DstAS:       15133,
		SrcMask:     24,
		DstMask:     19,
		StartTime:   time.Unix(1700000000, 0),
		EndTime:     time.Unix(1700000005, 0),
	}
}

func TestV5EncodeHeaderAndRecord(t *testing.T) {
	boot := time.Unix(1699999000, 0)
	enc := NewV5Encoder(boot)
	now := time.Unix(1700000010, 0)

	pkts := enc.Encode(now, []embed.FlowRecord{testFlow()})
	require.Len(t, pkts, 1)
	pkt := pkts[0]
	require.Len(t, pkt, 24+48) // header + one record

	require.Equal(t, uint16(5), binary.BigEndian.Uint16(pkt[0:2]))           // version
	require.Equal(t, uint16(1), binary.BigEndian.Uint16(pkt[2:4]))           // count
	require.Equal(t, uint32(1010*1000), binary.BigEndian.Uint32(pkt[4:8]))   // sysuptime ms (now-boot)
	require.Equal(t, uint32(1700000010), binary.BigEndian.Uint32(pkt[8:12])) // unix_secs

	rec := pkt[24:]
	require.Equal(t, []byte{10, 0, 0, 1}, rec[0:4])                      // srcaddr
	require.Equal(t, []byte{93, 184, 216, 34}, rec[4:8])                 // dstaddr
	require.Equal(t, []byte{10, 0, 0, 254}, rec[8:12])                   // nexthop
	require.Equal(t, uint16(2), binary.BigEndian.Uint16(rec[12:14]))     // input
	require.Equal(t, uint16(3), binary.BigEndian.Uint16(rec[14:16]))     // output
	require.Equal(t, uint32(42), binary.BigEndian.Uint32(rec[16:20]))    // dPkts
	require.Equal(t, uint32(4200), binary.BigEndian.Uint32(rec[20:24]))  // dOctets
	require.Equal(t, uint16(54321), binary.BigEndian.Uint16(rec[32:34])) // srcport
	require.Equal(t, uint16(443), binary.BigEndian.Uint16(rec[34:36]))   // dstport
	require.Equal(t, uint8(0x1b), rec[37])                               // tcp_flags
	require.Equal(t, uint8(6), rec[38])                                  // prot
	require.Equal(t, uint16(64500), binary.BigEndian.Uint16(rec[40:42])) // src_as
	require.Equal(t, uint16(15133), binary.BigEndian.Uint16(rec[42:44])) // dst_as
	require.Equal(t, uint8(24), rec[44])                                 // src_mask
	require.Equal(t, uint8(19), rec[45])                                 // dst_mask

	// first/last are sysuptime-relative ms
	first := binary.BigEndian.Uint32(rec[24:28])
	last := binary.BigEndian.Uint32(rec[28:32])
	require.Equal(t, uint32((1700000000-1699999000)*1000), first)
	require.Equal(t, uint32((1700000005-1699999000)*1000), last)
}

func TestV5SplitsAt30Records(t *testing.T) {
	enc := NewV5Encoder(time.Unix(1699999000, 0))
	flows := make([]embed.FlowRecord, 31)
	for i := range flows {
		flows[i] = testFlow()
	}
	pkts := enc.Encode(time.Unix(1700000010, 0), flows)
	require.Len(t, pkts, 2)
	require.Equal(t, uint16(30), binary.BigEndian.Uint16(pkts[0][2:4]))
	require.Equal(t, uint16(1), binary.BigEndian.Uint16(pkts[1][2:4]))
}

func TestV5FlowSequenceAdvances(t *testing.T) {
	enc := NewV5Encoder(time.Unix(1699999000, 0))
	now := time.Unix(1700000010, 0)
	enc.Encode(now, []embed.FlowRecord{testFlow(), testFlow()})
	pkts := enc.Encode(now, []embed.FlowRecord{testFlow()})
	// flow_sequence counts records, so the second packet starts at 2.
	require.Equal(t, uint32(2), binary.BigEndian.Uint32(pkts[0][16:20]))
}
