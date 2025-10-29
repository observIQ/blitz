# Blitz Configuration

Blitz supports configuration through multiple methods with the following priority order (highest to lowest):

1. **Command-line flags** (highest priority)
2. **Environment variables**
3. **Configuration file** (when `--config` flag is provided)
4. **Default values** (lowest priority)

## Configuration Methods

### Command-line Flags

Use the `--config` flag to specify a configuration file:

```bash
./blitz --config /path/to/config.yaml
```

### Environment Variables

All configuration options can be set using environment variables with the `BLITZ_` prefix:

```bash
export BLITZ_LOGGING_LEVEL=debug
export BLITZ_OUTPUT_TYPE=tcp
./blitz
```

### Configuration File

Configuration files must be in YAML format and can be specified using the `--config` flag:

```bash
./blitz --config config.yaml
```

## Configuration Options

### Logging Configuration

| YAML Path | Flag Name | Environment Variable | Default | Description |
|-----------|-----------|---------------------|---------|-------------|
| `logging.type` | `--logging-type` | `BLITZ_LOGGING_TYPE` | `stdout` | Output destination for logs. Currently only `stdout` is supported. |
| `logging.level` | `--logging-level` | `BLITZ_LOGGING_LEVEL` | `info` | Log level. Valid values: `debug`, `info`, `warn`, `error` |

### Generator Configuration

**Note:** Only a single generator can be configured at a time.

| YAML Path | Flag Name | Environment Variable | Default | Description |
|-----------|-----------|---------------------|---------|-------------|
| `generator.type` | `--generator-type` | `BLITZ_GENERATOR_TYPE` | `nop` | Generator type. Valid values: `nop`, `json` |

#### NOP Generator Configuration

The NOP (No Operation) generator performs no work and generates no data. It's useful for testing the application infrastructure without generating actual log data.

**No additional configuration options are required for the NOP generator.**

#### JSON Generator Configuration

| YAML Path | Flag Name | Environment Variable | Default | Description |
|-----------|-----------|---------------------|---------|-------------|
| `generator.json.workers` | `--generator-json-workers` | `BLITZ_GENERATOR_JSON_WORKERS` | `1` | Number of JSON generator workers (must be ≥ 1) |
| `generator.json.rate` | `--generator-json-rate` | `BLITZ_GENERATOR_JSON_RATE` | `1s` | Rate at which logs are generated per worker (duration format) |

### Output Configuration

**Note:** Only a single output can be configured at a time.

| YAML Path | Flag Name | Environment Variable | Default | Description |
|-----------|-----------|---------------------|---------|-------------|
| `output.type` | `--output-type` | `BLITZ_OUTPUT_TYPE` | `nop` | Output type. Valid values: `nop`, `tcp`, `udp`, `otlp-grpc` |

#### NOP Output Configuration

The NOP (No Operation) output performs no work and discards all data. It's useful for testing the application infrastructure without actually sending data to external destinations.

**No additional configuration options are required for the NOP output.**

#### TCP Output Configuration

| YAML Path | Flag Name | Environment Variable | Default | Description |
|-----------|-----------|---------------------|---------|-------------|
| `output.tcp.host` | `--output-tcp-host` | `BLITZ_OUTPUT_TCP_HOST` | `""` | TCP target host (IP address or hostname) |
| `output.tcp.port` | `--output-tcp-port` | `BLITZ_OUTPUT_TCP_PORT` | `0` | TCP target port (1-65535) |
| `output.tcp.workers` | `--output-tcp-workers` | `BLITZ_OUTPUT_TCP_WORKERS` | `1` | Number of TCP output workers (must be ≥ 0) |

##### TCP TLS Configuration

TLS is disabled by default. To enable TLS, provide both a certificate and private key.

