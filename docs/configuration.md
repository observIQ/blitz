# Bindplane Loader Configuration

Bindplane Loader supports configuration through multiple methods with the following priority order (highest to lowest):

1. **Command-line flags** (highest priority)
2. **Environment variables**
3. **Configuration file** (when `--config` flag is provided)
4. **Default values** (lowest priority)

## Configuration Methods

### Command-line Flags

Use the `--config` flag to specify a configuration file:

```bash
./bindplane-loader --config /path/to/config.yaml
```

### Environment Variables

All configuration options can be set using environment variables with the `BINDPLANE_` prefix:

```bash
export BINDPLANE_LOGGING_LEVEL=debug
export BINDPLANE_OUTPUT_TYPE=tcp
./bindplane-loader
```

### Configuration File

Configuration files must be in YAML format and can be specified using the `--config` flag:

```bash
./bindplane-loader --config config.yaml
```

## Configuration Options

### Logging Configuration

| YAML Path | Flag Name | Environment Variable | Default | Description |
|-----------|-----------|---------------------|---------|-------------|
| `logging.type` | `--logging-type` | `BINDPLANE_LOGGING_TYPE` | `stdout` | Output destination for logs. Currently only `stdout` is supported. |
| `logging.level` | `--logging-level` | `BINDPLANE_LOGGING_LEVEL` | `info` | Log level. Valid values: `debug`, `info`, `warn`, `error` |

### Generator Configuration

**Note:** Only a single generator can be configured at a time.

| YAML Path | Flag Name | Environment Variable | Default | Description |
|-----------|-----------|---------------------|---------|-------------|
| `generator.type` | `--generator-type` | `BINDPLANE_GENERATOR_TYPE` | `nop` | Generator type. Valid values: `nop`, `json` |

#### NOP Generator Configuration

The NOP (No Operation) generator performs no work and generates no data. It's useful for testing the application infrastructure without generating actual log data.

**No additional configuration options are required for the NOP generator.**

#### JSON Generator Configuration

| YAML Path | Flag Name | Environment Variable | Default | Description |
|-----------|-----------|---------------------|---------|-------------|
| `generator.json.workers` | `--generator-json-workers` | `BINDPLANE_GENERATOR_JSON_WORKERS` | `1` | Number of JSON generator workers (must be ≥ 1) |
| `generator.json.rate` | `--generator-json-rate` | `BINDPLANE_GENERATOR_JSON_RATE` | `1s` | Rate at which logs are generated per worker (duration format) |

### Output Configuration

**Note:** Only a single output can be configured at a time.

| YAML Path | Flag Name | Environment Variable | Default | Description |
|-----------|-----------|---------------------|---------|-------------|
| `output.type` | `--output-type` | `BINDPLANE_OUTPUT_TYPE` | `nop` | Output type. Valid values: `nop`, `tcp`, `udp` |

#### NOP Output Configuration

The NOP (No Operation) output performs no work and discards all data. It's useful for testing the application infrastructure without actually sending data to external destinations.

**No additional configuration options are required for the NOP output.**

#### TCP Output Configuration

| YAML Path | Flag Name | Environment Variable | Default | Description |
|-----------|-----------|---------------------|---------|-------------|
| `output.tcp.host` | `--output-tcp-host` | `BINDPLANE_OUTPUT_TCP_HOST` | `""` | TCP target host (IP address or hostname) |
| `output.tcp.port` | `--output-tcp-port` | `BINDPLANE_OUTPUT_TCP_PORT` | `0` | TCP target port (1-65535) |
| `output.tcp.workers` | `--output-tcp-workers` | `BINDPLANE_OUTPUT_TCP_WORKERS` | `1` | Number of TCP output workers (must be ≥ 0) |

#### UDP Output Configuration

| YAML Path | Flag Name | Environment Variable | Default | Description |
|-----------|-----------|---------------------|---------|-------------|
| `output.udp.host` | `--output-udp-host` | `BINDPLANE_OUTPUT_UDP_HOST` | `""` | UDP target host (IP address or hostname) |
| `output.udp.port` | `--output-udp-port` | `BINDPLANE_OUTPUT_UDP_PORT` | `0` | UDP target port (1-65535) |
| `output.udp.workers` | `--output-udp-workers` | `BINDPLANE_OUTPUT_UDP_WORKERS` | `1` | Number of UDP output workers (must be ≥ 0) |

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

### Validation Constraints
- `generator.json.workers` - Must be ≥ 1
- `generator.json.rate` - Must be > 0
- `output.tcp.workers` - Must be ≥ 0
- `output.udp.workers` - Must be ≥ 0
- `output.tcp.port` - Must be between 1 and 65535
- `output.udp.port` - Must be between 1 and 65535
- `logging.level` - Must be one of: `debug`, `info`, `warn`, `error`
- `logging.type` - Must be `stdout` (only supported type)
- `generator.type` - Must be one of: `nop`, `json`
- `output.type` - Must be one of: `nop`, `tcp`, `udp`

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
./bindplane-loader --config production.yaml
```

### Overriding Configuration File with Flags
```bash
./bindplane-loader --config production.yaml --logging-level debug --generator-json-workers 5
```

### Using Environment Variables
```bash
export BINDPLANE_LOGGING_LEVEL=debug
export BINDPLANE_OUTPUT_TYPE=tcp
export BINDPLANE_OUTPUT_TCP_HOST=logs.example.com
export BINDPLANE_OUTPUT_TCP_PORT=9090
./bindplane-loader
```

### Mixed Configuration Methods
```bash
export BINDPLANE_OUTPUT_TYPE=tcp
./bindplane-loader --config base.yaml --logging-level warn --generator-json-workers 3
```
