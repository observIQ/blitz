package sflow

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"runtime"
	"testing"
	"time"

	gsf "github.com/netsampler/goflow2/v2/decoders/sflow"
	protoproducer "github.com/netsampler/goflow2/v2/producer/proto"
	"github.com/stretchr/testify/require"

	"github.com/observiq/blitz/embed"
	"github.com/observiq/blitz/flow/wire"
)

func testFlow() embed.FlowRecord {
	return embed.FlowRecord{
		SrcIP: net.IPv4(10, 0, 0, 1), DstIP: net.IPv4(93, 184, 216, 34),
		SrcPort: 54321, DstPort: 443, Protocol: 6, TCPFlags: 0x02,
		InputIface: 2, OutputIface: 3, Packets: 1, Bytes: 100,
		SamplingRate: 1024,
	}
}

func be(b []byte, o int) uint32 { return binary.BigEndian.Uint32(b[o : o+4]) }

func TestSFlowDatagramHeader(t *testing.T) {
	enc := NewEncoder(net.IPv4(192, 0, 2, 1), 0, time.Unix(1699999000, 0))
	pkts := enc.Encode(time.Unix(1700000010, 0), []embed.FlowRecord{testFlow()})
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
	enc := NewEncoder(net.IPv4(192, 0, 2, 1), 0, time.Unix(1699999000, 0))
	d := enc.Encode(time.Unix(1700000010, 0), []embed.FlowRecord{testFlow()})[0]

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
	require.Equal(t, uint32(100), be(d0, 0))              // length = one packet
	require.Equal(t, uint32(6), be(d0, 4))                // protocol
	require.Equal(t, []byte{10, 0, 0, 1}, d0[8:12])       // src ip
	require.Equal(t, []byte{93, 184, 216, 34}, d0[12:16]) // dst ip
	require.Equal(t, uint32(54321), be(d0, 16))           // src port
	require.Equal(t, uint32(443), be(d0, 20))             // dst port
	require.Equal(t, uint32(0x02), be(d0, 24))            // tcp_flags: a 1-packet flow's packet is its SYN
}

// sampledIPv4 encodes f and returns its sampled_ipv4 data.
func sampledIPv4(f embed.FlowRecord) []byte {
	d := NewEncoder(net.IPv4(192, 0, 2, 1), 0, time.Unix(1699999000, 0)).Encode(time.Unix(1700000010, 0), []embed.FlowRecord{f})[0]
	return d[28+40+8:]
}

// TestSFlowLengthIsOnePacket pins sampled_ipv4.length to one packet's IP
// length, never the flow total: a collector multiplies it by sampling_rate.
func TestSFlowLengthIsOnePacket(t *testing.T) {
	f := testFlow()
	f.Packets, f.Bytes = 700, 700*1500
	d := sampledIPv4(f)
	require.Equal(t, uint32(1500), be(d, 0))
	require.LessOrEqual(t, be(d, 0), uint32(65535))
}

// TestSFlowPacketFlags maps a flow's cumulative flags to one sampled packet's.
func TestSFlowPacketFlags(t *testing.T) {
	syn := testFlow()
	syn.TCPFlags = 0x02
	require.Equal(t, uint32(0x02), be(sampledIPv4(syn), 24), "SYN-only flow samples a SYN packet")

	udp := testFlow()
	udp.Protocol, udp.TCPFlags = 17, 0
	require.Zero(t, be(sampledIPv4(udp), 24), "UDP has no TCP flags")
}

// TestSFlowZeroPacketsIsOnePacket treats a zero packet count as one packet,
// clamping the length to the IPv4 maximum.
func TestSFlowZeroPacketsIsOnePacket(t *testing.T) {
	f := testFlow()
	f.Packets, f.Bytes = 0, 900
	require.Equal(t, uint32(900), be(sampledIPv4(f), 0))
	f.Bytes = 1_000_000
	require.Equal(t, uint32(65535), be(sampledIPv4(f), 0))
}

// TestSFlowICMPPorts follows the sFlow reference decoder (sflowtool), which
// reads an ICMP sampled_ipv4 record's type from dst_port with src_port 0.
func TestSFlowICMPPorts(t *testing.T) {
	f := testFlow()
	f.Protocol, f.TCPFlags, f.SrcPort, f.DstPort = 1, 0, 0, 3<<8|3 // port unreachable
	d := sampledIPv4(f)
	require.Zero(t, be(d, 16), "src_port")
	require.Equal(t, uint32(3), be(d, 20), "dst_port carries the ICMP type")
}

