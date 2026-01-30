# Telemetry Generator

A Docker Compose setup that runs all Blitz log generators simultaneously and sends telemetry to BindPlane via a BindPlane Agent.

## Architecture

```
┌─────────────────┐
│   blitz-json    │──┐
├─────────────────┤  │
│   blitz-pii     │──┤  (10x workers - 37 PII types)
├─────────────────┤  │
│  blitz-winevt   │──┤
├─────────────────┤  │
│ blitz-palo-alto │──┤
├─────────────────┤  │    ┌──────────────────┐    ┌─────────────────┐
│ blitz-apache-*  │──┼───►│  BDOT Collector  │───►│    BindPlane    │
├─────────────────┤  │    │  (OTLP receiver) │    │     (OpAMP)     │
│   blitz-nginx   │──┤    └──────────────────┘    └─────────────────┘
├─────────────────┤  │
│ blitz-postgres  │──┤
├─────────────────┤  │
│ blitz-kubernetes│──┘
└─────────────────┘
```

## Prerequisites

- Docker and Docker Compose
- BindPlane instance with OpAMP enabled
- BindPlane secret key

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
| `OPAMP_ENDPOINT` | BindPlane OpAMP WebSocket endpoint | `wss://app.bindplane.com/v1/opamp` |
| `OPAMP_SECRET_KEY` | BindPlane secret key for authentication | `your-secret-key` |

### Optional Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `BLITZ_RATE` | `1s` | Log generation rate per generator |
| `BLITZ_WORKERS` | `1` | Number of workers per generator |
| `BLITZ_PII_WORKERS` | `10` | Number of workers for PII generator (10x default for comprehensive testing) |

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
| `blitz-pii` | PII | JSON logs with 37 PII types (SSN, credit card, email, passport, API keys, JWT, etc.) - runs at 10x rate |
| `blitz-winevt` | Windows Event | Windows Event logs in XML format |
| `blitz-palo-alto` | Palo Alto | Firewall syslog entries |
| `blitz-apache-common` | Apache Common | Apache Common Log Format (CLF) |
| `blitz-apache-combined` | Apache Combined | Apache Combined Log Format with referer/user-agent |
| `blitz-apache-error` | Apache Error | Apache error log format |
| `blitz-nginx` | NGINX | NGINX Combined Log Format |
| `blitz-postgres` | PostgreSQL | PostgreSQL database logs |
| `blitz-kubernetes` | Kubernetes | Container logs in CRI-O format |

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
| `collector-config.yaml` | BindPlane Agent OTLP receiver configuration |

## Kubernetes Deployment

For Kubernetes deployment, see the `app/telemetry-generator/` directory in the [iris-cluster-config](https://github.com/observIQ/iris-cluster-config) repository.
