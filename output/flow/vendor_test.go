package flow

import (
	"bytes"
	"context"
	"net"
	"testing"
	"time"

	nf "github.com/netsampler/goflow2/v2/decoders/netflow"
	"github.com/observiq/blitz/embed"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// sendOne builds the output for protocol/vendor, sends one flow, and returns
// the first datagram the collector receives.
func sendOne(t *testing.T, protocol Protocol, vendor string, f embed.FlowRecord) []byte {
	t.Helper()
	srv, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	require.NoError(t, err)
	t.Cleanup(func() { _ = srv.Close() })
	_, port, _ := net.SplitHostPort(srv.LocalAddr().String())
	o, err := New(zap.NewNop(), "127.0.0.1", port, protocol, vendor, "", embed.TelemetrySettings{})
	require.NoError(t, err)
	t.Cleanup(func() { _ = o.Stop(context.Background()) })
	require.NoError(t, o.ConsumeFlows(context.Background(), []embed.FlowRecord{f}))
	buf := make([]byte, 65535)
	require.NoError(t, srv.SetReadDeadline(time.Now().Add(2*time.Second)))
	n, _, err := srv.ReadFromUDP(buf)
	require.NoError(t, err)
	return buf[:n]
}

func testFlow() embed.FlowRecord {
	return embed.FlowRecord{
		SrcIP: net.IPv4(10, 0, 0, 1), DstIP: net.IPv4(8, 8, 8, 8), SrcPort: 1234, DstPort: 53,
		Protocol: 17, Packets: 1, Bytes: 60, SamplingRate: 1024,
		StartTime: time.Now().Add(-time.Second), EndTime: time.Now(),
	}
}

// enterpriseIEs lists (PEN, IE) pairs in a decoded IPFIX message's records.
func enterpriseIEs(t *testing.T, msg []byte) [][2]uint32 {
	t.Helper()
	var v9 nf.NFv9Packet
	var p nf.IPFIXPacket
	require.NoError(t, nf.DecodeMessageVersion(bytes.NewBuffer(msg), nf.CreateTemplateSystem(), &v9, &p))
	var out [][2]uint32
	for _, fs := range p.FlowSets {
		if d, ok := fs.(nf.DataFlowSet); ok {
			for _, r := range d.Records {
				for _, v := range r.Values {
					if v.PenProvided {
						out = append(out, [2]uint32{v.Pen, uint32(v.Type)})
					}
				}
			}
		}
	}
	return out
}

// TestVendorIEs: only vendors with a published, decoder-known IE mark IPFIX;
// the others (and every vendor on NetFlow v5/v9) are wire-standard.
func TestVendorIEs(t *testing.T) {
	require.Equal(t, [][2]uint32{{2636, 137}}, enterpriseIEs(t, sendOne(t, ProtocolIPFIX, "jflow", testFlow())))
	require.Equal(t, [][2]uint32{{5951, 133}}, enterpriseIEs(t, sendOne(t, ProtocolIPFIX, "appflow", testFlow())))
	for _, v := range []string{"", "netstream", "cflowd"} {
		require.Empty(t, enterpriseIEs(t, sendOne(t, ProtocolIPFIX, v, testFlow())), "vendor %q", v)
	}
}
