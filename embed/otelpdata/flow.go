// Package otelpdata converts blitz signals to raw OpenTelemetry protobuf
// (go.opentelemetry.io/proto/otlp/*). It deliberately does NOT depend on
// go.opentelemetry.io/collector/pdata — blitz's OTLP path is built on the raw
// proto types, which are already a dependency.
//
// FlowToLogRecord projects a FlowRecord onto an OTLP LogRecord with the same
// shape the collector's netflowreceiver emits for a decoded flow
// (receiver/netflowreceiver/parser.go addMessageAttributes): the same
// attribute keys and value semantics, the flow start as the timestamp, the
// receive time as the observed timestamp, and no severity. So a downstream
// consumer sees the log a real collector would produce from the equivalent
// wire packet.
package otelpdata

import (
	"fmt"
	"net"
	"net/netip"
	"sort"
	"time"

	"github.com/observiq/blitz/embed"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	logspb "go.opentelemetry.io/proto/otlp/logs/v1"
)

// IP protocol numbers for ICMP and ICMPv6.
const (
	protoICMP   = 1
	protoICMPv6 = 58
)

// FlowToLogRecord converts a FlowRecord, received at received, to an OTLP
// LogRecord carrying every attribute netflowreceiver's addMessageAttributes
// sets, in its order. Fields a FlowRecord does not model (MACs, VLANs, TTL,
// fragmentation, sequence number, ...) carry the receiver's zero values, and
// flow.type is "unknown" since no wire format is involved; Metadata.Attributes
// can set any of these and add more. ICMP follows the decoded-IPFIX view: no
// ports, type and code in flow.icmp_type/code. Metadata.Timestamp stands in
// for a missing StartTime.
func FlowToLogRecord(f embed.FlowRecord, received time.Time) *logspb.LogRecord {
	srcPort, dstPort := int64(f.SrcPort), int64(f.DstPort)
	var icmpType, icmpCode int64
	if f.Protocol == protoICMP || f.Protocol == protoICMPv6 {
		icmpType, icmpCode = int64(f.DstPort>>8), int64(f.DstPort&0xff)
		srcPort, dstPort = 0, 0
	}
	start := f.StartTime
	if start.IsZero() {
		start = f.Metadata.Timestamp
	}

	attrs := []*commonpb.KeyValue{
		str("source.address", addr(f.SrcIP).String()),
		i64("source.port", srcPort),
		str("destination.address", addr(f.DstIP).String()),
		i64("destination.port", dstPort),
		str("network.transport", transportName(f.Protocol)),
		str("network.type", networkType(f)),
		i64("flow.io.bytes", counter(f.Bytes)),
		i64("flow.io.packets", counter(f.Packets)),
		str("flow.type", "unknown"),
		i64("flow.sequence_num", 0),
		i64("flow.time_received", received.UnixNano()),
		i64("flow.start", unixNano(start)),
		i64("flow.end", unixNano(f.EndTime)),
		i64("flow.sampling_rate", int64(max(f.SamplingRate, 1))),
		str("flow.sampler_address", addr(f.ExporterIP).String()),
		i64("flow.tcp_flags", int64(f.TCPFlags)),
		i64("flow.in_if", int64(f.InputIface)),
		i64("flow.out_if", int64(f.OutputIface)),
		i64("flow.ip_tos", int64(f.TOS)),
		i64("flow.ip_ttl", 0),
		i64("flow.ip_flags", 0),
		i64("flow.fragment_id", 0),
		i64("flow.fragment_offset", 0),
		i64("flow.ipv6_flow_label", 0),
		i64("flow.icmp_type", icmpType),
		i64("flow.icmp_code", icmpCode),
		str("flow.src_mac", "00:00:00:00:00:00"),
		str("flow.dst_mac", "00:00:00:00:00:00"),
		i64("flow.src_vlan", 0),
		i64("flow.dst_vlan", 0),
		i64("flow.vlan_id", 0),
	}
	if nh := addr(f.NextHop); nh.IsValid() {
		attrs = append(attrs, str("flow.next_hop", nh.String()))
	}
	attrs = append(attrs,
		i64("flow.next_hop_as", 0),
		i64("flow.src_as", int64(f.SrcAS)),
		i64("flow.dst_as", int64(f.DstAS)),
		i64("flow.src_net", int64(f.SrcMask)),
		i64("flow.dst_net", int64(f.DstMask)),
		i64("flow.forwarding_status", 0),
		i64("flow.observation_domain_id", 0),
		i64("flow.observation_point_id", 0),
	)
	attrs = mergeAttributes(attrs, f.Metadata.Attributes)

	return &logspb.LogRecord{
		TimeUnixNano:         uint64(unixNano(start)),     // #nosec G115 -- wall-clock ns fits uint64 for any realistic time
		ObservedTimeUnixNano: uint64(received.UnixNano()), // #nosec G115 -- wall-clock ns fits uint64 for any realistic time
		Attributes:           attrs,
	}
}

// mergeAttributes sets each extra attribute, replacing a same-keyed one in
// place and appending new keys in sorted order for a stable record.
func mergeAttributes(attrs []*commonpb.KeyValue, extra map[string]any) []*commonpb.KeyValue {
	if len(extra) == 0 {
		return attrs
	}
	idx := make(map[string]int, len(attrs))
	for i, kv := range attrs {
		idx[kv.Key] = i
	}
	keys := make([]string, 0, len(extra))
	for k := range extra {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		kv := &commonpb.KeyValue{Key: k, Value: anyValue(extra[k])}
		if i, ok := idx[k]; ok {
			attrs[i] = kv
		} else {
			attrs = append(attrs, kv)
		}
	}
	return attrs
}

func anyValue(v any) *commonpb.AnyValue {
	switch x := v.(type) {
	case string:
		return &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: x}}
	case int:
		return &commonpb.AnyValue{Value: &commonpb.AnyValue_IntValue{IntValue: int64(x)}}
	case int64:
		return &commonpb.AnyValue{Value: &commonpb.AnyValue_IntValue{IntValue: x}}
	case float64:
		return &commonpb.AnyValue{Value: &commonpb.AnyValue_DoubleValue{DoubleValue: x}}
	case bool:
		return &commonpb.AnyValue{Value: &commonpb.AnyValue_BoolValue{BoolValue: x}}
	default:
		return &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: fmt.Sprint(x)}}
	}
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

// unixNano is t in Unix nanoseconds, 0 for the zero time (as the receiver
// reports an unset flow timestamp).
func unixNano(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.UnixNano()
}

// addr converts ip the way the receiver does (netip.AddrFromSlice on the
// decoded bytes), unmapping an IPv4 address to its 4-byte form. A nil ip is
// the invalid zero Addr.
func addr(ip net.IP) netip.Addr {
	if v4 := ip.To4(); v4 != nil {
		ip = v4
	}
	a, _ := netip.AddrFromSlice(ip)
	return a
}

// networkType is network.type for the flow's address family, the receiver's
// Ethernet-type name.
func networkType(f embed.FlowRecord) string {
	if (f.SrcIP != nil && f.SrcIP.To4() == nil) || (f.DstIP != nil && f.DstIP.To4() == nil) {
		return "ipv6"
	}
	return "ipv4"
}

// transportName maps an IP protocol number to the OTel network.transport value,
// matching netflowreceiver's getTransportName.
func transportName(proto uint8) string {
	if name, ok := transportNames[proto]; ok {
		return name
	}
	return "unknown"
}
