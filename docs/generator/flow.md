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
| NetFlow v5  | `netflow-v5`     | Fixed 48-byte records, IPv4 only, ≤30 flows/packet |
| NetFlow v9  | `netflow-v9`     | Template flowsets, periodic template resend |
| IPFIX       | `ipfix`          | RFC 7011; 64-bit counters, absolute ms timestamps |
| sFlow v5    | `sflow`          | Flow samples carrying sampled-IPv4 records |

NetFlow v1/v7/v8 are intentionally omitted (superseded; negligible residual
deployment).

## Vendor flavors

A vendor flavor makes the export genuinely vendor-distinct on the wire — not a
metadata tag. It selects a distinct encoding path whose distinguishing element a
decoder (e.g. netsampler/goflow2, which the collector `netflowreceiver` embeds)
reads back. The flavor is a single field, so flavors are mutually exclusive by
construction, and each must be paired with the protocol whose format carries its
signature.

How the distinction is carried depends on the format:

- **IPFIX** is the only format with a decoder-readable enterprise element: a
  template field with the enterprise bit (`0x8000`) set, followed by the
  vendor's 4-byte IANA Private Enterprise Number (PEN). goflow2 surfaces this as
  the decoded field's PEN. AppFlow, jFlow, and cflowd use this.
- **NetFlow v9** has no PEN mechanism, so a v9 flavor is distinguished by a
  proprietary field type id in the template (goflow2 decodes the field by its
  type). NetStream and rFlow use this.

A vendor may be paired with any format it supports in the real world;
`Validate()` rejects an unsupported pairing. Because NetFlow v5 is a fixed
layout with no vendor mechanism, a v5 export is byte-identical across vendors
(the pairing is allowed for realism but carries no vendor-distinct element).

| `vendor`    | Vendor | Supported formats | IPFIX PEN | v9 field type |
|-------------|--------|-------------------|-----------|---------------|
| `jflow`     | Juniper | `netflow-v5`, `netflow-v9`, `ipfix` | 2636 | `0x9003` |
| `netstream` | Huawei | `netflow-v5`, `netflow-v9`, `ipfix` | 2011 | `0x9001` |
| `cflowd`    | Nokia/Alcatel-Lucent | `netflow-v5`, `netflow-v9`, `ipfix` | 6527 | `0x9004` |
| `appflow`   | Citrix | `ipfix` | 5951 | — |
| `rflow`     | Redback/Ericsson | `netflow-v5`, `netflow-v9` | — | `0x9002` |

Format support sources: Juniper Flow Monitoring (jFlow v5/v9/IPFIX), Huawei
NetStream configuration guide (v5/v9/IPFIX), Nokia SR OS cflowd (v5/v8/v9/IPFIX;
v8 aggregation-only, excluded), Citrix AppFlow (IPFIX application), Redback/Ericsson
SmartEdge rFlow (v5/v9). Round-trip verified against netsampler/goflow2 v2.2.6,
the decoder the collector `netflowreceiver` embeds.

## OTLP output

Beyond the UDP wire formats, flows can be emitted as OTLP logs: set
`output.type: otlp-grpc` with `generator.type: flow`. Each FlowRecord is
projected to an OTLP `LogRecord` with the same network-flow semantic-convention
attributes (`source.address`, `destination.port`, `network.transport`,
`flow.io.bytes`, …) the collector's `netflowreceiver` emits for a decoded flow,
so a downstream consumer sees the same log a real collector would produce. This
path uses raw OTLP protobuf and adds no `collector/pdata` dependency.

## Transport

Flow export is UDP only — every flow collector listens on UDP. The flow output
dials `host:port` and sends each encoded packet.

## Scenario presets

The `scenario` setting shapes the byte/packet distribution and source/destination
spread:

| `scenario`    | Shape |
|---------------|-------|
| `default`     | Broad mix of flow sizes |
| `wan-edge`    | Few, large, bidirectional flows |
| `datacenter`  | Many small flows |
| `ddos-target` | Many sources onto a single destination |

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
| `output.flow.agentIP`  | `--output-flow-agentip`     | `BLITZ_OUTPUT_FLOW_AGENTIP`   | `""`         | sFlow exporter agent IP |

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

host := embed.Host{Flows: myConsumer{}}
```
