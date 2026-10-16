package otelpdata

import (
	"net"
	"testing"
	"time"

	"github.com/observiq/blitz/embed"
	"github.com/stretchr/testify/require"
	logspb "go.opentelemetry.io/proto/otlp/logs/v1"
)

// receiverKeys is every attribute key netflowreceiver's addMessageAttributes
// (receiver/netflowreceiver/parser.go) sets on a flow log, in its order;
// flow.next_hop and flow.bgp_next_hop only when the address is valid.
var receiverKeys = []string{
	"source.address", "source.port", "destination.address", "destination.port",
	"network.transport", "network.type",
	"flow.io.bytes", "flow.io.packets", "flow.type", "flow.sequence_num",
	"flow.time_received", "flow.start", "flow.end", "flow.sampling_rate",
	"flow.sampler_address", "flow.tcp_flags", "flow.in_if", "flow.out_if",
	"flow.ip_tos", "flow.ip_ttl", "flow.ip_flags", "flow.fragment_id",
	"flow.fragment_offset", "flow.ipv6_flow_label",
	"flow.icmp_type", "flow.icmp_code",
	"flow.src_mac", "flow.dst_mac", "flow.src_vlan", "flow.dst_vlan", "flow.vlan_id",
	"flow.next_hop", "flow.next_hop_as", "flow.src_as", "flow.dst_as",
	"flow.src_net", "flow.dst_net", "flow.forwarding_status",
	"flow.observation_domain_id", "flow.observation_point_id",
}

func keys(rec *logspb.LogRecord) []string {
	var k []string
	for _, kv := range rec.GetAttributes() {
		k = append(k, kv.Key)
	}
	return k
}

func tcpFlow() embed.FlowRecord {
	return embed.FlowRecord{
		SrcIP: net.IPv4(10, 0, 0, 1), DstIP: net.IPv4(8, 8, 8, 8),
		SrcPort: 54321, DstPort: 443, Protocol: 6, TCPFlags: 0x1b,
		Bytes: 4200, Packets: 42, InputIface: 2, OutputIface: 3,
		SrcAS: 64500, DstAS: 15133, SrcMask: 24, DstMask: 16, SamplingRate: 1024,
		NextHop:   net.IPv4(10, 0, 0, 254),
		StartTime: time.Unix(1700000000, 0), EndTime: time.Unix(1700000005, 0),
	}
}

func TestFlowLogMatchesReceiverAttributeSet(t *testing.T) {
	rec := FlowToLogRecord(tcpFlow(), time.Unix(1700000006, 0))
	// Same keys in the same order as the receiver's addMessageAttributes.
	require.Equal(t, receiverKeys, keys(rec))

	a := attrs(rec)
	require.Equal(t, logspb.SeverityNumber_SEVERITY_NUMBER_UNSPECIFIED, rec.SeverityNumber, "the receiver sets no severity")
	require.Empty(t, rec.SeverityText)
	require.Equal(t, int64(time.Unix(1700000006, 0).UnixNano()), a["flow.time_received"].GetIntValue())
	require.Equal(t, uint64(time.Unix(1700000006, 0).UnixNano()), rec.ObservedTimeUnixNano)
	require.Equal(t, int64(24), a["flow.src_net"].GetIntValue())
	require.Equal(t, int64(16), a["flow.dst_net"].GetIntValue())
	require.Equal(t, "unknown", a["flow.type"].GetStringValue(), "no wire format on the OTLP path")
	require.Equal(t, int64(0), a["flow.sequence_num"].GetIntValue())
	require.Equal(t, int64(1024), a["flow.sampling_rate"].GetIntValue())
	require.Equal(t, "00:00:00:00:00:00", a["flow.src_mac"].GetStringValue())
}

func TestFlowLogICMP(t *testing.T) {
	f := tcpFlow()
	f.Protocol, f.TCPFlags, f.SrcPort, f.DstPort = 1, 0, 0, 3<<8|3
	a := attrs(FlowToLogRecord(f, time.Unix(1700000006, 0)))
	require.Equal(t, int64(3), a["flow.icmp_type"].GetIntValue())
	require.Equal(t, int64(3), a["flow.icmp_code"].GetIntValue())
	require.Equal(t, int64(0), a["destination.port"].GetIntValue(), "ICMP carries no port")
	require.Equal(t, int64(0), a["source.port"].GetIntValue())
}

