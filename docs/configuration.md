# Blitz Configuration

## Table of Contents

- [Configuration Methods](#configuration-methods)
  - [Command-line Flags](#command-line-flags)
  - [Environment Variables](#environment-variables)
  - [Configuration File](#configuration-file)
  - [Linux packages and systemd](#linux-packages-and-systemd)
- [Configuration Options](#configuration-options)
  - [Logging Configuration](#logging-configuration)
    - [Default Logging Behavior](#default-logging-behavior)
    - [File Logging Configuration](#file-logging-configuration)
  - [Generator Configuration](#generator-configuration)
  - [Output Configuration](#output-configuration)
- [Example Configurations](#example-configurations)
  - [Basic TCP Output Configuration](#basic-tcp-output-configuration)
  - [High-Performance UDP Configuration](#high-performance-udp-configuration)
  - [Syslog over UDP](#syslog-over-udp)
  - [Debug Configuration](#debug-configuration)
  - [Minimal Configuration (NOP)](#minimal-configuration-nop)
  - [Stdout Output Configuration](#stdout-output-configuration)
  - [Minimal Configuration (JSON + TCP)](#minimal-configuration-json--tcp)
  - [OTLP gRPC Output Configuration](#otlp-grpc-output-configuration)
  - [Palo Alto Generator Configuration](#palo-alto-generator-configuration)
- [Duration Format](#duration-format)
- [Validation Rules](#validation-rules)
  - [Required Fields](#required-fields)
  - [Validation Constraints](#validation-constraints)
- [Error Handling](#error-handling)
- [Usage Examples](#usage-examples)
  - [Using Configuration File Only](#using-configuration-file-only)
  - [Overriding Configuration File with Flags](#overriding-configuration-file-with-flags)
  - [Using Palo Alto Generator](#using-palo-alto-generator)
  - [Using Environment Variables](#using-environment-variables)
  - [Using OTLP gRPC Output](#using-otlp-grpc-output)
  - [Mixed Configuration Methods](#mixed-configuration-methods)
  - [Using TLS with TCP Output](#using-tls-with-tcp-output)
  - [Syslog over TCP with TLS](#syslog-over-tcp-with-tls)
  - [Using TLS with OTLP gRPC Output](#using-tls-with-otlp-grpc-output)

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

- Configuration file at `/etc/blitz/config.yaml` (configured for file logging by default)
- A systemd service named `blitz`
- Log directory at `/var/log/blitz` (created by the package)

The packaged service runs:

```
/usr/bin/blitz --config /etc/blitz/config.yaml
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
| `logging.type` | `--logging-type` | `BLITZ_LOGGING_TYPE` | `stdout` | Output destination for logs. Valid values: `stdout`, `file` |
| `logging.level` | `--logging-level` | `BLITZ_LOGGING_LEVEL` | `info` | Log level. Valid values: `debug`, `info`, `warn`, `error` |

#### Default Logging Behavior

The default logging behavior depends on how Blitz is deployed:

- **CLI (command-line)**: Logs to `stdout` by default. This is suitable for interactive use and when running Blitz directly.
- **Linux packages** (`.deb`/`.rpm`): Logs to a file at `/var/log/blitz/blitz.log` with automatic rotation. The package includes a configuration file at `/etc/blitz/config.yaml` that sets file logging.
- **Containers** (Docker): Logs to `stdout` by default. The container image sets `BLITZ_LOGGING_TYPE=stdout` as an environment variable.

You can override the default behavior by setting `logging.type` in your configuration file, via command-line flags, or environment variables.

#### File Logging Configuration

When `logging.type` is set to `file`, logs are written to a file with automatic rotation.

| YAML Path | Flag Name | Environment Variable | Default | Description |
|-----------|-----------|---------------------|---------|-------------|
| `logging.file.path` | `--logging-file-path` | `BLITZ_LOGGING_FILE_PATH` | `/var/log/blitz/blitz.log` | File path for log output |
| `logging.file.rotation.maxSizeMB` | `--logging-file-rotation-maxsizemb` | `BLITZ_LOGGING_FILE_ROTATION_MAXSIZEMB` | `100` | Maximum size in MB before rotation |
| `logging.file.rotation.maxBackups` | `--logging-file-rotation-maxbackups` | `BLITZ_LOGGING_FILE_ROTATION_MAXBACKUPS` | `7` | Maximum number of backups to retain |
| `logging.file.rotation.maxAgeDays` | `--logging-file-rotation-maxagedays` | `BLITZ_LOGGING_FILE_ROTATION_MAXAGEDAYS` | `30` | Maximum age in days to retain backups |
| `logging.file.rotation.compress` | `--logging-file-rotation-compress` | `BLITZ_LOGGING_FILE_ROTATION_COMPRESS` | `true` | Compress rotated files |
| `logging.file.rotation.localTime` | `--logging-file-rotation-localtime` | `BLITZ_LOGGING_FILE_ROTATION_LOCALTIME` | `false` | Use local time for backup timestamps |

**Example File Logging Configuration:**

```yaml
logging:
  type: file
  level: info
  file:
    path: /var/log/blitz/blitz.log
    rotation:
      maxSizeMB: 100
      maxBackups: 7
      maxAgeDays: 30
      compress: true
      localTime: false
```

**Example Stdout Logging Configuration:**

```yaml
logging:
  type: stdout
  level: info
```

### Generator Configuration

**Note:** Only a single generator can be configured at a time.

| YAML Path | Flag Name | Environment Variable | Default | Description |
|-----------|-----------|---------------------|---------|-------------|
| `generator.type` | `--generator-type` | `BLITZ_GENERATOR_TYPE` | `nop` | Generator type. Valid values: `nop`, `json`, `winevt`, `palo-alto`, `apache-common`, `apache-combined`, `apache-error`, `nginx` |

For detailed configuration options for each generator, see the individual generator documentation:

- [NOP Generator](https://github.com/observiq/blitz/blob/main/docs/generator/nop.md)
- [JSON Generator](https://github.com/observiq/blitz/blob/main/docs/generator/json.md)
- [Windows Event (winevt) Generator](https://github.com/observiq/blitz/blob/main/docs/generator/winevt.md)
- [Palo Alto Generator](https://github.com/observiq/blitz/blob/main/docs/generator/palo-alto.md)
- [Apache Common Log Format Generator](https://github.com/observiq/blitz/blob/main/docs/generator/apache-common.md)
- [Apache Combined Log Format Generator](https://github.com/observiq/blitz/blob/main/docs/generator/apache-combined.md)
- [Apache Error Log Format Generator](https://github.com/observiq/blitz/blob/main/docs/generator/apache-error.md)
- [NGINX Combined Log Format Generator](https://github.com/observiq/blitz/blob/main/docs/generator/nginx.md)

### Output Configuration

**Note:** Only a single output can be configured at a time.

| YAML Path | Flag Name | Environment Variable | Default | Description |
|-----------|-----------|---------------------|---------|-------------|
| `output.type` | `--output-type` | `BLITZ_OUTPUT_TYPE` | `nop` | Output type. Valid values: `nop`, `stdout`, `tcp`, `udp`, `syslog`, `otlp-grpc`, `file` |

For detailed configuration options for each output, see the individual output documentation:

- [NOP Output](https://github.com/observiq/blitz/blob/main/docs/output/nop.md)
- [Stdout Output](https://github.com/observiq/blitz/blob/main/docs/output/stdout.md)
- [TCP Output](https://github.com/observiq/blitz/blob/main/docs/output/tcp.md)
- [UDP Output](https://github.com/observiq/blitz/blob/main/docs/output/udp.md)
- [Syslog Output](https://github.com/observiq/blitz/blob/main/docs/output/syslog.md)
- [OTLP gRPC Output](https://github.com/observiq/blitz/blob/main/docs/output/otlp-grpc.md)
- [File Output](https://github.com/observiq/blitz/blob/main/docs/output/file.md)

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
- `logging.type` - Must be one of: `stdout`, `file`
- `logging.file.path` - Required when `logging.type` is `file`
- `generator.type` - Must be one of: `nop`, `json`, `winevt`, `palo-alto`, `apache-common`, `apache-combined`, `apache-error`, `nginx`
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
