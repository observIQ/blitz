package ipfix

import (
	"bytes"
	"encoding/binary"
	"testing"
	"time"

	nf "github.com/netsampler/goflow2/v2/decoders/netflow"
	"github.com/observiq/blitz/embed"
	"github.com/stretchr/testify/require"
)

// decodeIPFIX decodes an IPFIX message with goflow2 and returns the data records.
func decodeIPFIX(t *testing.T, msg []byte) []nf.DataRecord {
	t.Helper()
	ts := nf.CreateTemplateSystem()
	var v9 nf.NFv9Packet
	var pkt nf.IPFIXPacket
	require.NoError(t, nf.DecodeMessageVersion(bytes.NewBuffer(msg), ts, &v9, &pkt))
	require.Equal(t, uint16(10), pkt.Version)
	var recs []nf.DataRecord
	for _, fs := range pkt.FlowSets {
		if d, ok := fs.(nf.DataFlowSet); ok {
			recs = append(recs, d.Records...)
		}
	}
	return recs
}

func field(rec nf.DataRecord, typ uint16) ([]byte, bool) {
	for _, v := range rec.Values {
		if v.Type == typ {
			b, _ := v.Value.([]byte)
			return b, true
		}
	}
	return nil, false
}

func TestIPFIXRoundTripGoflow2(t *testing.T) {
	enc := NewEncoder(0, nil)
	msg := enc.Encode(time.Unix(1700000010, 0), []embed.FlowRecord{testFlow()})[0]
	recs := decodeIPFIX(t, msg)
	require.Len(t, recs, 1)

	// octetDeltaCount (IE 1) round-trips to 4200 (64-bit in IPFIX).
	b, ok := field(recs[0], 1)
	require.True(t, ok)
	require.Equal(t, uint64(4200), binary.BigEndian.Uint64(b))
	// sourceTransportPort (IE 7) round-trips to 54321.
	sp, ok := field(recs[0], 7)
	require.True(t, ok)
	require.Equal(t, uint16(54321), binary.BigEndian.Uint16(sp))
}

// TestIPFIXICMPTypeCode: ICMP carries icmpTypeCodeIPv4 (IE 32, type*256+code)
// and zero transport ports, which IANA defines only for port-bearing protocols.
func TestIPFIXICMPTypeCode(t *testing.T) {
	f := testFlow()
	f.Protocol, f.TCPFlags, f.SrcPort, f.DstPort = 1, 0, 0, 8<<8 // echo request
	recs := decodeIPFIX(t, NewEncoder(0, nil).Encode(time.Unix(1700000010, 0), []embed.FlowRecord{f})[0])
	require.Len(t, recs, 1)
	tc, ok := field(recs[0], 32)
	require.True(t, ok, "icmpTypeCodeIPv4 present")
	require.Equal(t, uint16(8<<8), binary.BigEndian.Uint16(tc))
	dp, _ := field(recs[0], 11)
	require.Equal(t, uint16(0), binary.BigEndian.Uint16(dp), "destinationTransportPort")

	tcp := decodeIPFIX(t, NewEncoder(0, nil).Encode(time.Unix(1700000010, 0), []embed.FlowRecord{testFlow()})[0])
	tc, _ = field(tcp[0], 32)
	require.Equal(t, uint16(0), binary.BigEndian.Uint16(tc), "non-ICMP carries 0")
}
