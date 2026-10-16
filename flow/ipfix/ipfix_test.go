package ipfix

import (
	"bytes"
	"encoding/binary"
	"net"
	"testing"
	"time"

	nf "github.com/netsampler/goflow2/v2/decoders/netflow"
	protoproducer "github.com/netsampler/goflow2/v2/producer/proto"
	"github.com/stretchr/testify/require"

	"github.com/observiq/blitz/embed"
	"github.com/observiq/blitz/flow/wire"
)

func testFlow() embed.FlowRecord {
	return embed.FlowRecord{
		SrcIP: net.IPv4(10, 0, 0, 1), DstIP: net.IPv4(93, 184, 216, 34),
		SrcPort: 54321, DstPort: 443, Protocol: 6, TCPFlags: 0x1b,
		NextHop: net.IPv4(10, 0, 0, 254), InputIface: 2, OutputIface: 3,
		Packets: 42, Bytes: 4200, SrcAS: 64500, DstAS: 15133, SrcMask: 24, DstMask: 19,
		StartTime: time.Unix(1700000000, 0), EndTime: time.Unix(1700000005, 0),
	}
}

func TestIPFIXFirstMessageHasTemplateAndData(t *testing.T) {
	enc := NewEncoder(0, nil)
	pkts := enc.Encode(time.Unix(1700000010, 0), []embed.FlowRecord{testFlow()})
	require.Len(t, pkts, 1)
	msg := pkts[0]

	require.Equal(t, uint16(10), binary.BigEndian.Uint16(msg[0:2]))         // version
	require.Equal(t, uint16(len(msg)), binary.BigEndian.Uint16(msg[2:4]))   // length = whole message
	require.Equal(t, uint32(1700000010), binary.BigEndian.Uint32(msg[4:8])) // export time
	require.Equal(t, uint32(0), binary.BigEndian.Uint32(msg[8:12]))         // sequence (data records prior)

	set := msg[16:]
	require.Equal(t, uint16(2), binary.BigEndian.Uint16(set[0:2])) // set_id 2 = template
	setLen := binary.BigEndian.Uint16(set[2:4])
	require.Equal(t, templateIDv4, binary.BigEndian.Uint16(set[4:6]))

	// The options template and options data sets precede the data set.
	data := set[int(setLen)+optionsTemplateSetLen+optionsDataSetLen:]
	require.Equal(t, templateIDv4, binary.BigEndian.Uint16(data[0:2])) // data set id = template id
}

// TestCitrixConnectionIDLeavesTheCallersBufferAlone pins that hashing an
// endpoint never writes past an address slice into the memory behind it.
func TestCitrixConnectionIDLeavesTheCallersBufferAlone(t *testing.T) {
	buf := make([]byte, 20)
	copy(buf, net.ParseIP("2001:db8::1").To16())
	buf[16], buf[17] = 0xaa, 0xaa
	f := embed.FlowRecord{SrcIP: net.IP(buf[:16]), DstIP: net.ParseIP("2001:db8::2"), SrcPort: 0x1234, DstPort: 443, Protocol: 6}
	CitrixConnectionID.put(make([]byte, 4), f)
	require.Equal(t, []byte{0xaa, 0xaa}, buf[16:18])
}

var ipfixNow = time.Unix(1700000010, 0)

// collector decodes an IPFIX stream as netflowreceiver does: one template and
// sampling-rate system for the stream, then goflow2's proto producer.
type collector struct {
	t        *testing.T
	ts       nf.NetFlowTemplateSystem
	sampling protoproducer.SamplingRateSystem
}

func newCollector(t *testing.T) *collector {
	return &collector{t: t, ts: nf.CreateTemplateSystem(), sampling: protoproducer.CreateSamplingSystem()}
}

