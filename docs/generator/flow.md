# Network Flow Generator

The flow generator produces network flow records — the classic 5-tuple (source
and destination IP/port plus protocol) with byte/packet counters and routing
dimensions — and the flow output exports them over UDP in the wire format a
collector expects. FlowRecord is blitz's fourth signal type, alongside logs,
metrics, and traces.

The generator is protocol-agnostic: it yields FlowRecords, and the flow output
chooses the wire format. So one generator drives every exporter format.

## Protocol coverage

| Wire format | `protocol` value | Notes |
|-------------|------------------|-------|
| NetFlow v5  | `netflow-v5`     | Fixed 48-byte records, IPv4 only (an IPv6 flow is rejected with an error), at most 29 flows per packet |
| NetFlow v9  | `netflow-v9`     | IPv4 (256) and IPv6 (257) templates, sampling options template (258), periodic template resend |
| IPFIX       | `ipfix`          | RFC 7011; IPv4 and IPv6 templates, 64-bit counters, absolute ms timestamps, sampling options |
| sFlow v5    | `sflow`          | Flow samples carrying `sampled_ipv4` (format 3) or `sampled_ipv6` (format 4) records; IPv4 or IPv6 agent address |

An empty `protocol` means `netflow-v9`, the default. NetFlow v1/v7/v8 are
intentionally omitted (superseded; negligible residual deployment).

### Packet sizing and timing

Every datagram fits a 1500-byte MTU over IPv4 or IPv6 (at most 1452 bytes of
UDP payload), so a large batch is split across packets. NetFlow v9 and IPFIX
also split where the address family or sampling rate changes, since one data
set carries one template. Templates are resent every 60 seconds or every 256
data records, whichever comes first, and the output sends packets in the order
it encoded them, so a template always reaches the collector before the data
that uses it, even with several generator workers. After a failed send, the
next packet carries the templates again, so a collector that missed them does
not drop data until the next scheduled resend.

The simulated exporter booted 72 hours before the stream starts, so NetFlow
v5/v9 and sFlow switch-uptime timestamps are realistic and every flow starts
after boot. A time before boot clamps to 0 instead of wrapping.

### Sampling

`FlowRecord.SamplingRate` is a 1-in-N packet sampling rate (0 means unsampled
and is reported as 1 in 1). The flow generator samples 1 in 1024, so a
collector that scales by the rate multiplies its packet and byte counts by
1024. Each format reports the rate the way its spec defines:

