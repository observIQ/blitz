# CLAUDE.md

This file provides guidance to Claude Code when working with the Blitz repository.

## Repository Overview

Blitz is an open-source load generation tool for testing OpenTelemetry collectors. It generates synthetic log data in various formats and sends it to configurable destinations.

## Architecture

### Generators (`generator/`)

Each generator creates a specific log format:
- `json/` - Structured JSON logs (supports `default` and `pii` log types)
- `winevt/` - Windows Event logs
- `paloalto/` - Palo Alto firewall logs
- `apache/` - Apache Common Log Format
- `apache_combined/` - Apache Combined Log Format
- `apache_error/` - Apache Error logs
- `nginx/` - NGINX logs
- `postgres/` - PostgreSQL logs
- `kubernetes/` - Kubernetes container logs (CRI-O format)
- `filegen/` - File-based log generation

### Outputs (`output/`)

Outputs send generated logs to destinations:
- `stdout/` - Standard output
- `tcp/` - TCP socket
- `udp/` - UDP socket
- `syslog/` - Syslog protocol
- `otlp/` - OpenTelemetry Protocol (gRPC)
- `file/` - File output with rotation

## Important: Keeping Docker Telemetry Generator in Sync

When adding a new generator to Blitz, you **MUST** also update the Docker telemetry generator setup:

### Files to Update

1. **`docker/docker-compose.telemetry-generator.yml`**
   - Add a new service block for the generator following the existing pattern
   - Use the `x-blitz-common` anchor for common configuration
   - Set appropriate environment variables for the generator type

2. **`docker/README.md`**
   - Add the new generator to the "Included Generators" table
   - Update the architecture diagram if needed

3. **`docs/generator/<name>.md`**
   - Create documentation for the new generator

### Example: Adding a New Generator

When adding a generator called `syslog-rfc5424`, update docker-compose:

```yaml
  # Syslog RFC5424 Log Generator
  blitz-syslog-rfc5424:
    <<: *blitz-common
    environment:
      BLITZ_GENERATOR_TYPE: syslog-rfc5424
      BLITZ_GENERATOR_SYSLOGRFC5424_WORKERS: ${BLITZ_WORKERS:-1}
      BLITZ_GENERATOR_SYSLOGRFC5424_RATE: ${BLITZ_RATE:-1s}
      BLITZ_OUTPUT_TYPE: otlp-grpc
      BLITZ_OUTPUT_OTLPGRPC_HOST: bdot-collector
      BLITZ_OUTPUT_OTLPGRPC_PORT: "4317"
```

## PII Generator

The JSON generator supports a `pii` log type that generates 37 different sensitive data types. When adding new PII types:

1. Update `internal/generator/logtypes/types.go` - Add field to `PIILogData` struct
2. Update `internal/generator/logtypes/pii.go` - Add generator function and call it in `GeneratePIIData()`
3. Update `generator/json/json.go` - Add field to JSON output in `formatAsJSON()`
4. Update `docs/generator/json.md` - Document the new PII type

## Common Commands

```bash
# Build
make build

# Run tests
make test

# Run linter
make lint

# Security scan
make security

# Generate man pages
make man

# Generate shell completions
make completion
```

## Code Style

- Use lowercase "Bindplane" (not "BindPlane") in all documentation and comments
- Follow existing patterns for new generators
- Include metrics for new components
- Add tests for new functionality
