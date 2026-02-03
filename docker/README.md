# Blitz Docker Compose

Docker Compose configurations for running Blitz log generators with Bindplane collectors.

## Pipelines

Three separate pipelines are available, each with production and stage variants:

| Pipeline | Generators | Workers | Prod Port | Stage Port |
|----------|-----------|---------|-----------|------------|
| **web-db** | Apache, PostgreSQL, Kubernetes | Apache: 10x, Others: 1x | 5140 | 5150 |
| **json-pii** | JSON, PII | JSON: 1x, PII: 15x | 5160 | 5170 |
| **okta** | Okta | 1x | 5180 | 5190 |

## Quick Start

### Production

```bash
# Web & DB logs (Apache 10x, PostgreSQL, Kubernetes)
OPAMP_ENDPOINT=wss://app.bindplane.com/v1/opamp \
OPAMP_SECRET_KEY=your-secret-key \
docker compose -f docker/docker-compose.web-db.yml -p web-db up -d

# JSON & PII logs (PII 15x)
OPAMP_ENDPOINT=wss://app.bindplane.com/v1/opamp \
OPAMP_SECRET_KEY=your-secret-key \
docker compose -f docker/docker-compose.json-pii.yml -p json-pii up -d

# Okta logs
OPAMP_ENDPOINT=wss://app.bindplane.com/v1/opamp \
OPAMP_SECRET_KEY=your-secret-key \
docker compose -f docker/docker-compose.okta.yml -p okta up -d
```

### Stage

```bash
# Web & DB logs
OPAMP_ENDPOINT_STAGE=wss://stage.bindplane.com/v1/opamp \
OPAMP_SECRET_KEY_STAGE=your-secret-key \
docker compose -f docker/docker-compose.web-db-stage.yml -p web-db-stage up -d

# JSON & PII logs
OPAMP_ENDPOINT_STAGE=wss://stage.bindplane.com/v1/opamp \
OPAMP_SECRET_KEY_STAGE=your-secret-key \
docker compose -f docker/docker-compose.json-pii-stage.yml -p json-pii-stage up -d

# Okta logs
OPAMP_ENDPOINT_STAGE=wss://stage.bindplane.com/v1/opamp \
OPAMP_SECRET_KEY_STAGE=your-secret-key \
docker compose -f docker/docker-compose.okta-stage.yml -p okta-stage up -d
```

### Run All Pipelines

```bash
# Production - all three pipelines
export OPAMP_ENDPOINT=wss://app.bindplane.com/v1/opamp
export OPAMP_SECRET_KEY=your-secret-key

docker compose -f docker/docker-compose.web-db.yml -p web-db up -d
docker compose -f docker/docker-compose.json-pii.yml -p json-pii up -d
docker compose -f docker/docker-compose.okta.yml -p okta up -d

# Stage - all three pipelines
export OPAMP_ENDPOINT_STAGE=wss://stage.bindplane.com/v1/opamp
export OPAMP_SECRET_KEY_STAGE=your-secret-key

docker compose -f docker/docker-compose.web-db-stage.yml -p web-db-stage up -d
docker compose -f docker/docker-compose.json-pii-stage.yml -p json-pii-stage up -d
docker compose -f docker/docker-compose.okta-stage.yml -p okta-stage up -d
```

## Stopping

```bash
# Production
docker compose -f docker/docker-compose.web-db.yml -p web-db down
docker compose -f docker/docker-compose.json-pii.yml -p json-pii down
docker compose -f docker/docker-compose.okta.yml -p okta down

# Stage
docker compose -f docker/docker-compose.web-db-stage.yml -p web-db-stage down
docker compose -f docker/docker-compose.json-pii-stage.yml -p json-pii-stage down
docker compose -f docker/docker-compose.okta-stage.yml -p okta-stage down
```

## Environment Variables

### Required

| Variable | Description |
|----------|-------------|
| `OPAMP_ENDPOINT` | Production Bindplane OpAMP endpoint |
| `OPAMP_SECRET_KEY` | Production Bindplane secret key |
| `OPAMP_ENDPOINT_STAGE` | Stage Bindplane OpAMP endpoint |
| `OPAMP_SECRET_KEY_STAGE` | Stage Bindplane secret key |

### Optional

| Variable | Default | Description |
|----------|---------|-------------|
| `BLITZ_RATE` | `1s` | Log generation rate |
| `BLITZ_WORKERS` | `1` | Default workers for most generators |
| `BLITZ_APACHE_WORKERS` | `10` | Apache generator workers |
| `BLITZ_PII_WORKERS` | `15` | PII generator workers |
| `BLITZ_OKTA_WORKERS` | `1` | Okta generator workers |

## TCP Ports

| Port | Pipeline | Environment |
|------|----------|-------------|
| 5140 | web-db | Production |
| 5150 | web-db | Stage |
| 5160 | json-pii | Production |
| 5170 | json-pii | Stage |
| 5180 | okta | Production |
| 5190 | okta | Stage |

## Files

| File | Description |
|------|-------------|
| `docker-compose.web-db.yml` | Apache/PostgreSQL/Kubernetes (prod) |
| `docker-compose.web-db-stage.yml` | Apache/PostgreSQL/Kubernetes (stage) |
| `docker-compose.json-pii.yml` | JSON/PII logs (prod) |
| `docker-compose.json-pii-stage.yml` | JSON/PII logs (stage) |
| `docker-compose.okta.yml` | Okta logs (prod) |
| `docker-compose.okta-stage.yml` | Okta logs (stage) |

## Building Local Image

To build and use a local image instead of `ghcr.io/observiq/blitz:latest`:

```bash
# Build the binary
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o package/blitz ./cmd/blitz

# Build the Docker image
docker build -t blitz:local -f package/Dockerfile package/

# Update compose files to use local image
sed -i 's|ghcr.io/observiq/blitz:latest|blitz:local|g' docker/docker-compose.*.yml
```
