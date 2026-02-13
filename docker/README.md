# Telemetry Generator

A Docker Compose setup that runs all Blitz log generators simultaneously and sends telemetry to Bindplane via a Bindplane Agent.

## Architecture

```
┌─────────────────┐
│   blitz-json    │──┐
├─────────────────┤  │
│   blitz-pii     │──┤  (37 PII types)
├─────────────────┤  │
│  blitz-winevt   │──┤
├─────────────────┤  │
│ blitz-palo-alto │──┤
├─────────────────┤  │    ┌──────────────────┐    ┌─────────────────┐
│ blitz-apache-*  │──┼───►│  BDOT Collector  │───►│    Bindplane    │
├─────────────────┤  │    │  (OTLP receiver) │    │     (OpAMP)     │
│   blitz-nginx   │──┤    └──────────────────┘    └─────────────────┘
├─────────────────┤  │
│ blitz-postgres  │──┤
├─────────────────┤  │
│ blitz-kubernetes│──┤
├─────────────────┤  │
│   blitz-okta    │──┘
└─────────────────┘
```

## Prerequisites

- Docker and Docker Compose
- Bindplane instance with OpAMP enabled
- Bindplane secret key

## Quick Start

```bash
# From the blitz repo root directory
OPAMP_ENDPOINT=wss://your-bindplane.com/v1/opamp \
OPAMP_SECRET_KEY=your-secret-key \
docker compose -f docker/docker-compose.telemetry-generator.yml up
```

## Configuration

### Required Environment Variables

| Variable | Description | Example |
|----------|-------------|---------|
| `OPAMP_ENDPOINT` | Bindplane OpAMP WebSocket endpoint | `wss://app.bindplane.com/v1/opamp` |
| `OPAMP_SECRET_KEY` | Bindplane secret key for authentication | `your-secret-key` |

### Optional Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `BLITZ_RATE` | `1s` | Log generation rate per generator |
| `BLITZ_WORKERS` | `1` | Number of workers per generator |
| `BLITZ_PII_WORKERS` | `1` | Number of workers for PII generator |

### Examples

**Increase log generation rate:**
```bash
OPAMP_ENDPOINT=wss://your-bindplane.com/v1/opamp \
OPAMP_SECRET_KEY=your-secret-key \
BLITZ_RATE=100ms \
docker compose -f docker/docker-compose.telemetry-generator.yml up
```

**Run with more workers:**
```bash
OPAMP_ENDPOINT=wss://your-bindplane.com/v1/opamp \
OPAMP_SECRET_KEY=your-secret-key \
BLITZ_WORKERS=3 \
docker compose -f docker/docker-compose.telemetry-generator.yml up
```

**Run in background:**
```bash
OPAMP_ENDPOINT=wss://your-bindplane.com/v1/opamp \
OPAMP_SECRET_KEY=your-secret-key \
docker compose -f docker/docker-compose.telemetry-generator.yml up -d
```

## Included Generators

| Generator | Log Type | Description |
|-----------|----------|-------------|
| `blitz-json` | JSON | Structured JSON logs |
| `blitz-pii` | PII | JSON logs with 37 PII types (SSN, credit card, email, passport, API keys, JWT, etc.) |
| `blitz-winevt` | Windows Event | Windows Event logs in XML format |
| `blitz-palo-alto` | Palo Alto | Firewall syslog entries |
| `blitz-apache-common` | Apache Common | Apache Common Log Format (CLF) with security attack patterns |
| `blitz-apache-combined` | Apache Combined | Apache Combined Log Format with referer/user-agent |
| `blitz-apache-error` | Apache Error | Apache error log format |
| `blitz-nginx` | NGINX | NGINX Combined Log Format with security attack patterns |
| `blitz-postgres` | PostgreSQL | PostgreSQL database logs with security events |
| `blitz-kubernetes` | Kubernetes | Container logs in CRI-O format with security events |
| `blitz-okta` | Okta | Okta System Log events (authentication, security, lifecycle) |

## Running Individual Generators

To run only specific generators:

```bash
OPAMP_ENDPOINT=wss://your-bindplane.com/v1/opamp \
OPAMP_SECRET_KEY=your-secret-key \
docker compose -f docker/docker-compose.telemetry-generator.yml up bdot-collector blitz-json blitz-nginx
```

## Stopping

```bash
docker compose -f docker/docker-compose.telemetry-generator.yml down
```

## Files

| File | Description |
|------|-------------|
| `docker-compose.telemetry-generator.yml` | Docker Compose configuration |

## Building Local Image

To build and use a local image instead of `ghcr.io/observiq/blitz:latest`:

```bash
# Build the binary
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o package/blitz ./cmd/blitz

# Build the Docker image
docker build -t blitz:local -f package/Dockerfile package/

# Update compose file to use local image
sed -i 's|ghcr.io/observiq/blitz:latest|blitz:local|g' docker/docker-compose.telemetry-generator.yml
```

## Kubernetes Deployment

For Kubernetes deployment, see the `app/telemetry-generator/` directory in the [iris-cluster-config](https://github.com/observIQ/iris-cluster-config) repository.