func (c *collector) decode(msg []byte) (nf.IPFIXPacket, []*protoproducer.ProtoProducerMessage) {
	c.t.Helper()
	var v9 nf.NFv9Packet
	var pkt nf.IPFIXPacket
	require.NoError(c.t, nf.DecodeMessageVersion(bytes.NewBuffer(msg), c.ts, &v9, &pkt))
	msgs, err := protoproducer.ProcessMessageIPFIXConfig(&pkt, c.sampling, nil)
	require.NoError(c.t, err)
	out := make([]*protoproducer.ProtoProducerMessage, 0, len(msgs))
	for _, m := range msgs {
		out = append(out, m.(*protoproducer.ProtoProducerMessage))
	}
	return pkt, out
}

// dataRecords returns the decoded data records of msg (for raw IE checks).
func (c *collector) dataRecords(msg []byte) []nf.DataRecord {
	p, _ := c.decode(msg)
	var recs []nf.DataRecord
	for _, fs := range p.FlowSets {
		if d, ok := fs.(nf.DataFlowSet); ok {
			recs = append(recs, d.Records...)
		}
	}
	return recs
}

func v6Flow() embed.FlowRecord {
	f := testFlow()
	f.SrcIP, f.DstIP, f.NextHop = net.ParseIP("2001:db8::1"), net.ParseIP("2001:db8:ffff::2"), net.ParseIP("fe80::1")
	f.SrcMask, f.DstMask = 64, 48
	return f
}

func TestIPFIXSplitsWithinMTU(t *testing.T) {
	flows := make([]embed.FlowRecord, 100)
	for i := range flows {
		flows[i] = testFlow()
	}
	msgs := NewEncoder(0, nil).Encode(ipfixNow, flows)
	require.Greater(t, len(msgs), 1)
	c := newCollector(t)
	got := 0
	for _, m := range msgs {
		require.LessOrEqual(t, len(m), wire.MaxPayload)
		require.Equal(t, uint16(len(m)), binary.BigEndian.Uint16(m[2:4]))
		_, fm := c.decode(m)
		got += len(fm)
	}
	require.Equal(t, 100, got)
}

func TestIPFIXIPv6Flow(t *testing.T) {
	_, msgs := newCollector(t).decode(NewEncoder(0, nil).Encode(ipfixNow, []embed.FlowRecord{v6Flow()})[0])
	require.Len(t, msgs, 1)
	require.Equal(t, net.ParseIP("2001:db8::1").To16(), net.IP(msgs[0].SrcAddr))
	require.Equal(t, net.ParseIP("2001:db8:ffff::2").To16(), net.IP(msgs[0].DstAddr))
	require.Equal(t, uint32(64), msgs[0].SrcNet)
	require.Equal(t, uint32(0x86dd), msgs[0].Etype)
}

func TestIPFIXICMPv6TypeCode(t *testing.T) {
	f := v6Flow()
	f.Protocol, f.TCPFlags, f.SrcPort, f.DstPort = 58, 0, 0, 128<<8 // echo request
	recs := newCollector(t).dataRecords(NewEncoder(0, nil).Encode(ipfixNow, []embed.FlowRecord{f})[0])
	tc, ok := field(recs[0], 139)
	require.True(t, ok, "icmpTypeCodeIPv6 present")
	require.Equal(t, uint16(128<<8), binary.BigEndian.Uint16(tc))
}

func TestIPFIXSamplingRateOption(t *testing.T) {
	enc := NewEncoder(0, nil)
	c := newCollector(t)
	f := testFlow()
	f.SamplingRate = 1024
	_, msgs := c.decode(enc.Encode(ipfixNow, []embed.FlowRecord{f})[0])
	require.Equal(t, uint64(1024), msgs[0].SamplingRate)
	f.SamplingRate = 256
	_, msgs = c.decode(enc.Encode(ipfixNow, []embed.FlowRecord{f})[0])
	require.Equal(t, uint64(256), msgs[0].SamplingRate)
}

func TestIPFIXTCPControlBitsIsUnsigned16(t *testing.T) {
	recs := newCollector(t).dataRecords(NewEncoder(0, nil).Encode(ipfixNow, []embed.FlowRecord{testFlow()})[0])
	b, ok := field(recs[0], 6)
	require.True(t, ok)
	require.Len(t, b, 2)
	require.Equal(t, uint16(0x1b), binary.BigEndian.Uint16(b))
}