| YAML Path | Flag Name | Environment Variable | Default | Description |
|-----------|-----------|---------------------|---------|-------------|
| `output.tcp.tls.tlsCert` | `--output-tcp-tls-cert` | `BLITZ_OUTPUT_TCP_TLS_TLS_CERT` | `""` | Path to the TLS certificate file (PEM format) |
| `output.tcp.tls.tlsKey` | `--output-tcp-tls-key` | `BLITZ_OUTPUT_TCP_TLS_TLS_KEY` | `""` | Path to the TLS private key file (PEM format) |
| `output.tcp.tls.tlsCA` | `--output-tcp-tls-ca` | `BLITZ_OUTPUT_TCP_TLS_TLS_CA` | `[]` | Paths to TLS CA certificate files (PEM format). Optional, if not provided the host's root CA set will be used |
| `output.tcp.tls.tlsSkipVerify` | `--output-tcp-tls-skip-verify` | `BLITZ_OUTPUT_TCP_TLS_TLS_SKIP_VERIFY` | `false` | Whether to skip TLS certificate verification (not recommended for production) |
| `output.tcp.tls.tlsMinVersion` | `--output-tcp-tls-min-version` | `BLITZ_OUTPUT_TCP_TLS_TLS_MIN_VERSION` | `1.2` | Minimum TLS version. Valid values: `1.2`, `1.3` |

#### UDP Output Configuration

| YAML Path | Flag Name | Environment Variable | Default | Description |
|-----------|-----------|---------------------|---------|-------------|
| `output.udp.host` | `--output-udp-host` | `BLITZ_OUTPUT_UDP_HOST` | `""` | UDP target host (IP address or hostname) |
| `output.udp.port` | `--output-udp-port` | `BLITZ_OUTPUT_UDP_PORT` | `0` | UDP target port (1-65535) |
| `output.udp.workers` | `--output-udp-workers` | `BLITZ_OUTPUT_UDP_WORKERS` | `1` | Number of UDP output workers (must be ≥ 0) |

#### OTLP gRPC Output Configuration

The OTLP gRPC output sends logs to an OpenTelemetry collector via gRPC using the OTLP protocol.

| YAML Path | Flag Name | Environment Variable | Default | Description |
|-----------|-----------|---------------------|---------|-------------|
| `output.otlpGrpc.host` | `--output-otlpgrpc-host` | `BLITZ_OUTPUT_OTLPGRPC_HOST` | `localhost` | OTLP gRPC target host (IP address or hostname) |
| `output.otlpGrpc.port` | `--output-otlpgrpc-port` | `BLITZ_OUTPUT_OTLPGRPC_PORT` | `4317` | OTLP gRPC target port (1-65535) |
| `output.otlpGrpc.workers` | `--output-otlpgrpc-workers` | `BLITZ_OUTPUT_OTLPGRPC_WORKERS` | `1` | Number of OTLP gRPC output workers (must be ≥ 0) |
| `output.otlpGrpc.batchTimeout` | `--output-otlpgrpc-batchtimeout` | `BLITZ_OUTPUT_OTLPGRPC_BATCHTIMEOUT` | `1s` | Timeout for batching log records before sending (duration format) |
| `output.otlpGrpc.maxQueueSize` | `--output-otlpgrpc-maxqueuesize` | `BLITZ_OUTPUT_OTLPGRPC_MAXQUEUESIZE` | `100` | Maximum queue size for batching logs (must be ≥ 0) |
| `output.otlpGrpc.maxExportBatchSize` | `--output-otlpgrpc-maxexportbatchsize` | `BLITZ_OUTPUT_OTLPGRPC_MAXEXPORTBATCHSIZE` | `200` | Maximum number of logs per export batch (must be ≥ 0) |

##### OTLP gRPC TLS Configuration

By default, OTLP gRPC uses insecure credentials (no TLS). To enable TLS, set `insecure` to `false` and provide certificate and key files.