| Format     | Where the rate is carried |
|------------|---------------------------|
| NetFlow v5 | Header `sampling_interval`: mode `01` (packet interval) in the top 2 bits, the rate in the low 14 (saturating at 16383). Records at different rates go in separate packets. |
| NetFlow v9 | Options template (scope System) with `SAMPLING_INTERVAL` (34) and `SAMPLING_ALGORITHM` (35) = deterministic, sent with the templates and whenever the rate changes ([RFC 3954](https://www.rfc-editor.org/rfc/rfc3954) section 6.1) |
| IPFIX      | Options template (scope `observationDomainId`) with `samplingInterval` (34) and `samplingAlgorithm` (35) = deterministic. IANA marks these deprecated in favor of the PSAMP `samplingPacketInterval`/`samplingPacketSpace` pair (305/306), but 34 is the 1-in-N rate collectors read: goflow2 (and so `netflowreceiver`) takes 305 as the rate itself, which would report 1-in-N systematic sampling as 1 in 1. |
| sFlow v5   | Each flow sample's `sampling_rate`, with `sample_pool` growing by the rate per sample |

### sFlow details

A record's packets are its sampled packets, as NetFlow and IPFIX count them,
so each becomes one flow sample (a zero packet count is one sample). A
collector scaling samples by `sampling_rate` then totals the same packets and
bytes as it does from the record over NetFlow or IPFIX. Each sample's
`source_id` is the ingress ifIndex (type 0), and `sequence_number` and
`sample_pool` advance per `source_id`, as
[sFlow v5](https://sflow.org/sflow_version_5.txt) defines them. The sampled
record describes its packet: `length` is the flow's mean packet size
(bytes / packets), and `tcp_flags` is that packet's flags.

## Vendor flavors

A vendor flavor marks an IPFIX export with that vendor's real
enterprise-specific Information Element: a template field with the enterprise
bit (`0x8000`) set and the vendor's IANA Private Enterprise Number (PEN), whose
value is typed as the vendor defines it. Only vendors with a published IE that
a decoder knows carry one; the source for both is Wireshark's
[`packet-netflow.c`](https://gitlab.com/wireshark/wireshark/-/blob/master/epan/dissectors/packet-netflow.c).

| `vendor`    | Vendor | Supported formats | IPFIX enterprise IE |
|-------------|--------|-------------------|---------------------|
| `jflow`     | Juniper | `netflow-v5`, `netflow-v9`, `ipfix` | PEN 2636, IE 137 Common Properties (4 bytes: CPID 1, forwarding class and drop priority) |
| `appflow`   | Citrix | `ipfix` | PEN 5951, IE 133 NetScaler Connection ID (uint32, shared by both directions of a connection) |
| `netstream` | Huawei | `netflow-v5`, `netflow-v9`, `ipfix` | none |
| `cflowd`    | Nokia/Alcatel-Lucent | `netflow-v5`, `netflow-v9`, `ipfix` | none |
| `rflow`     | Redback/Ericsson | `netflow-v5`, `netflow-v9` | none |

NetFlow v5 and v9 have no vendor mechanism, and `netstream`, `cflowd`, and
`rflow` have no published IE, so those exports are wire-standard: byte-identical
to an export with no vendor. `Validate()` still rejects a vendor paired with a
format it does not support in the real world.

Format support sources: Juniper Flow Monitoring (jFlow v5/v9/IPFIX), Huawei
NetStream configuration guide (v5/v9/IPFIX), Nokia SR OS cflowd
(v5/v8/v9/IPFIX; v8 aggregation-only, excluded), Citrix AppFlow (IPFIX
application), Redback/Ericsson SmartEdge rFlow (v5/v9).

Every format is round-trip tested against netsampler/goflow2 v2.2.6, the
decoder the collector `netflowreceiver` embeds.

## OTLP output

Beyond the UDP wire formats, flows can be emitted as OTLP logs: set
`output.type: otlp-grpc` with `generator.type: flow`. Each FlowRecord becomes
an OTLP `LogRecord` shaped like the one the collector's `netflowreceiver`
emits for a decoded flow (`addMessageAttributes` in
[`parser.go`](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/receiver/netflowreceiver/parser.go)):

- The same attribute keys, in the same order (`source.address`,
  `destination.port`, `network.transport`, `network.type`, `flow.io.bytes`,
  `flow.src_net`, `flow.icmp_type`, ...). Fields a FlowRecord does not model
  (MACs, VLANs, TTL, sequence number, ...) carry the receiver's zero values,
  and `flow.type` is `unknown` since no wire format is involved.
- The flow start as the timestamp (falling back to `Metadata.Timestamp`), the
  receive time as the observed timestamp and `flow.time_received`, and no
  severity.
- `flow.sampler_address` is the record's `ExporterIP`, the device the flow came
  from, as the receiver reports the exporter that sent the packet (`invalid IP`
  when a record names none). A flow generator describes one exporter, a
  private address drawn from its seed.
- ICMP flows carry no ports; type and code are in `flow.icmp_type` and
  `flow.icmp_code`.
- `Metadata.Attributes` are added, and can override any of the above (for
  example `flow.type`).

The flow generator stamps each record's `Metadata.Resource` with the simulated
host (`host.name`, `telemetry.source: flow`, and the environment identity when
one is configured). The OTLP output currently sends every log under its own
`service.name: blitz` resource, as it does for other logs.

This path uses raw OTLP protobuf and adds no `collector/pdata` dependency.

## Transport

Flow export is UDP only — every flow collector listens on UDP. The flow output
dials `host:port` and sends each encoded packet.

## Scenario presets

The generator emits IPv4 flows only, since the scenarios have no IP-family
option. The encoders already carry IPv6 records from embedded callers.

The scenario shapes each flow; the flow rate comes from `workers` and `rate`,
and each record is one unidirectional flow. Packet counts are sampled packets
(1 in 1024; see [Sampling](#sampling)).

| `scenario`    | Shape                                  | Packets/flow | Mean packet size |
|---------------|----------------------------------------|--------------|------------------|
| `default`     | Broad mix of flow sizes                | 1-1,000      | min-1,500 B      |
| `wan-edge`    | Few, large, long-lived flows           | 100-10,000   | min-1,500 B      |
| `datacenter`  | Many small flows                       | 1-10         | min-9,000 B (jumbo frames) |
| `ddos-target` | Many sources onto one public destination, drawn from the seed | 1-3          | min-60 B         |

`datacenter` flows run east-west between private (RFC 1918) hosts.
`default` and `wan-edge` flows cross the network edge, one private host and one
public, in either direction. `ddos-target` flows run from public sources to the
public victim. A public host carries a public 16-bit AS number (never AS_TRANS
23456, documentation, or private-use numbers), and a private host carries AS 0.

Flows are mostly TCP (about 80%), with some UDP (15%) and a little ICMP (5%).

Each flow's bytes are its packet count times a mean packet size, so every
record is physically possible. The minimum packet is the protocol's smallest
IP packet: 40 B for TCP (IPv4 + TCP headers) and 28 B for UDP and ICMP. A TCP
flow whose packets never carried ACK (a bare SYN, or the ddos-target flood) is
handshake packets only, at most 60 B each (headers plus TCP options).

TCP and UDP flows run from an ephemeral client port (49152-65535, the IANA
dynamic range) to a service port, weighted toward common services: over TCP,
443 leads, then 80, 22, 8080, 8443, 445, 25, 3389, 3306, 5432, 993, and 6379;
over UDP, 53 leads, then 443 (QUIC), 123, 161, 514, 500, and 4500.
`ddos-target` floods 443.

A flow lasts as long as its packets span on the wire. Each sampled packet
stands for 1,024 real ones, and those arrive 1-10 ms apart, so a flow lasts at
least 1 ms per gap between its real packets, up to a 60 s active timeout.

TCP flags are the cumulative OR over the flow, as NetFlow and IPFIX carry them.
A connection's packets add `SYN`, `ACK`, `PSH`, `FIN` in that order, so a flow
carries only the flags its packets could: 1 packet is `SYN` (`0x02`), 2 add
`ACK` (`0x12`), 3 add `PSH` (`0x1a`), and 4 or more are a completed connection,
`SYN|ACK|PSH|FIN` (`0x1b`). `ddos-target` TCP flows are `SYN` (a SYN flood). UDP and ICMP flows carry `0`. sFlow
samples packets, so each `sampled_ipv4` record carries one packet's length
(bytes / packets, with a zero packet count treated as one packet and the
length capped at the 65,535-byte IPv4 maximum) and that packet's flags: the
first is `SYN`, the second `ACK`, later ones `PSH|ACK`, and the last `FIN|ACK`
when the flow closed, so they OR back to the record's flags. A SYN flood's
packets are all `SYN`.

ICMP flows carry real messages: echo request (8/0), echo reply (0/0),
destination unreachable (3/0, 3/1, 3/3), and time exceeded (11/0), and
`ddos-target` ICMP is an echo-request (ping) flood. ICMP has no ports, so each
format carries type and code its own way:

| Format        | Source port | Destination port | ICMP field |
|---------------|-------------|------------------|------------|
| NetFlow v5    | `0`         | type*256+code    | none (the fixed layout has no ICMP field; the port carries it by convention) |
| NetFlow v9    | `0`         | type*256+code    | `ICMP_TYPE` (32) = type*256+code, [RFC 3954](https://www.rfc-editor.org/rfc/rfc3954) |
| IPFIX         | `0`         | `0`              | `icmpTypeCodeIPv4` (IE 32), or `icmpTypeCodeIPv6` (IE 139) for ICMPv6, = type*256+code; IANA defines IE 7/11 only for port-bearing transports, [IANA IPFIX registry](https://www.iana.org/assignments/ipfix/ipfix.xhtml) |
| sFlow v5      | `0`         | type             | none; the [sFlow v5 spec](https://sflow.org/sflow_version_5.txt) says "port number or equivalent" without an ICMP encoding, and the reference decoder, [sflowtool](https://github.com/sflow/sflowtool), reads the type from `dst_port`. The code is not carried. |

## Configuration

Generator:

| YAML Path                  | Flag                          | Env                             | Default   | Description |
|----------------------------|-------------------------------|---------------------------------|-----------|-------------|
| `generator.flow.workers`   | `--generator-flow-workers`    | `BLITZ_GENERATOR_FLOW_WORKERS`  | `1`       | Worker goroutines |
| `generator.flow.rate`      | `--generator-flow-rate`       | `BLITZ_GENERATOR_FLOW_RATE`     | `1s`      | Interval per worker |
| `generator.flow.scenario`  | `--generator-flow-scenario`   | `BLITZ_GENERATOR_FLOW_SCENARIO` | `default` | Traffic-shape preset |
| `generator.flow.seed`      | `--generator-flow-seed`       | `BLITZ_GENERATOR_FLOW_SEED`     | `-1`      | RNG seed (negative randomizes) |

Output:

| YAML Path              | Flag                        | Env                           | Default      | Description |
|------------------------|-----------------------------|-------------------------------|--------------|-------------|
| `output.flow.host`     | `--output-flow-host`        | `BLITZ_OUTPUT_FLOW_HOST`      | `127.0.0.1`  | Collector host |
| `output.flow.port`     | `--output-flow-port`        | `BLITZ_OUTPUT_FLOW_PORT`      | `2055`       | Collector UDP port |
| `output.flow.protocol` | `--output-flow-protocol`    | `BLITZ_OUTPUT_FLOW_PROTOCOL`  | `netflow-v9` | Wire format |
| `output.flow.vendor`   | `--output-flow-vendor`      | `BLITZ_OUTPUT_FLOW_VENDOR`    | `""`         | Optional vendor flavor |
| `output.flow.agentIP`  | `--output-flow-agentip`     | `BLITZ_OUTPUT_FLOW_AGENTIP`   | `""`         | sFlow exporter agent IP; empty reports each record's exporter (`127.0.0.1` when it names none) |

## Example

```yaml
generator:
  type: flow
  flow:
    workers: 2
    rate: 500ms
    scenario: datacenter
output:
  type: flow
  flow:
    host: collector.example.com
    port: 2055
    protocol: netflow-v9
```

## Embedding

A host consumes flows in-process by implementing `embed.FlowConsumer`:

```go
type myConsumer struct{}

func (myConsumer) ConsumeFlows(ctx context.Context, recs []embed.FlowRecord) error {
    for _, r := range recs {
        // r.SrcIP, r.DstIP, r.SrcPort, r.DstPort, r.Protocol, r.Bytes, r.Packets ...
    }
    return nil
}

modules, err := blitzconfig.LoadModules(yamlBytes, blitzconfig.EmbedOpts{
    Logger:       logger,
    FlowConsumer: myConsumer{},
})
if err != nil { /* ... */ }
runner, err := embed.New(embed.Config{Modules: modules})
```

`LoadModules` wires every flow generator to `EmbedOpts.FlowConsumer` and
rejects a config with a flow generator when it is nil. See
[Embedding blitz](../embed.md).