func TestIPFIXSequenceCountsDataRecordsIncludingOptions(t *testing.T) {
	enc := NewEncoder(0, nil)
	enc.Encode(ipfixNow, []embed.FlowRecord{testFlow(), testFlow()}) // 1 options + 2 flows
	msg := enc.Encode(ipfixNow, []embed.FlowRecord{testFlow()})[0]
	require.Equal(t, uint32(3), binary.BigEndian.Uint32(msg[8:12]))
}

func TestIPFIXTemplateRefresh(t *testing.T) {
	has := func(m []byte) bool { return binary.BigEndian.Uint16(m[16:18]) == 2 }
	enc := NewEncoder(0, nil)
	enc.refreshRecords = 3
	require.True(t, has(enc.Encode(ipfixNow, []embed.FlowRecord{testFlow(), testFlow()})[0]))
	require.False(t, has(enc.Encode(ipfixNow, []embed.FlowRecord{testFlow()})[0]))
	require.True(t, has(enc.Encode(ipfixNow, []embed.FlowRecord{testFlow()})[0]), "3 records since the template")

	enc = NewEncoder(0, nil)
	require.True(t, has(enc.Encode(ipfixNow, []embed.FlowRecord{testFlow()})[0]))
	require.False(t, has(enc.Encode(ipfixNow.Add(59*time.Second), []embed.FlowRecord{testFlow()})[0]))
	require.True(t, has(enc.Encode(ipfixNow.Add(60*time.Second), []embed.FlowRecord{testFlow()})[0]))
}

// TestIPFIXVendorIEs checks each vendor IE against its Wireshark definition
// (epan/dissectors/packet-netflow.c): Juniper 2636/137 Common Properties
// (uint32, CPID in the top 6 bits) and Citrix NetScaler 5951/133 Connection ID
// (uint32, shared by both flows of a connection).
func TestIPFIXVendorIEs(t *testing.T) {
	ie := func(v *Vendor, f embed.FlowRecord) []byte {
		t.Helper()
		recs := newCollector(t).dataRecords(NewEncoder(0, v).Encode(ipfixNow, []embed.FlowRecord{f})[0])
		for _, val := range recs[0].Values {
			if val.PenProvided && val.Pen == v.PEN && val.Type == v.IEID {
				b, _ := val.Value.([]byte)
				return b
			}
		}
		t.Fatalf("PEN %d IE %d missing", v.PEN, v.IEID)
		return nil
	}

	j := ie(JuniperCommonProperties, testFlow())
	require.Len(t, j, 4)
	require.Equal(t, uint32(1), binary.BigEndian.Uint32(j)>>26, "CPID 1: forwarding class and drop priority")

	fwd := testFlow()
	rev := fwd
	rev.SrcIP, rev.DstIP, rev.SrcPort, rev.DstPort = fwd.DstIP, fwd.SrcIP, fwd.DstPort, fwd.SrcPort
	a, b := ie(CitrixConnectionID, fwd), ie(CitrixConnectionID, rev)
	require.Len(t, a, 4)
	require.NotZero(t, binary.BigEndian.Uint32(a))
	require.Equal(t, a, b, "both directions of a connection share the id")
	other := fwd
	other.SrcPort++
	require.NotEqual(t, a, ie(CitrixConnectionID, other))
}

