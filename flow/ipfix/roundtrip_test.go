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

func TestIPFIXVendorRoundTripGoflow2(t *testing.T) {
	cases := []struct {
		name string
		pen  uint32
		ieid uint16
	}{
		{"appflow-citrix", 5951, 130},
		{"jflow-juniper", 2636, 137},
		{"cflowd-nokia", 6527, 91},
		{"netstream-huawei", 2011, 140},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			enc := NewEncoder(0, &Vendor{PEN: tc.pen, IEID: tc.ieid})
			msg := enc.Encode(time.Unix(1700000010, 0), []embed.FlowRecord{testFlow()})[0]
			recs := decodeIPFIX(t, msg)
			require.Len(t, recs, 1)

			var found bool
			for _, v := range recs[0].Values {
				if v.PenProvided && v.Pen == tc.pen {
					found = true
					require.Equal(t, tc.ieid, v.Type, "enterprise IE id")
				}
			}
			require.True(t, found, "decoded record must carry enterprise PEN %d", tc.pen)
		})
	}
}