// samplesOf encodes f and returns every flow sample's sampled_ipv4 data, in
// order, across all datagrams.
func samplesOf(t *testing.T, f embed.FlowRecord) [][]byte {
	t.Helper()
	var out [][]byte
	for _, d := range NewEncoder(net.IPv4(192, 0, 2, 1), 0, time.Unix(1699999000, 0)).Encode(time.Unix(1700000010, 0), []embed.FlowRecord{f}) {
		off := 28
		for i := uint32(0); i < be(d, 24); i++ {
			out = append(out, d[off+40+8:off+40+8+sampledIPv4Len])
			off += sampleLen(false)
		}
	}
	return out
}

// TestSFlowOneSamplePerPacket pins that a record's Packets are its sampled
// packets, as NetFlow/IPFIX count them: each becomes one flow sample, so a
// collector scaling samples by sampling_rate totals what it totals from the
// same record over NetFlow or IPFIX.
func TestSFlowOneSamplePerPacket(t *testing.T) {
	f := testFlow()
	f.Packets, f.Bytes, f.TCPFlags = 500, 500*1400, 0x1b
	s := samplesOf(t, f)
	require.Len(t, s, 500)
	var bytes uint64
	for _, d := range s {
		bytes += uint64(be(d, 0))
	}
	require.Equal(t, f.Bytes, bytes)
}

// TestSFlowPacketFlagsFollowConnection pins each sampled packet's flags to its
// place in the connection, so the flags OR to the record's cumulative flags.
func TestSFlowPacketFlagsFollowConnection(t *testing.T) {
	for _, tc := range []struct {
		packets uint64
		flags   uint8
		want    []uint32
	}{
		{1, 0x02, []uint32{0x02}},
		{2, 0x12, []uint32{0x02, 0x10}},
		{3, 0x1a, []uint32{0x02, 0x10, 0x18}},
		{5, 0x1b, []uint32{0x02, 0x10, 0x18, 0x18, 0x11}},
		{3, 0x02, []uint32{0x02, 0x02, 0x02}}, // SYN flood: every packet is a SYN
	} {
		f := testFlow()
		f.Packets, f.Bytes, f.TCPFlags = tc.packets, tc.packets*60, tc.flags
		var got []uint32
		for _, d := range samplesOf(t, f) {
			got = append(got, be(d, 24))
		}
		require.Equal(t, tc.want, got, "%d packets, flags %#x", tc.packets, tc.flags)
	}
}

// TestSFlowEncodeToStreamsDatagrams pins that EncodeTo holds one datagram at a
// time rather than every sample of a record: a record of a million sampled
// packets streams out with a small, fixed allocation, and a send error stops
// encoding at once.
func TestSFlowEncodeToStreamsDatagrams(t *testing.T) {
	f := testFlow()
	f.Packets, f.Bytes = 1_000_000, 1_000_000*100
	enc := NewEncoder(net.IPv4(192, 0, 2, 1), 0, time.Unix(1699999000, 0))
	var samples uint64
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	require.NoError(t, enc.EncodeTo(context.Background(), time.Unix(1700000010, 0), []embed.FlowRecord{f}, func(d []byte) error {
		samples += uint64(be(d, 24))
		return nil
	}))
	runtime.ReadMemStats(&after)
	require.Equal(t, f.Packets, samples)
	// TotalAlloc is process-wide, so the bound leaves room for other
	// goroutines: streaming allocates about 1 MiB, materializing a million
	// samples well over 100 MiB.
	require.Less(t, after.TotalAlloc-before.TotalAlloc, uint64(16<<20), "a million samples must not be materialized")

	sends := 0
	stop := errors.New("stop")
	require.ErrorIs(t, enc.EncodeTo(context.Background(), time.Unix(1700000010, 0), []embed.FlowRecord{f}, func([]byte) error { sends++; return stop }), stop)
	require.Equal(t, 1, sends)
}

// TestSFlowPacketFlagsORToCumulative pins that a record's sampled packets
// always carry, together, exactly its cumulative flags, for any packet count
// and any flag set a record can hold.
func TestSFlowPacketFlagsORToCumulative(t *testing.T) {
	for cum := uint8(0); cum < 0x20; cum++ {
		for n := uint64(1); n <= 6; n++ {
			var or uint8
			for i := uint64(0); i < n; i++ {
				or |= packetFlags(cum, i, n)
			}
			require.Equal(t, cum, or, "flags %#x over %d packets", cum, n)
		}
	}
}