// TestIPFIXByteExact pins the first message for one IPv4 flow against the
// RFC 7011 layout and the IANA IE widths.
func TestIPFIXByteExact(t *testing.T) {
	f := testFlow()
	f.SamplingRate = 1024
	msg := NewEncoder(7, nil).Encode(ipfixNow, []embed.FlowRecord{f})[0]

	var w []byte
	u8 := func(v uint8) { w = append(w, v) }
	u16 := func(v uint16) { w = binary.BigEndian.AppendUint16(w, v) }
	u32 := func(v uint32) { w = binary.BigEndian.AppendUint32(w, v) }
	u64 := func(v uint64) { w = binary.BigEndian.AppendUint64(w, v) }

	v4 := [][2]uint16{{1, 8}, {2, 8}, {4, 1}, {5, 1}, {6, 2}, {7, 2}, {8, 4}, {9, 1}, {10, 4}, {11, 2}, {12, 4}, {13, 1}, {14, 4}, {15, 4}, {16, 4}, {17, 4}, {152, 8}, {153, 8}, {32, 2}}
	v6 := [][2]uint16{{1, 8}, {2, 8}, {4, 1}, {5, 1}, {6, 2}, {7, 2}, {27, 16}, {29, 1}, {10, 4}, {11, 2}, {28, 16}, {30, 1}, {14, 4}, {62, 16}, {16, 4}, {17, 4}, {152, 8}, {153, 8}, {139, 2}}
	tmplSet := 4 + 4 + 4*len(v4) + 4 + 4*len(v6)
	recLen := 0
	for _, fl := range v4 {
		recLen += int(fl[1])
	}
	optTmplSet := 4 + 6 + 3*4 + 2 // header, (id, count, scope count), 3 fields, pad
	optDataSet := 4 + 9 + 3       // header, record (domain 4, interval 4, algorithm 1), pad
	dataSet := (4 + recLen + 3) &^ 3

	u16(10)
	u16(uint16(16 + tmplSet + optTmplSet + optDataSet + dataSet))
	u32(1700000010)
	u32(0)
	u32(7)
	u16(2)
	u16(uint16(tmplSet))
	for _, tm := range []struct {
		id uint16
		fs [][2]uint16
	}{{256, v4}, {257, v6}} {
		u16(tm.id)
		u16(uint16(len(tm.fs)))
		for _, fl := range tm.fs {
			u16(fl[0])
			u16(fl[1])
		}
	}
	// Options template set (id 3): template 258, 3 fields, 1 scope field
	// observationDomainId (149), then samplingInterval (34) and
	// samplingAlgorithm (35).
	u16(3)
	u16(uint16(optTmplSet))
	u16(258)
	u16(3)
	u16(1)
	u16(149)
	u16(4)
	u16(34)
	u16(4)
	u16(35)
	u16(1)
	u16(0)
	// Options data set: domain 7, 1-in-1024, deterministic (1).
	u16(258)
	u16(uint16(optDataSet))
	u32(7)
	u32(1024)
	u8(1)
	w = append(w, 0, 0, 0)
	// Data set (template 256).
	u16(256)
	u16(uint16(dataSet))
	u64(4200)
	u64(42)
	u8(6)
	u8(0)
	u16(0x1b)
	u16(54321)
	w = append(w, 10, 0, 0, 1)
	u8(24)
	u32(2)
	u16(443)
	w = append(w, 93, 184, 216, 34)
	u8(19)
	u32(3)
	w = append(w, 10, 0, 0, 254)
	u32(64500)
	u32(15133)
	u64(uint64(time.Unix(1700000000, 0).UnixMilli()))
	u64(uint64(time.Unix(1700000005, 0).UnixMilli()))
	u16(0)
	for len(w) < len(msg) && len(w)%4 != 0 {
		u8(0)
	}

	require.Equal(t, w, msg)
}

func TestIPFIXZeroTimesAndNilAddresses(t *testing.T) {
	f := testFlow()
	f.StartTime, f.EndTime = time.Time{}, time.Time{}
	f.SrcIP, f.DstIP, f.NextHop = net.ParseIP("2001:db8::1"), net.ParseIP("2001:db8::2"), nil
	recs := newCollector(t).dataRecords(NewEncoder(0, nil).Encode(ipfixNow, []embed.FlowRecord{f})[0])
	start, _ := field(recs[0], 152)
	require.Zero(t, binary.BigEndian.Uint64(start))
	nh, _ := field(recs[0], 62)
	require.Equal(t, make([]byte, 16), nh, "a nil next hop is all zeros")
}
