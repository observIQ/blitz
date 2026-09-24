package netflow

import (
	"encoding/binary"
	"testing"
	"time"

	"github.com/observiq/blitz/embed"
	"github.com/stretchr/testify/require"
)

func TestV9FirstPacketCarriesTemplateAndData(t *testing.T) {
	enc := NewV9Encoder(time.Unix(1699999000, 0), 0, 0)
	pkts := enc.Encode(time.Unix(1700000010, 0), []embed.FlowRecord{testFlow()})
	require.Len(t, pkts, 1)
	pkt := pkts[0]

	require.Equal(t, uint16(9), binary.BigEndian.Uint16(pkt[0:2])) // version
	// count = template records (1) + data records (1) = 2
	require.Equal(t, uint16(2), binary.BigEndian.Uint16(pkt[2:4]))
	require.Equal(t, uint32(0), binary.BigEndian.Uint32(pkt[12:16])) // package_sequence starts at 0

	// First flowset after the 20-byte header is the template flowset (id 0).
	fs := pkt[20:]
	require.Equal(t, uint16(0), binary.BigEndian.Uint16(fs[0:2])) // flowset_id 0 = template
	tmplLen := binary.BigEndian.Uint16(fs[2:4])
	require.Equal(t, templateID, binary.BigEndian.Uint16(fs[4:6])) // template_id
	require.NotZero(t, binary.BigEndian.Uint16(fs[6:8]))           // field count

	// Data flowset follows the template flowset.
	data := fs[tmplLen:]
	require.Equal(t, templateID, binary.BigEndian.Uint16(data[0:2])) // data flowset id = template id
}

func TestV9TemplateRefreshByCount(t *testing.T) {
	// refresh every 3 records; template should reappear on packet 1 and again
	// once 3 records have been sent.
	enc := NewV9Encoder(time.Unix(1699999000, 0), 0, 0)
	enc.refreshRecords = 3
	enc.refreshInterval = time.Hour
	now := time.Unix(1700000010, 0)

	p1 := enc.Encode(now, []embed.FlowRecord{testFlow(), testFlow()}) // 2 records, template included
	require.True(t, hasTemplate(p1[0]))
	p2 := enc.Encode(now, []embed.FlowRecord{testFlow()}) // crosses 3 → template again
	require.True(t, hasTemplate(p2[0]))
	p3 := enc.Encode(now, []embed.FlowRecord{testFlow()}) // no refresh due yet
	require.False(t, hasTemplate(p3[0]))
}

func hasTemplate(pkt []byte) bool {
	return binary.BigEndian.Uint16(pkt[20:22]) == 0
}
