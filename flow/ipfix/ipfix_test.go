package ipfix

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
	require.Equal(t, templateID, binary.BigEndian.Uint16(set[4:6]))

	data := set[setLen:]
	require.Equal(t, templateID, binary.BigEndian.Uint16(data[0:2])) // data set id = template id
}

func TestIPFIXSequenceCountsDataRecords(t *testing.T) {
	enc := NewEncoder(0, nil)
	now := time.Unix(1700000010, 0)
	enc.Encode(now, []embed.FlowRecord{testFlow(), testFlow()}) // 2 data records
	msg := enc.Encode(now, []embed.FlowRecord{testFlow()})[0]
	// sequence = number of data records sent before this message = 2
	require.Equal(t, uint32(2), binary.BigEndian.Uint32(msg[8:12]))
}