func TestFlowLogIPv6AndNoNextHop(t *testing.T) {
	f := tcpFlow()
	f.SrcIP, f.DstIP, f.NextHop = net.ParseIP("2001:db8::1"), net.ParseIP("2001:db8::2"), nil
	rec := FlowToLogRecord(f, time.Unix(1700000006, 0))
	a := attrs(rec)
	require.Equal(t, "ipv6", a["network.type"].GetStringValue())
	require.Equal(t, "2001:db8::1", a["source.address"].GetStringValue())
	_, has := a["flow.next_hop"]
	require.False(t, has, "an invalid next hop is omitted, as the receiver does")
}

// TestFlowLogUsesMetadata: Metadata.Timestamp stands in for a missing
// StartTime, and Metadata.Attributes are carried onto the log.
func TestFlowLogUsesMetadata(t *testing.T) {
	f := tcpFlow()
	f.StartTime = time.Time{}
	f.Metadata = embed.FlowRecordMetadata{
		Timestamp:  time.Unix(1700000003, 0),
		Attributes: map[string]any{"flow.type": "netflow_v9", "site": "edge-1", "flow.sequence_num": int64(9)},
	}
	rec := FlowToLogRecord(f, time.Unix(1700000006, 0))
	a := attrs(rec)
	require.Equal(t, uint64(time.Unix(1700000003, 0).UnixNano()), rec.TimeUnixNano)
	require.Equal(t, "netflow_v9", a["flow.type"].GetStringValue())
	require.Equal(t, "edge-1", a["site"].GetStringValue())
	require.Equal(t, int64(9), a["flow.sequence_num"].GetIntValue())
}

func TestFlowLogValueKinds(t *testing.T) {
	f := tcpFlow()
	f.Protocol, f.EndTime = 17, time.Time{}
	f.Metadata.Attributes = map[string]any{"i": 3, "f": 1.5, "b": true, "o": []int{1}}
	a := attrs(FlowToLogRecord(f, time.Unix(1700000006, 0)))
	require.Equal(t, "udp", a["network.transport"].GetStringValue())
	require.Equal(t, int64(0), a["flow.end"].GetIntValue(), "unset end is 0")
	require.Equal(t, int64(3), a["i"].GetIntValue())
	require.Equal(t, 1.5, a["f"].GetDoubleValue())
	require.True(t, a["b"].GetBoolValue())
	require.Equal(t, "[1]", a["o"].GetStringValue())
	require.Equal(t, "ipv6-icmp", transportName(58))
}

// TestTransportNameMatchesReceiver pins network.transport to netflowreceiver's
// IANA protocol-name table (getTransportName in parser.go), not just the
// common transports.
func TestTransportNameMatchesReceiver(t *testing.T) {
	for proto, want := range map[uint8]string{
		0: "hopopt", 2: "igmp", 47: "gre", 50: "esp", 51: "ah", 89: "ospfigp",
		112: "vrrp", 132: "sctp", 145: "nsh", 146: "unknown", 255: "unknown",
	} {
		require.Equal(t, want, transportName(proto), "protocol %d", proto)
	}
}

// TestSamplerAddressIsTheExporter pins flow.sampler_address to the exporting
// device, as netflowreceiver reports the packet's source; "invalid IP" (the
// receiver's rendering of no address) only when the record names none.
func TestSamplerAddressIsTheExporter(t *testing.T) {
	f := tcpFlow()
	f.ExporterIP = net.IPv4(10, 1, 2, 3)
	require.Equal(t, "10.1.2.3", attrs(FlowToLogRecord(f, time.Unix(1700000006, 0)))["flow.sampler_address"].GetStringValue())
	f.ExporterIP = nil
	require.Equal(t, "invalid IP", attrs(FlowToLogRecord(f, time.Unix(1700000006, 0)))["flow.sampler_address"].GetStringValue())
}
