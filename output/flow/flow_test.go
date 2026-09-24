package flow

import (
	"context"
	"encoding/binary"
	"net"
	"testing"
	"time"

	"github.com/observiq/blitz/embed"
	"github.com/observiq/blitz/output"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestConsumeFlowsSendsEncodedPacket(t *testing.T) {
	srv, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	require.NoError(t, err)
	defer srv.Close()
	_, portStr, _ := net.SplitHostPort(srv.LocalAddr().String())

	o, err := New(zap.NewNop(), "127.0.0.1", portStr, ProtocolNetFlowV5, "", "", embed.TelemetrySettings{})
	require.NoError(t, err)
	defer o.Stop(context.Background())

	rec := embed.FlowRecord{
		SrcIP: net.IPv4(10, 0, 0, 1), DstIP: net.IPv4(8, 8, 8, 8),
		SrcPort: 1234, DstPort: 53, Protocol: 17, Packets: 1, Bytes: 60,
	}
	require.NoError(t, o.ConsumeFlows(context.Background(), []embed.FlowRecord{rec}))

	buf := make([]byte, 2048)
	require.NoError(t, srv.SetReadDeadline(time.Now().Add(2*time.Second)))
	n, _, err := srv.ReadFromUDP(buf)
	require.NoError(t, err)
	require.Equal(t, uint16(5), binary.BigEndian.Uint16(buf[0:2])) // NetFlow v5 version
	require.Equal(t, 24+48, n)                                     // header + one record
}

func TestWriteUnsupported(t *testing.T) {
	o, err := New(zap.NewNop(), "127.0.0.1", "9995", ProtocolIPFIX, "", "", embed.TelemetrySettings{})
	require.NoError(t, err)
	defer o.Stop(context.Background())
	require.ErrorIs(t, o.Write(context.Background(), output.LogRecord{}), output.ErrUnsupportedTelemetryType)
}