// TestSFlowEncodeToHonorsContext pins that a cancelled context stops encoding
// before the next datagram, so Stop can interrupt a record of any size.
func TestSFlowEncodeToHonorsContext(t *testing.T) {
	f := testFlow()
	f.Packets, f.Bytes = 1_000_000_000, 1_000_000_000*100
	enc := NewEncoder(net.IPv4(192, 0, 2, 1), 0, time.Unix(1699999000, 0))
	ctx, cancel := context.WithCancel(context.Background())
	sends := 0
	err := enc.EncodeTo(ctx, time.Unix(1700000010, 0), []embed.FlowRecord{f}, func([]byte) error {
		sends++
		cancel()
		return nil
	})
	require.ErrorIs(t, err, context.Canceled)
	require.Equal(t, 1, sends)
}

// TestSFlowEncodeToStopsOnAgentChangeSendError pins that a send error while
// flushing one agent's datagram, on reaching a record from another agent,
// stops encoding and is returned.
func TestSFlowEncodeToStopsOnAgentChangeSendError(t *testing.T) {
	a, b := testFlow(), testFlow()
	a.ExporterIP, b.ExporterIP = net.IPv4(10, 0, 0, 1), net.IPv4(10, 0, 0, 2)
	enc := NewEncoder(nil, 0, time.Unix(1699999000, 0))
	stop := errors.New("stop")
	sends := 0
	err := enc.EncodeTo(context.Background(), time.Unix(1700000010, 0), []embed.FlowRecord{a, b}, func([]byte) error { sends++; return stop })
	require.ErrorIs(t, err, stop)
	require.Equal(t, 1, sends)
}

var (
	sfBoot = time.Unix(1699999000, 0)
	sfNow  = time.Unix(1700000010, 0)
)

// decodeSFlow decodes one datagram with goflow2 (the netflowreceiver decoder)
// and returns the packet, its flow samples, and the produced flow messages.
func decodeSFlow(t *testing.T, d []byte) (gsf.Packet, []gsf.FlowSample, []*protoproducer.ProtoProducerMessage) {
	t.Helper()
	var p gsf.Packet
	require.NoError(t, gsf.DecodeMessageVersion(bytes.NewBuffer(d), &p))
	var samples []gsf.FlowSample
	for _, s := range p.Samples {
		samples = append(samples, s.(gsf.FlowSample))
	}
	msgs, err := protoproducer.ProcessMessageSFlowConfig(&p, nil)
	require.NoError(t, err)
	out := make([]*protoproducer.ProtoProducerMessage, 0, len(msgs))
	for _, m := range msgs {
		out = append(out, m.(*protoproducer.ProtoProducerMessage))
	}
	return p, samples, out
}

func TestSFlowRoundTripGoflow2(t *testing.T) {
	_, _, msgs := decodeSFlow(t, NewEncoder(net.IPv4(192, 0, 2, 1), 0, sfBoot).Encode(sfNow, []embed.FlowRecord{testFlow()})[0])
	require.Len(t, msgs, 1)
	m := msgs[0]
	require.Equal(t, net.IPv4(10, 0, 0, 1).To4(), net.IP(m.SrcAddr))
	require.Equal(t, uint32(443), m.DstPort)
	require.Equal(t, uint64(100), m.Bytes, "one packet's length")
	require.Equal(t, uint64(1024), m.SamplingRate)
	require.Equal(t, uint32(2), m.InIf)
}

// TestSFlowSourceIDPoolAndSequence: source_id is the ingress ifIndex (type 0);
// sequence_number counts samples per source_id, and sample_pool grows by the
// sampling rate per sample (packets that could have been sampled).
func TestSFlowSourceIDPoolAndSequence(t *testing.T) {
	enc := NewEncoder(net.IPv4(192, 0, 2, 1), 0, sfBoot)
	a, b := testFlow(), testFlow()
	b.InputIface = 7
	_, s, _ := decodeSFlow(t, enc.Encode(sfNow, []embed.FlowRecord{a, a, b, a})[0])
	require.Len(t, s, 4)
	for i, want := range []struct{ src, seq, pool uint32 }{{2, 0, 1024}, {2, 1, 2048}, {7, 0, 1024}, {2, 2, 3072}} {
		require.Equal(t, uint32(0), s[i].Header.SourceIdType, "sample %d", i)
		require.Equal(t, want.src, s[i].Header.SourceIdValue, "sample %d", i)
		require.Equal(t, want.seq, s[i].Header.SampleSequenceNumber, "sample %d", i)
		require.Equal(t, want.pool, s[i].SamplePool, "sample %d", i)
	}
}

