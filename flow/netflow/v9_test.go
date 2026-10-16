package netflow

import (
	"encoding/binary"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/observiq/blitz/embed"
	"github.com/observiq/blitz/flow/wire"
)

func TestV9FirstPacketCarriesTemplateAndData(t *testing.T) {
	enc := NewV9Encoder(time.Unix(1699999000, 0), 0)
	pkts := enc.Encode(time.Unix(1700000010, 0), []embed.FlowRecord{testFlow()})
	require.Len(t, pkts, 1)
	pkt := pkts[0]

	require.Equal(t, uint16(9), binary.BigEndian.Uint16(pkt[0:2])) // version
	// count = 2 templates + 1 options template + 1 options data + 1 data record
	require.Equal(t, uint16(5), binary.BigEndian.Uint16(pkt[2:4]))
	require.Equal(t, uint32(0), binary.BigEndian.Uint32(pkt[12:16])) // package_sequence starts at 0

	// First flowset after the 20-byte header is the template flowset (id 0).
	fs := pkt[20:]
	require.Equal(t, uint16(0), binary.BigEndian.Uint16(fs[0:2])) // flowset_id 0 = template
	tmplLen := binary.BigEndian.Uint16(fs[2:4])
	require.Equal(t, templateIDv4, binary.BigEndian.Uint16(fs[4:6])) // template_id
	require.NotZero(t, binary.BigEndian.Uint16(fs[6:8]))             // field count

	// The options template and options data flowsets precede the data flowset.
	data := fs[int(tmplLen)+v9OptionsTemplateLen+v9OptionsDataLen:]
	require.Equal(t, templateIDv4, binary.BigEndian.Uint16(data[0:2])) // data flowset id = template id
}

func TestV9TemplateRefreshByCount(t *testing.T) {
	// refresh every 3 records: the template leads packet 1 and again the first
	// packet after 3 data records have been sent since it.
	enc := NewV9Encoder(time.Unix(1699999000, 0), 0)
	enc.refreshRecords = 3
	enc.refreshInterval = time.Hour
	now := time.Unix(1700000010, 0)

	p1 := enc.Encode(now, []embed.FlowRecord{testFlow(), testFlow()}) // 2 records, template included
	require.True(t, hasTemplate(p1[0]))
	p2 := enc.Encode(now, []embed.FlowRecord{testFlow()}) // 2 sent since: not due
	require.False(t, hasTemplate(p2[0]))
	p3 := enc.Encode(now, []embed.FlowRecord{testFlow()}) // 3 sent since: due
	require.True(t, hasTemplate(p3[0]))
	p4 := enc.Encode(now, []embed.FlowRecord{testFlow()})
	require.False(t, hasTemplate(p4[0]))
}

func hasTemplate(pkt []byte) bool {
	return binary.BigEndian.Uint16(pkt[20:22]) == 0
}

// TestV9FlowBeforeBootClampsNotWraps: a flow that started before the encoder's
// boot must not wrap FIRST_SWITCHED to ~49 days.
func TestV9FlowBeforeBootClampsNotWraps(t *testing.T) {
	boot := time.Unix(1700000000, 0)
	f := testFlow()
	f.StartTime, f.EndTime = boot.Add(-5*time.Second), boot.Add(time.Second)
	recs := decodeV9(t, NewV9Encoder(boot, 0).Encode(boot.Add(2*time.Second), []embed.FlowRecord{f})[0])
	first, _ := fieldValue(recs[0], 22)
	require.Equal(t, uint32(0), binary.BigEndian.Uint32(first))
}

func newV9(boot time.Time) *V9Encoder { return NewV9Encoder(boot, 0) }

var (
	v9Boot = time.Unix(1699999000, 0)
	v9Now  = time.Unix(1700000010, 0)
)

func v6Flow() embed.FlowRecord {
	f := testFlow()
	f.SrcIP = net.ParseIP("2001:db8::1")
	f.DstIP = net.ParseIP("2001:db8:ffff::2")
	f.NextHop = net.ParseIP("fe80::1")
	f.SrcMask, f.DstMask = 64, 48
	return f
}

func TestV9SplitsWithinMTU(t *testing.T) {
	flows := make([]embed.FlowRecord, 100)
	for i := range flows {
		flows[i] = testFlow()
	}
	pkts := newV9(v9Boot).Encode(v9Now, flows)
	require.Greater(t, len(pkts), 1, "100 records do not fit one MTU")
	c := newV9Collector(t)
	var got int
	for _, p := range pkts {
		require.LessOrEqual(t, len(p), wire.MaxPayload)
		_, msgs := c.decode(p)
		got += len(msgs)
	}
	require.Equal(t, 100, got)
}

func TestV9IPv6Flow(t *testing.T) {
	c := newV9Collector(t)
	_, msgs := c.decode(newV9(v9Boot).Encode(v9Now, []embed.FlowRecord{v6Flow()})[0])
	require.Len(t, msgs, 1)
	m := msgs[0]
	require.Equal(t, net.ParseIP("2001:db8::1").To16(), net.IP(m.SrcAddr))
	require.Equal(t, net.ParseIP("2001:db8:ffff::2").To16(), net.IP(m.DstAddr))
	require.Equal(t, net.ParseIP("fe80::1").To16(), net.IP(m.NextHop))
	require.Equal(t, uint32(64), m.SrcNet)
	require.Equal(t, uint32(48), m.DstNet)
	require.Equal(t, uint32(0x86dd), m.Etype)
}

