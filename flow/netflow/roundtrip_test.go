package netflow

import (
	"bytes"
	"encoding/binary"
	"testing"
	"time"

	nf "github.com/netsampler/goflow2/v2/decoders/netflow"
	nfl "github.com/netsampler/goflow2/v2/decoders/netflowlegacy"
	"github.com/observiq/blitz/embed"
	"github.com/stretchr/testify/require"
)

// TestV5RoundTripGoflow2 encodes with blitz and decodes with the exact decoder
// the shipped collector runs (netsampler/goflow2 netflowlegacy), asserting the
// decoded flow matches the input.
func TestV5RoundTripGoflow2(t *testing.T) {
	enc := NewV5Encoder(time.Unix(1699999000, 0))
	pkt := enc.Encode(time.Unix(1700000010, 0), []embed.FlowRecord{testFlow()})[0]

	var p nfl.PacketNetFlowV5
	require.NoError(t, nfl.DecodeMessageVersion(bytes.NewBuffer(pkt), &p))
	require.Equal(t, uint16(5), p.Version)
	require.Len(t, p.Records, 1)
	r := p.Records[0]
	require.Equal(t, uint32(0x0A000001), uint32(r.SrcAddr)) // 10.0.0.1
	require.Equal(t, uint16(443), r.DstPort)
	require.Equal(t, uint32(42), r.DPkts)
	require.Equal(t, uint32(4200), r.DOctets)
	require.Equal(t, uint8(6), r.Proto)
}

// decodeV9 decodes a v9 packet with goflow2 and returns the data records.
func decodeV9(t *testing.T, pkt []byte) []nf.DataRecord {
	t.Helper()
	ts := nf.CreateTemplateSystem()
	var v9 nf.NFv9Packet
	var ipfix nf.IPFIXPacket
	require.NoError(t, nf.DecodeMessageVersion(bytes.NewBuffer(pkt), ts, &v9, &ipfix))
	require.Equal(t, uint16(9), v9.Version)
	var recs []nf.DataRecord
	for _, fs := range v9.FlowSets {
		if d, ok := fs.(nf.DataFlowSet); ok {
			recs = append(recs, d.Records...)
		}
	}
	return recs
}

func fieldValue(rec nf.DataRecord, typ uint16) ([]byte, bool) {
	for _, v := range rec.Values {
		if v.Type == typ {
			b, _ := v.Value.([]byte)
			return b, true
		}
	}
	return nil, false
}

func TestV9RoundTripGoflow2(t *testing.T) {
	enc := NewV9Encoder(time.Unix(1699999000, 0), 0, 0)
	pkt := enc.Encode(time.Unix(1700000010, 0), []embed.FlowRecord{testFlow()})[0]
	recs := decodeV9(t, pkt)
	require.Len(t, recs, 1)

	// IN_BYTES (type 1) round-trips to 4200.
	b, ok := fieldValue(recs[0], 1)
	require.True(t, ok)
	require.Equal(t, uint32(4200), binary.BigEndian.Uint32(b))
	// L4_SRC_PORT (type 7) round-trips to 54321.
	sp, ok := fieldValue(recs[0], 7)
	require.True(t, ok)
	require.Equal(t, uint16(54321), binary.BigEndian.Uint16(sp))
}

func TestV9VendorRoundTripGoflow2(t *testing.T) {
	cases := []struct {
		name  string
		field uint16
	}{
		{"netstream-huawei", 0x9001},
		{"rflow-redback", 0x9002},
		{"jflow-juniper", 0x9003},
		{"cflowd-nokia", 0x9004},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			enc := NewV9Encoder(time.Unix(1699999000, 0), 0, tc.field)
			pkt := enc.Encode(time.Unix(1700000010, 0), []embed.FlowRecord{testFlow()})[0]
			recs := decodeV9(t, pkt)
			require.Len(t, recs, 1)
			_, ok := fieldValue(recs[0], tc.field)
			require.True(t, ok, "decoded record must carry the vendor field type 0x%x", tc.field)
		})
	}
}
