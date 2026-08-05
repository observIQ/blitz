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

The UDP output exposes the following metrics:

- **`blitz_udp_logs_received_total`** (Counter): Number of logs received from the write channel
- **`blitz_udp_workers_active`** (Gauge): Number of active worker goroutines
- **`blitz_udp_log_rate_total`** (Counter, Float64): Rate at which logs are successfully sent to the configured host
- **`blitz_udp_request_size_bytes`** (Histogram): Size of requests in bytes
- **`blitz_udp_send_errors_total`** (Counter): Total number of send errors, labeled by `error_type` (`unknown` or `timeout`)
- **`blitz_udp_channel_size`** (Gauge): Current size of the data channel

All metrics include a `component` label set to `output_udp`.