func TestV9MixedFamiliesKeepOrder(t *testing.T) {
	c := newV9Collector(t)
	var src []string
	for _, p := range newV9(v9Boot).Encode(v9Now, []embed.FlowRecord{testFlow(), v6Flow(), testFlow()}) {
		_, msgs := c.decode(p)
		for _, m := range msgs {
			src = append(src, net.IP(m.SrcAddr).String())
		}
	}
	require.Equal(t, []string{"10.0.0.1", "2001:db8::1", "10.0.0.1"}, src)
}

func TestV9SamplingRateOption(t *testing.T) {
	enc := newV9(v9Boot)
	c := newV9Collector(t)
	f := testFlow()
	f.SamplingRate = 1024
	_, msgs := c.decode(enc.Encode(v9Now, []embed.FlowRecord{f})[0])
	require.Equal(t, uint64(1024), msgs[0].SamplingRate)

	f.SamplingRate = 512 // a rate change re-announces the option
	_, msgs = c.decode(enc.Encode(v9Now, []embed.FlowRecord{f})[0])
	require.Equal(t, uint64(512), msgs[0].SamplingRate)
}

func TestV9FourByteAS(t *testing.T) {
	f := testFlow()
	f.SrcAS, f.DstAS = 4_200_000_000, 4_200_000_001
	_, msgs := newV9Collector(t).decode(newV9(v9Boot).Encode(v9Now, []embed.FlowRecord{f})[0])
	require.Equal(t, uint32(4_200_000_000), msgs[0].SrcAs)
	require.Equal(t, uint32(4_200_000_001), msgs[0].DstAs)
}

func TestV9TemplateRefreshByInterval(t *testing.T) {
	enc := newV9(v9Boot)
	require.True(t, hasTemplate(enc.Encode(v9Now, []embed.FlowRecord{testFlow()})[0]))
	require.False(t, hasTemplate(enc.Encode(v9Now.Add(59*time.Second), []embed.FlowRecord{testFlow()})[0]))
	require.True(t, hasTemplate(enc.Encode(v9Now.Add(60*time.Second), []embed.FlowRecord{testFlow()})[0]))
}

// TestV9ByteExact pins the full first packet for one IPv4 flow, byte for byte,
// against the RFC 3954 layout.
func TestV9ByteExact(t *testing.T) {
	f := testFlow()
	f.SamplingRate = 1024
	pkt := newV9(v9Boot).Encode(v9Now, []embed.FlowRecord{f})[0]

	var want []byte
	u16 := func(v uint16) { want = binary.BigEndian.AppendUint16(want, v) }
	u32 := func(v uint32) { want = binary.BigEndian.AppendUint32(want, v) }
	// Header: version, count (2 templates + 1 options template + 1 options
	// data + 1 data record), sysUptime, unix secs, sequence, source id.
	u16(9)
	u16(5)
	u32(1010 * 1000)
	u32(1700000010)
	u32(0)
	u32(0)
	// Template flowset: IPv4 (256) and IPv6 (257) templates.
	v4 := [][2]uint16{{1, 4}, {2, 4}, {4, 1}, {5, 1}, {6, 1}, {7, 2}, {8, 4}, {9, 1}, {10, 2}, {11, 2}, {12, 4}, {13, 1}, {14, 2}, {15, 4}, {16, 4}, {17, 4}, {21, 4}, {22, 4}, {32, 2}}
	v6 := [][2]uint16{{1, 4}, {2, 4}, {4, 1}, {5, 1}, {6, 1}, {7, 2}, {27, 16}, {29, 1}, {10, 2}, {11, 2}, {28, 16}, {30, 1}, {14, 2}, {62, 16}, {16, 4}, {17, 4}, {21, 4}, {22, 4}, {32, 2}}
	u16(0)
	u16(uint16(4 + 4 + len(v4)*4 + 4 + len(v6)*4))
	for _, tmpl := range []struct {
		id     uint16
		fields [][2]uint16
	}{{256, v4}, {257, v6}} {
		u16(tmpl.id)
		u16(uint16(len(tmpl.fields)))
		for _, fl := range tmpl.fields {
			u16(fl[0])
			u16(fl[1])
		}
	}
	// Options template flowset (id 1): template 258, scope System (1, len 4),
	// options SAMPLING_INTERVAL (34, 4) and SAMPLING_ALGORITHM (35, 1).
	u16(1)
	u16(24)
	u16(258)
	u16(4)
	u16(8)
	u16(1)
	u16(4)
	u16(34)
	u16(4)
	u16(35)
	u16(1)
	u16(0) // padding to 4 bytes
	// Options data flowset: scope value (source id 0), interval 1024,
	// algorithm 1 (deterministic), padded to 4 bytes.
	u16(258)
	u16(16)
	u32(0)
	u32(1024)
	want = append(want, 1, 0, 0, 0)
	// Data flowset (template 256): one 51-byte record, padded to 4 bytes.
	u16(256)
	u16(4 + 51 + 1)
	u32(4200)
	u32(42)
	want = append(want, 6, 0, 0x1b)
	u16(54321)
	want = append(want, 10, 0, 0, 1, 24)
	u16(2)
	u16(443)
	want = append(want, 93, 184, 216, 34, 19)
	u16(3)
	want = append(want, 10, 0, 0, 254)
	u32(64500)
	u32(15133)
	u32(1005 * 1000)       // LAST_SWITCHED
	u32(1000 * 1000)       // FIRST_SWITCHED
	u16(0)                 // ICMP_TYPE (TCP flow)
	want = append(want, 0) // padding

	require.Equal(t, want, pkt)
}