| YAML Path | Flag Name | Environment Variable | Default | Description |
|-----------|-----------|---------------------|---------|-------------|
| `output.otlpGrpc.tls.insecure` | `--otlp-grpc-tls-insecure` | `BLITZ_OUTPUT_OTLPGRPC_TLS_INSECURE` | `true` | Whether to use insecure credentials (no TLS). When `true`, TLS is not used. When `false` and TLS certificates are provided, TLS will be enabled |
| `output.otlpGrpc.tls.tlsCert` | `--otlp-grpc-tls-cert` | `BLITZ_OUTPUT_OTLPGRPC_TLS_TLS_CERT` | `""` | Path to the TLS certificate file (PEM format) |
| `output.otlpGrpc.tls.tlsKey` | `--otlp-grpc-tls-key` | `BLITZ_OUTPUT_OTLPGRPC_TLS_TLS_KEY` | `""` | Path to the TLS private key file (PEM format) |
| `output.otlpGrpc.tls.tlsCA` | `--otlp-grpc-tls-ca` | `BLITZ_OUTPUT_OTLPGRPC_TLS_TLS_CA` | `[]` | Paths to TLS CA certificate files (PEM format). Optional, if not provided the host's root CA set will be used |
| `output.otlpGrpc.tls.tlsSkipVerify` | `--otlp-grpc-tls-skip-verify` | `BLITZ_OUTPUT_OTLPGRPC_TLS_TLS_SKIP_VERIFY` | `false` | Whether to skip TLS certificate verification (not recommended for production) |
| `output.otlpGrpc.tls.tlsMinVersion` | `--otlp-grpc-tls-min-version` | `BLITZ_OUTPUT_OTLPGRPC_TLS_TLS_MIN_VERSION` | `1.2` | Minimum TLS version. Valid values: `1.2`, `1.3` |

## Example Configurations

### Basic TCP Output Configuration

```yaml
logging:
  type: stdout
  level: info

generator:
  type: json
  json:
    workers: 2
    rate: 500ms

output:
  type: tcp
  tcp:
    host: 127.0.0.1
    port: 9090
    workers: 3
```

### High-Performance UDP Configuration

```yaml
logging:
  type: stdout
  level: warn

generator:
  type: json
  json:
    workers: 10
    rate: 100ms

output:
  type: udp
  udp:
    host: logs.example.com
    port: 514
    workers: 5
```

### Debug Configuration

```yaml
logging:
  type: stdout
  level: debug

generator:
  type: json
  json:
    workers: 1
    rate: 1s

output:
  type: tcp
  tcp:
    host: localhost
    port: 8080
    workers: 1
```

### Minimal Configuration (NOP)

```yaml
# No configuration required - uses NOP generator and output by default
# This configuration performs no work and is useful for testing
```

### Minimal Configuration (JSON + TCP)

```yaml
generator:
  type: json

output:
  type: tcp
  tcp:
    host: 127.0.0.1
    port: 9090
```

### OTLP gRPC Output Configuration

