// Package otelpdata converts blitz signals to raw OpenTelemetry protobuf
// (go.opentelemetry.io/proto/otlp/*). It deliberately does NOT depend on
// go.opentelemetry.io/collector/pdata — blitz's OTLP path is built on the raw
// proto types, which are already a dependency.
//
// FlowToLogRecord projects a FlowRecord onto an OTLP LogRecord using the same
// attribute keys the collector's netflowreceiver emits for a decoded flow
// (receiver/netflowreceiver/parser.go addMessageAttributes), so a downstream
// consumer sees the same log a real collector would produce from the equivalent
// wire packet.
package otelpdata

import (
	"net"

	"github.com/observiq/blitz/embed"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	logspb "go.opentelemetry.io/proto/otlp/logs/v1"
)

// OTel semantic-convention attribute keys shared with netflowreceiver.
const (
	attrSourceAddress      = "source.address"
	attrSourcePort         = "source.port"
	attrDestinationAddress = "destination.address"
	attrDestinationPort    = "destination.port"
	attrNetworkTransport   = "network.transport"
	attrNetworkType        = "network.type"
)

// FlowToLogRecord converts a FlowRecord to an OTLP LogRecord with network-flow
// semantic-convention attributes matching netflowreceiver's output.
func FlowToLogRecord(f embed.FlowRecord) *logspb.LogRecord {
	attrs := []*commonpb.KeyValue{
		str(attrSourceAddress, ipString(f.SrcIP)),
		i64(attrSourcePort, int64(f.SrcPort)),
		str(attrDestinationAddress, ipString(f.DstIP)),
		i64(attrDestinationPort, int64(f.DstPort)),
		str(attrNetworkTransport, transportName(f.Protocol)),
		str(attrNetworkType, "ipv4"),
		i64("flow.io.bytes", counter(f.Bytes)),
		i64("flow.io.packets", counter(f.Packets)),
		i64("flow.tcp_flags", int64(f.TCPFlags)),
		i64("flow.ip_tos", int64(f.TOS)),
		i64("flow.in_if", int64(f.InputIface)),
		i64("flow.out_if", int64(f.OutputIface)),
		i64("flow.src_as", int64(f.SrcAS)),
		i64("flow.dst_as", int64(f.DstAS)),
		i64("flow.sampling_rate", int64(f.SamplingRate)),
	}
	if len(f.NextHop) > 0 {
		attrs = append(attrs, str("flow.next_hop", ipString(f.NextHop)))
	}

	rec := &logspb.LogRecord{
		SeverityNumber: logspb.SeverityNumber_SEVERITY_NUMBER_INFO,
		SeverityText:   "INFO",
		Attributes:     attrs,
	}
	if !f.StartTime.IsZero() {
		rec.TimeUnixNano = uint64(f.StartTime.UnixNano()) // #nosec G115 -- wall-clock ns fits uint64 for any realistic time
	}
	if !f.EndTime.IsZero() {
		rec.Attributes = append(rec.Attributes, i64("flow.end", f.EndTime.UnixNano()))
	}
	if !f.StartTime.IsZero() {
		rec.Attributes = append(rec.Attributes, i64("flow.start", f.StartTime.UnixNano()))
	}
	return rec
}

func str(k, v string) *commonpb.KeyValue {
	return &commonpb.KeyValue{Key: k, Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: v}}}
}

func i64(k string, v int64) *commonpb.KeyValue {
	return &commonpb.KeyValue{Key: k, Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_IntValue{IntValue: v}}}
}

// counter narrows a uint64 flow counter to the int64 an OTLP attribute holds,
// matching netflowreceiver, which does the same int64(pm.Bytes) conversion.
func counter(v uint64) int64 {
	return int64(v) // #nosec G115 -- semconv counters are int64; matches netflowreceiver
}

func ipString(ip net.IP) string {
	if len(ip) == 0 {
		return ""
	}
	return ip.String()
}

// transportName maps an IP protocol number to the OTel network.transport value,
// matching netflowreceiver's getTransportName for the common cases.
func transportName(proto uint8) string {
	switch proto {
	case 1:
		return "icmp"
	case 6:
		return "tcp"
	case 17:
		return "udp"
	case 58:
		return "ipv6-icmp"
	default:
		return "unknown"
	}
}
