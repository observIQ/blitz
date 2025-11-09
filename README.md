<div align="center">
  <img src="docs/assets/blitz-logo-banner-neon-1.png" alt="Blitz">
</div>

# Blitz

A load generation tool for Bindplane managed collectors.

## Documentation

- [Configuration Guide](/docs/configuration.md) - Complete guide to configuring blitz with YAML files, environment variables, and command-line flags
- [Architecture Overview](/docs/architecture.md) - Detailed explanation of the application architecture, components, and data flow
- [Metrics Documentation](/docs/metrics.md) - Comprehensive guide to monitoring and metrics exposed by blitz
- [Shell Completion](/docs/shell-completion.md) - Guide to installing and using shell autocompletion for bash, zsh, fish, and PowerShell
- [Development Guide](/docs/development.md) - Guidelines for contributing to the project
- [Contributing Guidelines](/docs/CONTRIBUTING.md) - How to contribute to the project

## Components

Blitz consists of generators that create log data and outputs that send data to destinations.

### Generators

- **[nop](https://github.com/observiq/blitz/blob/main/docs/generator/nop.md)** - No operation generator for testing infrastructure without generating data
- **[json](https://github.com/observiq/blitz/blob/main/docs/generator/json.md)** - Generates structured JSON logs
- **[winevt](https://github.com/observiq/blitz/blob/main/docs/generator/winevt.md)** - Generates Windows Event logs in unparsed XML format
- **[palo-alto](https://github.com/observiq/blitz/blob/main/docs/generator/palo-alto.md)** - Generates realistic Palo Alto firewall syslog entries
- **[apache-common](https://github.com/observiq/blitz/blob/main/docs/generator/apache-common.md)** - Generates Apache Common Log Format (CLF) entries
- **[apache-combined](https://github.com/observiq/blitz/blob/main/docs/generator/apache-combined.md)** - Generates Apache Combined Log Format entries with referer and user-agent
- **[apache-error](https://github.com/observiq/blitz/blob/main/docs/generator/apache-error.md)** - Generates Apache Error Log Format entries for server diagnostics
- **[nginx](https://github.com/observiq/blitz/blob/main/docs/generator/nginx.md)** - Generates NGINX Combined Log Format entries matching NGINX's default log format
- **[postgres](https://github.com/observiq/blitz/blob/main/docs/generator/postgres.md)** - Generates PostgreSQL log format entries including query logs, connections, and database events

### Outputs

- **[nop](https://github.com/observiq/blitz/blob/main/docs/output/nop.md)** - No operation output for testing infrastructure without sending data
- **[stdout](https://github.com/observiq/blitz/blob/main/docs/output/stdout.md)** - Writes logs to standard output for debugging and testing
- **[tcp](https://github.com/observiq/blitz/blob/main/docs/output/tcp.md)** - Sends logs over TCP connections with optional TLS encryption
- **[udp](https://github.com/observiq/blitz/blob/main/docs/output/udp.md)** - Sends logs over UDP connections
- **[syslog](https://github.com/observiq/blitz/blob/main/docs/output/syslog.md)** - Formats and sends logs via syslog (RFC 3164 or 5424) over UDP or TCP
- **[otlp-grpc](https://github.com/observiq/blitz/blob/main/docs/output/otlp-grpc.md)** - Sends logs via OpenTelemetry Protocol (OTLP) over gRPC
- **[file](https://github.com/observiq/blitz/blob/main/docs/output/file.md)** - Writes logs to files with automatic rotation and compression

## Installation

Blitz supports the following platforms:
- **Operating Systems**: Linux, macOS (Darwin), Windows, FreeBSD
- **CPU Architectures**: amd64 (x86_64), arm64

If your platform is not listed above, please [open an issue](https://github.com/observiq/blitz/issues)
and we'll do our best to add support for it.

### CLI

Download the binary for your platform from the [latest release](https://github.com/observiq/blitz/releases/latest):

Extract the archive and run the binary directly in a terminal:

```bash
tar -xzf blitz_*_linux_amd64.tar.gz
```

Run with default NOP configuration:

```bash
./blitz
```

Run with JSON generator and TCP output:

```bash
./blitz \
  --generator-type json \
  --generator-json-workers 2 \
  --generator-json-rate 500ms \
  --output-type tcp \
  --output-tcp-host logs.example.com \
  --output-tcp-port 9090 \
  --output-tcp-workers 3 \
  --logging-level info
```

### Linux Systemd Service

Download the appropriate package for your Linux distribution from the [latest release](https://github.com/observiq/blitz/releases/latest):

- **Debian/Ubuntu**: `blitz_amd64.deb` or `blitz_arm64.deb`
- **Red Hat/CentOS/Fedora**: `blitz_amd64.rpm` or `blitz_arm64.rpm`

Install the package with your package manager:

**Debian**

```bash
sudo apt-get install -f ./blitz_amd64.deb
```

**RHEL**

```bash
sudo dnf install ./blitz_amd64.rpm
```

Edit the configuration file:

```bash
sudo vi /etc/blitz/config.yaml
```

Example minimal configuration for JSON generator and TCP output:

```yaml
generator:
  type: json
  json:
    workers: 2
    rate: 500ms
output:
  type: tcp
  tcp:
    host: logs.example.com
    port: 9090
    workers: 3
logging:
  level: info
```

Enable and start the service

```bash
sudo systemctl enable blitz
sudo systemctl start blitz
sudo systemctl status blitz
```

View service logs:

```bash
sudo journalctl -u blitz -f
```

### Container

Pull the Docker image from GitHub Container Registry and run it with environment variables for configuration:

Run with default NOP configuration:

```bash
docker run --rm ghcr.io/observiq/blitz:latest
```

Run with JSON generator and TCP output:

```bash
docker run --rm \
  -e BINDPLANE_GENERATOR_TYPE=json \
  -e BINDPLANE_GENERATOR_JSON_WORKERS=2 \
  -e BINDPLANE_GENERATOR_JSON_RATE=500ms \
  -e BINDPLANE_OUTPUT_TYPE=tcp \
  -e BINDPLANE_OUTPUT_TCP_HOST=logs.example.com \
  -e BINDPLANE_OUTPUT_TCP_PORT=9090 \
  -e BINDPLANE_OUTPUT_TCP_WORKERS=3 \
  -e BINDPLANE_LOGGING_LEVEL=info \
  ghcr.io/observiq/blitz:latest
```

For detailed configuration options, see the [Configuration Guide](/docs/configuration.md).

## Community

The Blitz is an open source project. If you'd like to contribute, take a look at our [contribution guidelines](/docs/CONTRIBUTING.md) and [developer guide](/docs/development.md). We look forward to building with you.

## Similar Tools

- [flog](https://github.com/mingrammer/flog) - A fake log generator for common log formats
