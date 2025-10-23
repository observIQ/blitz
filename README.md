# Bindplane Loader

A load generation tool for Bindplane managed collectors.

## Documentation

- [Configuration Guide](/docs/configuration.md) - Complete guide to configuring bindplane-loader with YAML files, environment variables, and command-line flags
- [Architecture Overview](/docs/architecture.md) - Detailed explanation of the application architecture, components, and data flow
- [Development Guide](/docs/development.md) - Guidelines for contributing to the project
- [Contributing Guidelines](/docs/CONTRIBUTING.md) - How to contribute to the project

## Installation

### CLI

Download the binary for your platform from the [latest release](https://github.com/observiq/bindplane-loader/releases/latest):

Extract the archive and run the binary directly in a terminal:

```bash
tar -xzf bindplane-loader_*_linux_amd64.tar.gz
```

Run with default NOP configuration:

```bash
./bindplane-loader
```

Run with JSON generator and TCP output:

```bash
./bindplane-loader \
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

Download the appropriate package for your Linux distribution from the [latest release](https://github.com/observiq/bindplane-loader/releases/latest):

- **Debian/Ubuntu**: `bindplane-loader_amd64.deb` or `bindplane-loader_arm64.deb`
- **Red Hat/CentOS/Fedora**: `bindplane-loader_amd64.rpm` or `bindplane-loader_arm64.rpm`

#### Debian/Ubuntu Installation

Install the package with your package manager:

**Debian**

```bash
sudo apt-get install -f ./bindplane-loader_amd64.deb
```

**RHEL**

```bash
sudo dnf install ./bindplane-loader_amd64.rpm
```

Edit the configuration file:

```bash
sudo vi /etc/bindplane-loader/config.yaml
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
sudo systemctl enable bindplane-loader
sudo systemctl start bindplane-loader
sudo systemctl status bindplane-loader
```

View service logs:

```bash
sudo journalctl -u bindplane-loader -f
```

### Container

Pull the Docker image from GitHub Container Registry and run it with environment variables for configuration:

Run with default NOP configuration:

```bash
docker run --rm ghcr.io/observiq/bindplane-loader:latest
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
  ghcr.io/observiq/bindplane-loader:latest
```

For detailed configuration options, see the [Configuration Guide](/docs/configuration.md).

## Community

The Bindplane Loader is an open source project. If you'd like to contribute, take a look at our [contribution guidelines](/docs/CONTRIBUTING.md) and [developer guide](/docs/development.md). We look forward to building with you.
