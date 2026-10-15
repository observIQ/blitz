# Flow Output

The flow output exports flow records over UDP in a flow collector's wire
format: NetFlow v5, NetFlow v9, IPFIX, or sFlow v5. It consumes the flow
signal only, so pair it with the `flow` generator. See
[the flow generator](../generator/flow.md) for the record model, per-format
encoding, and vendor flavors.

## Data Mutation

The flow output encodes each batch of flow records into one or more UDP
packets of the selected format and sends them in order to `host:port`. Packets
are sent in the order they are encoded, so a template always precedes the data
that needs it.

## Configuration

| YAML Path | Flag Name | Environment Variable | Default | Description |
|-----------|-----------|---------------------|---------|-------------|
| `output.type` | `--output-type` | `BLITZ_OUTPUT_TYPE` | `nop` | Output type. Set to `flow` to use this output. |
| `output.flow.host` | `--output-flow-host` | `BLITZ_OUTPUT_FLOW_HOST` | `127.0.0.1` | Flow collector host |
| `output.flow.port` | `--output-flow-port` | `BLITZ_OUTPUT_FLOW_PORT` | `2055` | Flow collector UDP port (1-65535). `2055` is the usual NetFlow and IPFIX port; sFlow collectors conventionally listen on `6343`, so set it for `sflow` |
| `output.flow.protocol` | `--output-flow-protocol` | `BLITZ_OUTPUT_FLOW_PROTOCOL` | `netflow-v9` | Wire format: `netflow-v5`, `netflow-v9`, `ipfix`, or `sflow` |
| `output.flow.vendor` | `--output-flow-vendor` | `BLITZ_OUTPUT_FLOW_VENDOR` | `""` | Optional vendor flavor: `jflow`, `netstream`, `cflowd`, `appflow`, or `rflow`; must suit the protocol |
| `output.flow.agentIP` | `--output-flow-agentip` | `BLITZ_OUTPUT_FLOW_AGENTIP` | `""` | sFlow agent address; empty reports each record's exporter (`127.0.0.1` when it names none) |

## Example Configuration

```yaml
generator:
  type: flow
output:
  type: flow
  flow:
    host: collector.example.com
    port: 2055
    protocol: ipfix
```

## Metrics

The flow output counts records it receives in `blitz.output.entries_received`
and failed UDP sends in `blitz.output.send_errors`, both with
`telemetry_type` set to `flows`.
