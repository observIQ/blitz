# UDP Output

The UDP output sends telemetry over UDP connections to a specified host and port.

## Data Mutation

The UDP output does not mutate data; it sends log records as-is over UDP connections. Each log record is sent as a separate datagram.

## Configuration

| YAML Path | Flag Name | Environment Variable | Default | Description |
|-----------|-----------|---------------------|---------|-------------|
| `output.type` | `--output-type` | `BLITZ_OUTPUT_TYPE` | `nop` | Output type. Set to `udp` to use this output. |
| `output.udp.host` | `--output-udp-host` | `BLITZ_OUTPUT_UDP_HOST` | `""` | UDP target host (IP address or hostname) |
| `output.udp.port` | `--output-udp-port` | `BLITZ_OUTPUT_UDP_PORT` | `0` | UDP target port (1-65535) |
| `output.udp.workers` | `--output-udp-workers` | `BLITZ_OUTPUT_UDP_WORKERS` | `1` | Number of UDP output workers (must be ≥ 0) |

## Example Configuration

```yaml
output:
  type: udp
  udp:
    host: logs.example.com
    port: 514
    workers: 5
```

## Metrics

The UDP output records the shared output metrics:

- **`blitz.output.entries_received`** (Counter): Entries received by the output
- **`blitz.output.active_workers`** (Gauge): Number of active worker goroutines
- **`blitz.output.entry_rate`** (Float64 Counter): Rate at which entries are sent
- **`blitz.output.request_size`** (Histogram): Size of requests in bytes
- **`blitz.output.send_errors`** (Counter): Total number of send errors, by `telemetry_type`
- **`blitz.output.queue_size`** (Observable Gauge): Current number of entries in the queue

All metrics carry an `output_type` attribute set to `udp`. See [Metrics](../metrics.md) for the exported names.

