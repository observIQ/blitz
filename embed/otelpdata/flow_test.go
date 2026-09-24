package otelpdata

import (
	"net"
	"testing"
	"time"

	"github.com/observiq/blitz/embed"
	"github.com/stretchr/testify/require"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
)

func attrs(rec interface{ GetAttributes() []*commonpb.KeyValue }) map[string]*commonpb.AnyValue {
	m := map[string]*commonpb.AnyValue{}
	for _, kv := range rec.GetAttributes() {
		m[kv.Key] = kv.Value
	}
	return m
}

func TestFlowToLogRecordSemconv(t *testing.T) {
	f := embed.FlowRecord{
		SrcIP: net.IPv4(10, 0, 0, 1), DstIP: net.IPv4(8, 8, 8, 8),
		SrcPort: 54321, DstPort: 443, Protocol: 6, TCPFlags: 0x12,
		Bytes: 4200, Packets: 42, InputIface: 2, OutputIface: 3,
		SrcAS: 64500, DstAS: 15133, TOS: 0, SamplingRate: 1024,
		NextHop:   net.IPv4(10, 0, 0, 254),
		StartTime: time.Unix(1700000000, 0), EndTime: time.Unix(1700000005, 0),
	}
	rec := FlowToLogRecord(f)
	a := attrs(rec)

	require.Equal(t, "10.0.0.1", a[attrSourceAddress].GetStringValue())
	require.Equal(t, int64(54321), a[attrSourcePort].GetIntValue())
	require.Equal(t, "8.8.8.8", a[attrDestinationAddress].GetStringValue())
	require.Equal(t, int64(443), a[attrDestinationPort].GetIntValue())
	require.Equal(t, "tcp", a[attrNetworkTransport].GetStringValue())
	require.Equal(t, "ipv4", a[attrNetworkType].GetStringValue())
	require.Equal(t, int64(4200), a["flow.io.bytes"].GetIntValue())
	require.Equal(t, int64(42), a["flow.io.packets"].GetIntValue())
	require.Equal(t, int64(64500), a["flow.src_as"].GetIntValue())
	require.Equal(t, "10.0.0.254", a["flow.next_hop"].GetStringValue())
	require.Equal(t, uint64(time.Unix(1700000000, 0).UnixNano()), rec.TimeUnixNano)
}

func TestTransportName(t *testing.T) {
	require.Equal(t, "tcp", transportName(6))
	require.Equal(t, "udp", transportName(17))
	require.Equal(t, "icmp", transportName(1))
	require.Equal(t, "unknown", transportName(99))
}
