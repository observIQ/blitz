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

### Linux packages and systemd

When installed via the Linux packages (`.deb`/`.rpm`), Blitz provides:

- Configuration file at `/etc/blitz/config.yaml`
- A systemd service named `blitz`

The packaged service runs:

```
/usr/bin/blitz --config /etc/blitz/config.yaml
```

To set or override configuration via environment variables using systemd overrides:

1) Create a drop-in override:

```bash
sudo systemctl edit blitz
```

2) Add environment variables under the `[Service]` section (see “Environment Variables” naming above):

```
[Service]
Environment=BLITZ_OUTPUT_TYPE=tcp
Environment=BLITZ_OUTPUT_TCP_HOST=127.0.0.1
Environment=BLITZ_OUTPUT_TCP_PORT=5000
```

3) Reload and restart:

```bash
sudo systemctl daemon-reload
sudo systemctl restart blitz
```

Optionally, you can store many variables in `/etc/blitz/blitz.env` (loaded by the packaged systemd unit by default):

```
BLITZ_OUTPUT_TYPE=tcp
BLITZ_OUTPUT_TCP_HOST=127.0.0.1
BLITZ_OUTPUT_TCP_PORT=5000
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
| `generator.type` | `--generator-type` | `BLITZ_GENERATOR_TYPE` | `nop` | Generator type. Valid values: `nop`, `json`, `winevt`, `palo-alto` |

#### NOP Generator Configuration

The NOP (No Operation) generator performs no work and generates no data. It's useful for testing the application infrastructure without generating actual log data.

**No additional configuration options are required for the NOP generator.**

#### JSON Generator Configuration

| YAML Path | Flag Name | Environment Variable | Default | Description |
|-----------|-----------|---------------------|---------|-------------|
| `generator.json.workers` | `--generator-json-workers` | `BLITZ_GENERATOR_JSON_WORKERS` | `1` | Number of JSON generator workers (must be ≥ 1) |
| `generator.json.rate` | `--generator-json-rate` | `BLITZ_GENERATOR_JSON_RATE` | `1s` | Rate at which logs are generated per worker (duration format) |

#### Windows Event (winevt) Generator Configuration

| YAML Path | Flag Name | Environment Variable | Default | Description |
|-----------|-----------|---------------------|---------|-------------|
| `generator.winevt.workers` | `--generator-winevt-workers` | `BLITZ_GENERATOR_WINEVT_WORKERS` | `1` | Number of winevt generator workers (must be ≥ 1) |
| `generator.winevt.rate` | `--generator-winevt-rate` | `BLITZ_GENERATOR_WINEVT_RATE` | `1s` | Rate at which logs are generated per worker (duration format) |

#### Palo Alto Generator Configuration

The Palo Alto generator generates realistic Palo Alto firewall syslog entries in the standard Palo Alto log format.

| YAML Path | Flag Name | Environment Variable | Default | Description |
|-----------|-----------|---------------------|---------|-------------|
| `generator.paloAlto.workers` | `--generator-paloalto-workers` | `BLITZ_GENERATOR_PALOALTO_WORKERS` | `1` | Number of Palo Alto generator workers (must be ≥ 1) |
| `generator.paloAlto.rate` | `--generator-paloalto-rate` | `BLITZ_GENERATOR_PALOALTO_RATE` | `1s` | Rate at which logs are generated per worker (duration format) |

### Output Configuration

**Note:** Only a single output can be configured at a time.

| YAML Path | Flag Name | Environment Variable | Default | Description |
|-----------|-----------|---------------------|---------|-------------|
| `output.type` | `--output-type` | `BLITZ_OUTPUT_TYPE` | `nop` | Output type. Valid values: `nop`, `stdout`, `tcp`, `udp`, `syslog`, `otlp-grpc`, `file` |

#### NOP Output Configuration

The NOP (No Operation) output performs no work and discards all data. It's useful for testing the application infrastructure without actually sending data to external destinations.

**No additional configuration options are required for the NOP output.**

#### Stdout Output Configuration

The stdout output writes all generated logs to standard output (stdout). This is useful for debugging and testing.

**Note:** The stdout output may not be suitable for piping to another process, as stdout is shared with the main blitz logger. Both application logs and generated log data will be written to stdout, which can make it difficult to separate them when piping.

**No additional configuration options are required for the stdout output.**

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
| `output.tcp.enableTLS` | `--output-tcp-enable-tls` | `BLITZ_OUTPUT_TCP_ENABLE_TLS` | `false` | Enable TLS for TCP connections |
| `output.tcp.tls.cert` | `--output-tcp-tls-cert` | `BLITZ_OUTPUT_TCP_TLS_CERT` | `""` | Path to the TLS certificate file (PEM format) |
| `output.tcp.tls.key` | `--output-tcp-tls-key` | `BLITZ_OUTPUT_TCP_TLS_KEY` | `""` | Path to the TLS private key file (PEM format) |
| `output.tcp.tls.ca` | `--output-tcp-tls-ca` | `BLITZ_OUTPUT_TCP_TLS_CA` | `[]` | Paths to TLS CA certificate files (PEM format). Optional, if not provided the host's root CA set will be used |
| `output.tcp.tls.skipVerify` | `--output-tcp-tls-skip-verify` | `BLITZ_OUTPUT_TCP_TLS_SKIP_VERIFY` | `false` | Whether to skip TLS certificate verification (not recommended for production) |
| `output.tcp.tls.minVersion` | `--output-tcp-tls-min-version` | `BLITZ_OUTPUT_TCP_TLS_MIN_VERSION` | `1.2` | Minimum TLS version. Valid values: `1.2`, `1.3` |

#### UDP Output Configuration

| YAML Path | Flag Name | Environment Variable | Default | Description |
|-----------|-----------|---------------------|---------|-------------|
| `output.udp.host` | `--output-udp-host` | `BLITZ_OUTPUT_UDP_HOST` | `""` | UDP target host (IP address or hostname) |
| `output.udp.port` | `--output-udp-port` | `BLITZ_OUTPUT_UDP_PORT` | `0` | UDP target port (1-65535) |
| `output.udp.workers` | `--output-udp-workers` | `BLITZ_OUTPUT_UDP_WORKERS` | `1` | Number of UDP output workers (must be ≥ 0) |

#### Syslog Output Configuration

The Syslog output formats messages per RFC 3164 or RFC 5424 and sends them via UDP or TCP (called "transport"). When using TCP transport, TLS can be enabled.

| YAML Path | Flag Name | Environment Variable | Default | Description |
|-----------|-----------|---------------------|---------|-------------|
| `output.syslog.host` | `--output-syslog-host` | `BLITZ_OUTPUT_SYSLOG_HOST` | `""` | Syslog target host (IP address or hostname) |
| `output.syslog.port` | `--output-syslog-port` | `BLITZ_OUTPUT_SYSLOG_PORT` | `0` | Syslog target port (1-65535) |
| `output.syslog.transport` | `--output-syslog-transport` | `BLITZ_OUTPUT_SYSLOG_TRANSPORT` | `udp` | Transport: `udp` or `tcp` |
| `output.syslog.rfc` | `--output-syslog-rfc` | `BLITZ_OUTPUT_SYSLOG_RFC` | `5424` | Syslog format: `3164` or `5424` |
| `output.syslog.workers` | `--output-syslog-workers` | `BLITZ_OUTPUT_SYSLOG_WORKERS` | `1` | Number of Syslog output workers (must be ≥ 0) |
| `output.syslog.facility` | `--output-syslog-facility` | `BLITZ_OUTPUT_SYSLOG_FACILITY` | `1` | Syslog facility (0–23) |
| `output.syslog.appName` | `--output-syslog-appname` | `BLITZ_OUTPUT_SYSLOG_APPNAME` | `blitz` | App name used in syslog header |
| `output.syslog.hostname` | `--output-syslog-hostname` | `BLITZ_OUTPUT_SYSLOG_HOSTNAME` | `""` | Hostname used in syslog header |
| `output.syslog.procId` | `--output-syslog-procid` | `BLITZ_OUTPUT_SYSLOG_PROCID` | `""` | Process ID used in syslog header |
| `output.syslog.msgId` | `--output-syslog-msgid` | `BLITZ_OUTPUT_SYSLOG_MSGID` | `""` | Message ID used in syslog header |
| `output.syslog.maxDatagramBytes` | `--output-syslog-maxdatagrambytes` | `BLITZ_OUTPUT_SYSLOG_MAXDATAGRAMBYTES` | `0` | UDP safety limit in bytes; if > 0, messages are truncated to fit |

##### Syslog TLS Configuration (TCP transport only)

TLS is disabled by default. To enable TLS for Syslog over TCP, set `enableTLS: true` and provide certificate and key.

| YAML Path | Flag Name | Environment Variable | Default | Description |
|-----------|-----------|---------------------|---------|-------------|
| `output.syslog.enableTLS` | `--output-syslog-enable-tls` | `BLITZ_OUTPUT_SYSLOG_ENABLE_TLS` | `false` | Enable TLS for Syslog over TCP |
| `output.syslog.tls.cert` | `--output-syslog-tls-cert` | `BLITZ_OUTPUT_SYSLOG_TLS_CERT` | `""` | Path to the TLS certificate file (PEM format) |
| `output.syslog.tls.key` | `--output-syslog-tls-key` | `BLITZ_OUTPUT_SYSLOG_TLS_KEY` | `""` | Path to the TLS private key file (PEM format) |
| `output.syslog.tls.ca` | `--output-syslog-tls-ca` | `BLITZ_OUTPUT_SYSLOG_TLS_CA` | `[]` | Paths to TLS CA certificate files (PEM format). Optional |
| `output.syslog.tls.skipVerify` | `--output-syslog-tls-skip-verify` | `BLITZ_OUTPUT_SYSLOG_TLS_SKIP_VERIFY` | `false` | Whether to skip TLS certificate verification (not recommended) |
| `output.syslog.tls.minVersion` | `--output-syslog-tls-min-version` | `BLITZ_OUTPUT_SYSLOG_TLS_MIN_VERSION` | `1.2` | Minimum TLS version. Valid values: `1.2`, `1.3` |

###### Framing and limitations

- TCP transport currently uses newline-delimited (non-transparent) framing; octet-counting per RFC 6587 is not supported. Many syslog servers accept newline-delimited framing, but if your receiver requires octet-counting, this output will not work as-is.
- UDP transport sends each formatted message as a single datagram. If `maxDatagramBytes` is set and the message would exceed it, the message is truncated to fit.
- To avoid breaking framing, embedded CR/LF in messages are replaced with spaces during formatting.

#### File Output Configuration

| YAML Path | Flag Name | Environment Variable | Default | Description |
|-----------|-----------|---------------------|---------|-------------|
| `output.file.path` | `--output-file-path` | `BLITZ_OUTPUT_FILE_PATH` | `""` | Destination file path (required when using file output) |
| `output.file.workers` | `--output-file-workers` | `BLITZ_OUTPUT_FILE_WORKERS` | `1` | Number of File output workers (must be ≥ 0) |
| `output.file.rotation.maxSizeMB` | `--output-file-rotation-maxsizemb` | `BLITZ_OUTPUT_FILE_ROTATION_MAXSIZEMB` | `100` | Maximum size in MB before rotation |
| `output.file.rotation.maxBackups` | `--output-file-rotation-maxbackups` | `BLITZ_OUTPUT_FILE_ROTATION_MAXBACKUPS` | `7` | Maximum number of backups to retain |
| `output.file.rotation.maxAgeDays` | `--output-file-rotation-maxagedays` | `BLITZ_OUTPUT_FILE_ROTATION_MAXAGEDAYS` | `30` | Maximum age in days to retain backups |
| `output.file.rotation.compress` | `--output-file-rotation-compress` | `BLITZ_OUTPUT_FILE_ROTATION_COMPRESS` | `true` | Compress rotated files |
| `output.file.rotation.localTime` | `--output-file-rotation-localtime` | `BLITZ_OUTPUT_FILE_ROTATION_LOCALTIME` | `false` | Use local time for backup timestamps |

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
| `output.otlpGrpc.enableTLS` | `--output-otlpgrpc-enable-tls` | `BLITZ_OUTPUT_OTLPGRPC_ENABLE_TLS` | `false` | Enable TLS for OTLP gRPC connections |
| `output.otlpGrpc.tls.insecure` | `--otlp-grpc-tls-insecure` | `BLITZ_OUTPUT_OTLPGRPC_TLS_INSECURE` | `true` | Whether to use insecure credentials (no TLS). When `true`, TLS is not used. When `false` and TLS certificates are provided, TLS will be enabled |
| `output.otlpGrpc.tls.cert` | `--otlp-grpc-tls-cert` | `BLITZ_OUTPUT_OTLPGRPC_TLS_CERT` | `""` | Path to the TLS certificate file (PEM format) |
| `output.otlpGrpc.tls.key` | `--otlp-grpc-tls-key` | `BLITZ_OUTPUT_OTLPGRPC_TLS_KEY` | `""` | Path to the TLS private key file (PEM format) |
| `output.otlpGrpc.tls.ca` | `--otlp-grpc-tls-ca` | `BLITZ_OUTPUT_OTLPGRPC_TLS_CA` | `[]` | Paths to TLS CA certificate files (PEM format). Optional, if not provided the host's root CA set will be used |
| `output.otlpGrpc.tls.skipVerify` | `--otlp-grpc-tls-skip-verify` | `BLITZ_OUTPUT_OTLPGRPC_TLS_SKIP_VERIFY` | `false` | Whether to skip TLS certificate verification (not recommended for production) |
| `output.otlpGrpc.tls.minVersion` | `--otlp-grpc-tls-min-version` | `BLITZ_OUTPUT_OTLPGRPC_TLS_MIN_VERSION` | `1.2` | Minimum TLS version. Valid values: `1.2`, `1.3` |

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

### Syslog over UDP

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
  type: syslog
  syslog:
    host: logs.example.com
    port: 514
    transport: udp
    rfc: "5424"
    workers: 2
    facility: 1
    appName: blitz
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

### Stdout Output Configuration

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
  type: stdout
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

### Palo Alto Generator Configuration

```yaml
logging:
  type: stdout
  level: info

