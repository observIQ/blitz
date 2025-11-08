# OTLP gRPC Output

The OTLP gRPC output sends logs to an OpenTelemetry collector via gRPC using the OTLP protocol. Logs are batched and sent as OTLP log records with resource and instrumentation scope information.

## Data Mutation

The OTLP gRPC output transforms log records into OTLP format. Each log record is converted to an OTLP LogRecord with:
- Timestamp from the log record metadata
- Severity level mapped to OTLP severity levels
- Body containing the log message
- Attributes extracted from parsed log data (if available)

Logs are batched together and sent in ExportLogsServiceRequest messages per the OTLP specification.

## Configuration

| YAML Path | Flag Name | Environment Variable | Default | Description |
|-----------|-----------|---------------------|---------|-------------|
| `output.type` | `--output-type` | `BLITZ_OUTPUT_TYPE` | `nop` | Output type. Set to `otlp-grpc` to use this output. |
| `output.otlpGrpc.host` | `--output-otlpgrpc-host` | `BLITZ_OUTPUT_OTLPGRPC_HOST` | `localhost` | OTLP gRPC target host (IP address or hostname) |
| `output.otlpGrpc.port` | `--output-otlpgrpc-port` | `BLITZ_OUTPUT_OTLPGRPC_PORT` | `4317` | OTLP gRPC target port (1-65535) |
| `output.otlpGrpc.workers` | `--output-otlpgrpc-workers` | `BLITZ_OUTPUT_OTLPGRPC_WORKERS` | `1` | Number of OTLP gRPC output workers (must be ≥ 0) |
| `output.otlpGrpc.batchTimeout` | `--output-otlpgrpc-batchtimeout` | `BLITZ_OUTPUT_OTLPGRPC_BATCHTIMEOUT` | `1s` | Timeout for batching log records before sending (duration format) |
| `output.otlpGrpc.maxQueueSize` | `--output-otlpgrpc-maxqueuesize` | `BLITZ_OUTPUT_OTLPGRPC_MAXQUEUESIZE` | `100` | Maximum queue size for batching logs (must be ≥ 0) |
| `output.otlpGrpc.maxExportBatchSize` | `--output-otlpgrpc-maxexportbatchsize` | `BLITZ_OUTPUT_OTLPGRPC_MAXEXPORTBATCHSIZE` | `200` | Maximum number of logs per export batch (must be ≥ 0) |

### TLS Configuration

By default, OTLP gRPC uses insecure credentials (no TLS). To enable TLS, set `insecure` to `false` and provide certificate and key files.

| YAML Path | Flag Name | Environment Variable | Default | Description |
|-----------|-----------|---------------------|---------|-------------|
| `output.otlpGrpc.enableTLS` | `--output-otlpgrpc-enable-tls` | `BLITZ_OUTPUT_OTLPGRPC_ENABLE_TLS` | `false` | Enable TLS for OTLP gRPC connections |
| `output.otlpGrpc.tls.insecure` | `--otlp-grpc-tls-insecure` | `BLITZ_OUTPUT_OTLPGRPC_TLS_INSECURE` | `true` | Whether to use insecure credentials (no TLS). When `true`, TLS is not used. When `false` and TLS certificates are provided, TLS will be enabled |
| `output.otlpGrpc.tls.cert` | `--otlp-grpc-tls-cert` | `BLITZ_OUTPUT_OTLPGRPC_TLS_CERT` | `""` | Path to the TLS certificate file (PEM format) |
| `output.otlpGrpc.tls.key` | `--otlp-grpc-tls-key` | `BLITZ_OUTPUT_OTLPGRPC_TLS_KEY` | `""` | Path to the TLS private key file (PEM format) |
| `output.otlpGrpc.tls.ca` | `--otlp-grpc-tls-ca` | `BLITZ_OUTPUT_OTLPGRPC_TLS_CA` | `[]` | Paths to TLS CA certificate files (PEM format). Optional, if not provided the host's root CA set will be used |
| `output.otlpGrpc.tls.skipVerify` | `--otlp-grpc-tls-skip-verify` | `BLITZ_OUTPUT_OTLPGRPC_TLS_SKIP_VERIFY` | `false` | Whether to skip TLS certificate verification (not recommended for production) |
| `output.otlpGrpc.tls.minVersion` | `--otlp-grpc-tls-min-version` | `BLITZ_OUTPUT_OTLPGRPC_TLS_MIN_VERSION` | `1.2` | Minimum TLS version. Valid values: `1.2`, `1.3` |

## Example Configuration

### Basic OTLP gRPC Output

```yaml
output:
  type: otlp-grpc
  otlpGrpc:
    host: collector.example.com
    port: 4317
    workers: 3
    batchTimeout: 5s
    maxQueueSize: 2048
    maxExportBatchSize: 512
```

### OTLP gRPC Output with TLS

```yaml
output:
  type: otlp-grpc
  otlpGrpc:
    host: collector.example.com
    port: 4317
    workers: 3
    batchTimeout: 5s
    maxQueueSize: 2048
    maxExportBatchSize: 512
    enableTLS: true
    tls:
      insecure: false
      cert: /path/to/cert.pem
      key: /path/to/key.pem
      ca:
        - /path/to/ca.pem
      skipVerify: false
      minVersion: "1.2"
```

## Metrics

The OTLP gRPC output exposes the following metrics:

- **`blitz_otlp_grpc_logs_received_total`** (Counter): Number of logs received from the write channel
- **`blitz_otlp_grpc_workers_active`** (Gauge): Number of active worker goroutines
- **`blitz_otlp_grpc_log_rate_total`** (Counter, Float64): Rate at which logs are successfully sent to the configured host
- **`blitz_otlp_grpc_request_size_bytes`** (Histogram): Size of requests in bytes
- **`blitz_otlp_grpc_request_latency`** (Histogram): Request latency in seconds
- **`blitz_otlp_grpc_send_errors_total`** (Counter): Total number of send errors, labeled by `error_type`
- **`blitz_otlp_grpc_channel_size`** (Gauge): Current size of the data channel

All metrics include a `component` label set to `output_otlp_grpc`.

