package netflow

import (
	"bytes"
	"encoding/binary"
	"net"
	"testing"
	"time"

	nfl "github.com/netsampler/goflow2/v2/decoders/netflowlegacy"
	protoproducer "github.com/netsampler/goflow2/v2/producer/proto"
	"github.com/stretchr/testify/require"

	"github.com/observiq/blitz/embed"
	"github.com/observiq/blitz/flow/wire"
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

	pkts := v5Encode(t, enc, now, []embed.FlowRecord{testFlow()})
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

func TestV5SplitsAtMaxFlows(t *testing.T) {
	enc := NewV5Encoder(time.Unix(1699999000, 0))
	flows := make([]embed.FlowRecord, v5MaxFlows+1)
	for i := range flows {
		flows[i] = testFlow()
	}
	pkts := v5Encode(t, enc, time.Unix(1700000010, 0), flows)
	require.Len(t, pkts, 2)
	require.Equal(t, uint16(29), binary.BigEndian.Uint16(pkts[0][2:4]), "30 records exceed the MTU budget")
	require.Equal(t, uint16(1), binary.BigEndian.Uint16(pkts[1][2:4]))
}

func TestV5FlowSequenceAdvances(t *testing.T) {
	enc := NewV5Encoder(time.Unix(1699999000, 0))
	now := time.Unix(1700000010, 0)
	v5Encode(t, enc, now, []embed.FlowRecord{testFlow(), testFlow()})
	pkts := v5Encode(t, enc, now, []embed.FlowRecord{testFlow()})
	// flow_sequence counts records, so the second packet starts at 2.
	require.Equal(t, uint32(2), binary.BigEndian.Uint32(pkts[0][16:20]))
}

func v5Encode(t *testing.T, enc *V5Encoder, now time.Time, flows []embed.FlowRecord) [][]byte {
	t.Helper()
	pkts, err := enc.Encode(now, flows)
	require.NoError(t, err)
	return pkts
}

func decodeV5(t *testing.T, pkt []byte) []*protoproducer.ProtoProducerMessage {
	t.Helper()
	var p nfl.PacketNetFlowV5
	require.NoError(t, nfl.DecodeMessageVersion(bytes.NewBuffer(pkt), &p))
	msgs, err := protoproducer.ProcessMessageNetFlowLegacy(&p)
	require.NoError(t, err)
	out := make([]*protoproducer.ProtoProducerMessage, 0, len(msgs))
	for _, m := range msgs {
		out = append(out, m.(*protoproducer.ProtoProducerMessage))
	}
	return out
}

// TestV5SamplingInterval: the header's sampling_interval carries mode 01
// (packet interval) in the top 2 bits and the 1-in-N rate in the low 14.
func TestV5SamplingInterval(t *testing.T) {
	f := testFlow()
	f.SamplingRate = 1024
	pkt := v5Encode(t, NewV5Encoder(v9Boot), v9Now, []embed.FlowRecord{f})[0]
	require.Equal(t, uint16(1<<14|1024), binary.BigEndian.Uint16(pkt[22:24]))
	require.Equal(t, uint64(1024), decodeV5(t, pkt)[0].SamplingRate)

	// Unsampled reports 1 in 1, the rate every other format reports for it,
	// so a collector sees the same sampling_rate whichever format carries it.
	f.SamplingRate = 0
	pkt = v5Encode(t, NewV5Encoder(v9Boot), v9Now, []embed.FlowRecord{f})[0]
	require.Equal(t, uint16(1<<14|1), binary.BigEndian.Uint16(pkt[22:24]))
	require.Equal(t, uint64(1), decodeV5(t, pkt)[0].SamplingRate)

	// The interval field is 14 bits: a larger rate caps at 16383 and never
	// spills into the two sampling-mode bits.
	for _, rate := range []uint32{16383, 16384, 40000, 0xFFFFFFFF} {
		f.SamplingRate = rate
		pkt = v5Encode(t, NewV5Encoder(v9Boot), v9Now, []embed.FlowRecord{f})[0]
		want := uint16(min(rate, 16383))
		require.Equal(t, uint16(1<<14)|want, binary.BigEndian.Uint16(pkt[22:24]), "rate %d", rate)
	}
}

// TestV5SplitsOnRateChange: the rate lives in the packet header, so records at
// different rates never share a packet.
func TestV5SplitsOnRateChange(t *testing.T) {
	a, b := testFlow(), testFlow()
	a.SamplingRate, b.SamplingRate = 1024, 512
	pkts := v5Encode(t, NewV5Encoder(v9Boot), v9Now, []embed.FlowRecord{a, a, b})
	require.Len(t, pkts, 2)
	require.Equal(t, uint16(2), binary.BigEndian.Uint16(pkts[0][2:4]))
	require.Equal(t, uint16(1<<14|512), binary.BigEndian.Uint16(pkts[1][22:24]))
}

// TestV5FourByteASIsASTrans: v5's AS fields are 2 bytes, so a 32-bit ASN is
// reported as AS_TRANS (23456, RFC 6793).
func TestV5FourByteASIsASTrans(t *testing.T) {
	f := testFlow()
	f.SrcAS, f.DstAS = 4_200_000_000, 64500
	m := decodeV5(t, v5Encode(t, NewV5Encoder(v9Boot), v9Now, []embed.FlowRecord{f})[0])[0]
	require.Equal(t, uint32(23456), m.SrcAs)
	require.Equal(t, uint32(64500), m.DstAs)
}

func TestV5PacketsFitMTU(t *testing.T) {
	flows := make([]embed.FlowRecord, 60)
	for i := range flows {
		flows[i] = testFlow()
	}
	for _, p := range v5Encode(t, NewV5Encoder(v9Boot), v9Now, flows) {
		require.LessOrEqual(t, len(p), wire.MaxPayload)
	}
}

// TestV5RejectsIPv6: v5's record layout is IPv4-only.
func TestV5RejectsIPv6(t *testing.T) {
	_, err := NewV5Encoder(v9Boot).Encode(v9Now, []embed.FlowRecord{testFlow(), v6Flow()})
	require.ErrorContains(t, err, "IPv6")
}
