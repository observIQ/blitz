# Metrics Documentation

This document describes the metrics exposed by the Blitz application.

## Overview

Blitz exposes OpenTelemetry-compatible metrics via a Prometheus HTTP endpoint. The metrics provide insights into the application's performance, including telemetry generation rates, output throughput, error rates, worker activity, and channel utilization.

All metrics are defined declaratively in YAML registry files and code is generated using [OTel Weaver](https://github.com/open-telemetry/weaver). This ensures consistent naming, descriptions, units, and type-safe wrappers across all components.

## Metrics Endpoint

The metrics are exposed on the following endpoint:

```
http://localhost:9100/metrics
```

### Example: Fetching Metrics with curl

```bash
curl http://localhost:9100/metrics
```

## Naming Convention

Blitz uses a normalized metric naming scheme:

```
blitz.<component>.<metric_name>
```

- **Generator metrics**: `blitz.generator.*` with a required `generator_type` attribute (e.g., `json`, `apache`, `kubernetes`)
- **Output metrics**: `blitz.output.*` with a required `output_type` attribute (e.g., `tcp`, `udp`, `file`, `otlp-grpc`, `hec`)
- **HEC-specific metrics**: `blitz.output.hec.*` for metrics unique to the Splunk HEC output

This approach uses a single set of metric instruments per component type, differentiated by attributes, rather than per-output metric names (e.g., `blitz.output.entries_received{output_type="tcp"}` instead of the old `blitz.tcp.logs.received`).

## Generator Metrics

| Metric | Type | Unit | Description |
|--------|------|------|-------------|
| `blitz.generator.entries` | Counter | `{entry}` | Total number of telemetry entries generated |
| `blitz.generator.active_workers` | Gauge | `{worker}` | Number of active worker goroutines |
| `blitz.generator.write_errors` | Counter | `{error}` | Total number of write errors |

All generator metrics include a required `generator_type` attribute. The `write_errors` metric also supports an optional `error_type` enum attribute with values `unknown` and `timeout`.

For detailed generator metric documentation, see [`generator/monitoring.md`](../generator/monitoring.md).

## Output Metrics

| Metric | Type | Unit | Description |
|--------|------|------|-------------|
| `blitz.output.entries_received` | Counter | `{entry}` | Total number of telemetry entries received by the output |
| `blitz.output.active_workers` | Gauge | `{worker}` | Number of active output worker goroutines |
| `blitz.output.entry_rate` | Float64 Counter | `{entry}/s` | Rate of telemetry entries processed per second |
| `blitz.output.request_size` | Histogram | `By` | Size of output requests in bytes |
| `blitz.output.request_latency` | Float64 Histogram | `s` | Latency of output requests |
| `blitz.output.send_errors` | Counter | `{error}` | Total number of send errors |
| `blitz.output.queue_size` | Observable Gauge | `{entry}` | Current number of entries in the output queue |

All output metrics include a required `output_type` attribute with values such as `tcp`, `udp`, `file`, `otlp-grpc`, `hec`, or `stdout`.

For detailed output metric documentation, see [`output/monitoring.md`](../output/monitoring.md).

## HEC-Specific Metrics

The Splunk HEC output exposes additional metrics for batch and ACK tracking:

| Metric | Type | Unit | Description |
|--------|------|------|-------------|
| `blitz.output.hec.batch_size` | Histogram | `{entry}` | Number of entries per HEC batch |
| `blitz.output.hec.ack_pending` | Gauge | `{ack}` | Number of ACKs currently pending confirmation |
| `blitz.output.hec.ack_confirmed` | Counter | `{ack}` | Total number of ACKs confirmed by the server |
| `blitz.output.hec.ack_expired` | Counter | `{ack}` | Total number of ACKs that expired without confirmation |
| `blitz.output.hec.ack_retried` | Counter | `{ack}` | Total number of batches retried due to ACK failure |
| `blitz.output.hec.ack_dropped` | Counter | `{ack}` | Total number of batches dropped after max retries |
| `blitz.output.hec.ack_poll_latency` | Float64 Histogram | `s` | Latency of ACK polling requests |

These metrics are in addition to the shared output metrics (which use `output_type="hec"`).

## Adding New Metrics

Metrics are defined in YAML registry files and generated using OTel Weaver. To add new metrics:

1. **Define the metric** in the appropriate registry YAML file:
   - Generator metrics: `generator/monitoring/metric.yaml`
   - Output metrics: `output/monitoring/metric.yaml`
   - HEC-specific metrics: `output/hec/monitoring/metric.yaml`

2. **Run code generation**:
   ```bash
   make generate-o11y
   ```
   This generates both Go code (`monitoring.go`) and Markdown documentation (`monitoring.md`) for each registry.

3. **Use the generated wrappers** in your code. For example:
   ```go
   // Output metrics use type-safe wrappers with required attributes
   output.BlitzOutputEntriesReceivedCounter.Add(ctx, 1, "tcp")
   output.BlitzOutputRequestLatencyHistogram.Record(ctx, latency, "tcp")

   // Generator metrics
   generator.BlitzGeneratorEntriesCounter.Add(ctx, 1, "json")
   ```

4. **Verify generation is up to date** (CI runs this automatically):
   ```bash
   make generate-o11y-check
   ```

### Registry YAML Format

Each metric is defined in the `metric.yaml` file within the component's `monitoring/` directory. One
group defines one metric. The instrument, unit, and brief sit directly on the group. This is the
`blitz.output.entries_received` definition from `output/monitoring/metric.yaml`:

```yaml
groups:
- id: output.entries_received
  type: metric
  metric_name: "blitz.output.entries_received"
  stability: stable
  brief: "total number of telemetry entries received by the output"
  instrument: counter
  unit: "{entry}"
  annotations:
    exported: true
  attributes:
    - id: output_type
      type: string
      stability: stable
      requirement_level: required
      brief: "type of output"
      examples: ["tcp", "udp", "file", "otlp-grpc", "hec"]
    - id: telemetry_type
      type: string
      stability: stable
      requirement_level: required
      brief: "telemetry signal type"
      examples: ["logs", "metrics", "traces"]
```