generator:
  type: palo-alto
  paloAlto:
    workers: 2
    rate: 500ms

output:
  type: tcp
  tcp:
    host: 127.0.0.1
    port: 514
    workers: 3
```

## Duration Format

Duration values (like `generator.json.rate`, `generator.winevt.rate`, or `generator.paloAlto.rate`) support the following formats:

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
- `output.syslog.host` - Required when using Syslog output
- `output.syslog.port` - Required when using Syslog output
- `output.otlpGrpc.host` - Required when using OTLP gRPC output
- `output.otlpGrpc.port` - Required when using OTLP gRPC output
- `output.file.path` - Required when using File output

### Validation Constraints
- `generator.json.workers` - Must be ≥ 1
- `generator.json.rate` - Must be > 0
- `generator.winevt.workers` - Must be ≥ 1
- `generator.winevt.rate` - Must be > 0
- `generator.paloAlto.workers` - Must be ≥ 1
- `generator.paloAlto.rate` - Must be > 0
- `output.tcp.workers` - Must be ≥ 0
- `output.udp.workers` - Must be ≥ 0
- `output.syslog.workers` - Must be ≥ 0
- `output.otlpGrpc.workers` - Must be ≥ 0
- `output.tcp.port` - Must be between 1 and 65535
- `output.udp.port` - Must be between 1 and 65535
- `output.syslog.port` - Must be between 1 and 65535
- `output.otlpGrpc.port` - Must be between 1 and 65535
- `output.otlpGrpc.maxQueueSize` - Must be ≥ 0
- `output.otlpGrpc.maxExportBatchSize` - Must be ≥ 0
- `output.otlpGrpc.batchTimeout` - Must be > 0 (duration format)
- `logging.level` - Must be one of: `debug`, `info`, `warn`, `error`
- `logging.type` - Must be `stdout` (only supported type)
- `generator.type` - Must be one of: `nop`, `json`, `winevt`, `palo-alto`
- `output.type` - Must be one of: `nop`, `stdout`, `tcp`, `udp`, `syslog`, `otlp-grpc`, `file`

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

### Using Palo Alto Generator
```bash
./blitz --generator-type palo-alto --generator-paloalto-workers 3 --generator-paloalto-rate 250ms
```

Or with environment variables:
```bash
export BLITZ_GENERATOR_TYPE=palo-alto
export BLITZ_GENERATOR_PALOALTO_WORKERS=3
export BLITZ_GENERATOR_PALOALTO_RATE=250ms
./blitz
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
      cert: /path/to/cert.pem
      key: /path/to/key.pem
      ca:
        - /path/to/ca.pem
      skipVerify: false
      minVersion: "1.2"
```

### Syslog over TCP with TLS

```yaml
output:
  type: syslog
  syslog:
    host: logs.example.com
    port: 6514
    transport: tcp
    rfc: "5424"
    enableTLS: true
    tls:
      cert: /path/to/cert.pem
      key: /path/to/key.pem
      ca:
        - /path/to/ca.pem
      skipVerify: false
      minVersion: "1.2"
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
      cert: /path/to/cert.pem
      key: /path/to/key.pem
      ca:
        - /path/to/ca.pem
      skipVerify: false
      minVersion: "1.2"
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