func TestSFlowUnsampledRateIsOne(t *testing.T) {
	f := testFlow()
	f.SamplingRate = 0
	_, s, _ := decodeSFlow(t, NewEncoder(net.IPv4(192, 0, 2, 1), 0, sfBoot).Encode(sfNow, []embed.FlowRecord{f})[0])
	require.Equal(t, uint32(1), s[0].SamplingRate)
	require.Equal(t, uint32(1), s[0].SamplePool)
}

func TestSFlowSplitsWithinMTU(t *testing.T) {
	flows := make([]embed.FlowRecord, 100)
	for i := range flows {
		flows[i] = testFlow()
	}
	enc := NewEncoder(net.IPv4(192, 0, 2, 1), 0, sfBoot)
	ds := enc.Encode(sfNow, flows)
	require.Greater(t, len(ds), 1)
	got := 0
	for i, d := range ds {
		require.LessOrEqual(t, len(d), wire.MaxPayload)
		p, s, _ := decodeSFlow(t, d)
		require.Equal(t, uint32(i), p.SequenceNumber, "datagram sequence")
		got += len(s)
	}
	require.Equal(t, 100, got)
}

func TestSFlowSampledIPv6(t *testing.T) {
	f := testFlow()
	f.SrcIP, f.DstIP = net.ParseIP("2001:db8::1"), net.ParseIP("2001:db8::2")
	f.TOS = 0x28
	_, s, msgs := decodeSFlow(t, NewEncoder(net.ParseIP("2001:db8::ff"), 0, sfBoot).Encode(sfNow, []embed.FlowRecord{f})[0])
	rec, ok := s[0].Records[0].Data.(gsf.SampledIPv6)
	require.True(t, ok, "sampled_ipv6 (format 4)")
	require.Equal(t, uint32(0x28), rec.Priority)
	require.Equal(t, net.ParseIP("2001:db8::1").To16(), net.IP(msgs[0].SrcAddr))
}

func TestSFlowIPv6Agent(t *testing.T) {
	p, _, _ := decodeSFlow(t, NewEncoder(net.ParseIP("2001:db8::ff"), 0, sfBoot).Encode(sfNow, []embed.FlowRecord{testFlow()})[0])
	require.Equal(t, uint32(2), p.IPVersion, "agent address type IPv6")
	require.Equal(t, net.ParseIP("2001:db8::ff").To16(), net.IP(p.AgentIP))
}

// TestSFlowAgentDefaultsToTheExporter pins that with no configured agent
// address each datagram's agent is its records' exporter, so one sFlow stream
// describes the device that sampled the traffic; a record with no exporter
// falls back to 127.0.0.1.
func TestSFlowAgentDefaultsToTheExporter(t *testing.T) {
	a, b, none := testFlow(), testFlow(), testFlow()
	a.ExporterIP, b.ExporterIP = net.IPv4(10, 0, 0, 1), net.ParseIP("2001:db8::7")
	enc := NewEncoder(nil, 0, sfBoot)
	var agents []string
	for _, d := range enc.Encode(sfNow, []embed.FlowRecord{a, a, b, none}) {
		p, s, _ := decodeSFlow(t, d)
		agents = append(agents, fmt.Sprintf("%s x%d", net.IP(p.AgentIP), len(s)))
	}
	require.Equal(t, []string{"10.0.0.1 x2", "2001:db8::7 x1", "127.0.0.1 x1"}, agents)
}

// TestSFlowSourceCountersArePerAgent pins that a data source's sequence_number
// and sample_pool are scoped to its agent, as sFlow v5 defines a source by
// (agent, sub_agent, source_id): two exporters sharing an ifIndex each count
// from zero, so a collector sees no false sample loss.
func TestSFlowSourceCountersArePerAgent(t *testing.T) {
	a, b := testFlow(), testFlow()
	a.ExporterIP, b.ExporterIP = net.IPv4(10, 0, 0, 1), net.IPv4(10, 0, 0, 2)
	enc := NewEncoder(nil, 0, sfBoot)
	var got []string
	for _, d := range enc.Encode(sfNow, []embed.FlowRecord{a, b, a}) {
		p, s, _ := decodeSFlow(t, d)
		for _, x := range s {
			got = append(got, fmt.Sprintf("%s seq=%d pool=%d", net.IP(p.AgentIP), x.Header.SampleSequenceNumber, x.SamplePool))
		}
	}
	require.Equal(t, []string{
		"10.0.0.1 seq=0 pool=1024",
		"10.0.0.2 seq=0 pool=1024",
		"10.0.0.1 seq=1 pool=2048",
	}, got)
}