```yaml
logging:
  type: stdout
  level: info

generator:
  type: json
  json:
    workers: 2
    rate: 500ms

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

## Duration Format

Duration values (like `generator.json.rate`) support the following formats:

- `500ms` - 500 milliseconds
- `1s` - 1 second
- `1m` - 1 minute
- `1h` - 1 hour
- `1h30m` - 1 hour 30 minutes

## Validation Rules

### Required Fields
- `generator.type` - Must be specified (defaults to `nop` if not provided)
- `output.type` - Must be specified (defaults to `nop` if not provided)
- `output.tcp.host` - Required when using TCP output
- `output.tcp.port` - Required when using TCP output
- `output.udp.host` - Required when using UDP output
- `output.udp.port` - Required when using UDP output
- `output.otlpGrpc.host` - Required when using OTLP gRPC output
- `output.otlpGrpc.port` - Required when using OTLP gRPC output

### Validation Constraints
- `generator.json.workers` - Must be ≥ 1
- `generator.json.rate` - Must be > 0
- `output.tcp.workers` - Must be ≥ 0
- `output.udp.workers` - Must be ≥ 0
- `output.otlpGrpc.workers` - Must be ≥ 0
- `output.tcp.port` - Must be between 1 and 65535
- `output.udp.port` - Must be between 1 and 65535
- `output.otlpGrpc.port` - Must be between 1 and 65535
- `output.otlpGrpc.maxQueueSize` - Must be ≥ 0
- `output.otlpGrpc.maxExportBatchSize` - Must be ≥ 0
- `output.otlpGrpc.batchTimeout` - Must be > 0 (duration format)
- `logging.level` - Must be one of: `debug`, `info`, `warn`, `error`
- `logging.type` - Must be `stdout` (only supported type)
- `generator.type` - Must be one of: `nop`, `json`
- `output.type` - Must be one of: `nop`, `tcp`, `udp`, `otlp-grpc`

## Error Handling

If a configuration file is specified with the `--config` flag but cannot be read, the application will:

1. Display an error message indicating the file path and specific error
2. Exit with code 1

Example error message:
```
Failed to read config file nonexistent.yaml: open nonexistent.yaml: no such file or directory
```

## Usage Examples

### Using Configuration File Only
```bash
./blitz --config production.yaml
```

### Overriding Configuration File with Flags
```bash
./blitz --config production.yaml --logging-level debug --generator-json-workers 5
```

### Using Environment Variables
```bash
export BLITZ_LOGGING_LEVEL=debug
export BLITZ_OUTPUT_TYPE=tcp
export BLITZ_OUTPUT_TCP_HOST=logs.example.com
export BLITZ_OUTPUT_TCP_PORT=9090
./blitz
```

### Using OTLP gRPC Output
```bash
./blitz --output-type otlp-grpc --output-otlpgrpc-host collector.example.com --output-otlpgrpc-port 4317
```

Or with environment variables:
```bash
export BLITZ_OUTPUT_TYPE=otlp-grpc
export BLITZ_OUTPUT_OTLPGRPC_HOST=collector.example.com
export BLITZ_OUTPUT_OTLPGRPC_PORT=4317
export BLITZ_OUTPUT_OTLPGRPC_BATCHTIMEOUT=10s
./blitz
```

### Mixed Configuration Methods
```bash
export BLITZ_OUTPUT_TYPE=tcp
./blitz --config base.yaml --logging-level warn --generator-json-workers 3
```

### Using TLS with TCP Output

```yaml
output:
  type: tcp
  tcp:
    host: logs.example.com
    port: 9090
    workers: 3
    tls:
      tlsCert: /path/to/cert.pem
      tlsKey: /path/to/key.pem
      tlsCA:
        - /path/to/ca.pem
      tlsSkipVerify: false
      tlsMinVersion: "1.2"
```

Or with command-line flags:
```bash
./blitz --output-type tcp \
  --output-tcp-host logs.example.com \
  --output-tcp-port 9090 \
  --output-tcp-tls-cert /path/to/cert.pem \
  --output-tcp-tls-key /path/to/key.pem \
  --output-tcp-tls-ca /path/to/ca.pem \
  --output-tcp-tls-min-version 1.2
```

### Using TLS with OTLP gRPC Output

```yaml
output:
  type: otlp-grpc
  otlpGrpc:
    host: collector.example.com
    port: 4317
    workers: 3
    tls:
      insecure: false
      tlsCert: /path/to/cert.pem
      tlsKey: /path/to/key.pem
      tlsCA:
        - /path/to/ca.pem
      tlsSkipVerify: false
      tlsMinVersion: "1.2"
```

Or with command-line flags:
```bash
./blitz --output-type otlp-grpc \
  --output-otlpgrpc-host collector.example.com \
  --output-otlpgrpc-port 4317 \
  --otlp-grpc-tls-insecure false \
  --otlp-grpc-tls-cert /path/to/cert.pem \
  --otlp-grpc-tls-key /path/to/key.pem \
  --otlp-grpc-tls-ca /path/to/ca.pem \
  --otlp-grpc-tls-min-version 1.2
```
