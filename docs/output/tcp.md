# TCP Output

The TCP output sends telemetry over TCP connections to a specified host and port. TLS encryption can be enabled for secure transmission.

## Data Mutation

The TCP output does not mutate data; it sends log records as-is over TCP connections. Each log record is sent as a separate message.

## Configuration

| YAML Path | Flag Name | Environment Variable | Default | Description |
|-----------|-----------|---------------------|---------|-------------|
| `output.type` | `--output-type` | `BLITZ_OUTPUT_TYPE` | `nop` | Output type. Set to `tcp` to use this output. |
| `output.tcp.host` | `--output-tcp-host` | `BLITZ_OUTPUT_TCP_HOST` | `""` | TCP target host (IP address or hostname) |
| `output.tcp.port` | `--output-tcp-port` | `BLITZ_OUTPUT_TCP_PORT` | `0` | TCP target port (1-65535) |
| `output.tcp.workers` | `--output-tcp-workers` | `BLITZ_OUTPUT_TCP_WORKERS` | `1` | Number of TCP output workers (must be ≥ 0) |

### TLS Configuration

TLS is disabled by default. To enable TLS, provide both a certificate and private key.

| YAML Path | Flag Name | Environment Variable | Default | Description |
|-----------|-----------|---------------------|---------|-------------|
| `output.tcp.enableTLS` | `--output-tcp-enable-tls` | `BLITZ_OUTPUT_TCP_ENABLE_TLS` | `false` | Enable TLS for TCP connections |
| `output.tcp.tls.cert` | `--output-tcp-tls-cert` | `BLITZ_OUTPUT_TCP_TLS_CERT` | `""` | Path to the TLS certificate file (PEM format) |
| `output.tcp.tls.key` | `--output-tcp-tls-key` | `BLITZ_OUTPUT_TCP_TLS_KEY` | `""` | Path to the TLS private key file (PEM format) |
| `output.tcp.tls.ca` | `--output-tcp-tls-ca` | `BLITZ_OUTPUT_TCP_TLS_CA` | `[]` | Paths to TLS CA certificate files (PEM format). Optional, if not provided the host's root CA set will be used |
| `output.tcp.tls.skipVerify` | `--output-tcp-tls-skip-verify` | `BLITZ_OUTPUT_TCP_TLS_SKIP_VERIFY` | `false` | Whether to skip TLS certificate verification (not recommended for production) |
| `output.tcp.tls.minVersion` | `--output-tcp-tls-min-version` | `BLITZ_OUTPUT_TCP_TLS_MIN_VERSION` | `1.2` | Minimum TLS version. Valid values: `1.2`, `1.3` |

## Example Configuration

### Basic TCP Output

```yaml
output:
  type: tcp
  tcp:
    host: logs.example.com
    port: 9090
    workers: 3
```

### TCP Output with TLS

```yaml
output:
  type: tcp
  tcp:
    host: logs.example.com
    port: 9090
    workers: 3
    enableTLS: true
    tls:
      cert: /path/to/cert.pem
      key: /path/to/key.pem
      ca:
        - /path/to/ca.pem
      skipVerify: false
      minVersion: "1.2"
```

## Metrics

The TCP output exposes the following metrics:

- **`blitz_tcp_logs_received_total`** (Counter): Number of logs received from the write channel
- **`blitz_tcp_workers_active`** (Gauge): Number of active worker goroutines
- **`blitz_tcp_log_rate_total`** (Counter, Float64): Rate at which logs are successfully sent to the configured host
- **`blitz_tcp_request_size_bytes`** (Histogram): Size of requests in bytes
- **`blitz_tcp_request_latency`** (Histogram): Request latency in seconds
- **`blitz_tcp_send_errors_total`** (Counter): Total number of send errors, labeled by `error_type` (`unknown` or `timeout`)
- **`blitz_tcp_channel_size`** (Gauge): Current size of the data channel

All metrics include a `component` label set to `output_tcp`.

