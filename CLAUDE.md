# CLAUDE.md

This file provides guidance to Claude Code when working with the Blitz repository.

## Repository Overview

Blitz is an open-source load generation tool for testing OpenTelemetry collectors. It generates synthetic logs, metrics, traces, and flow records and sends them to configurable destinations.

## Architecture

### Generators (`generator/`)

Each generator creates one kind of telemetry. The README lists them all with links to their docs:
- `json/` - Structured JSON logs (supports `default` and `pii` log types)
- `winevt/` - Windows Event logs in XML (deprecated; use `wel/`)
- `wel/` - Windows Event Log entries across many channels
- `paloalto/` - Palo Alto firewall logs
- `apache/` - Apache Common Log Format
- `apache_combined/` - Apache Combined Log Format
- `apache_error/` - Apache Error logs
- `nginx/` - NGINX logs
- `postgres/` - PostgreSQL logs
- `kubernetes/` - Kubernetes container logs (CRI-O format)
- `filegen/` - Lines from files or the data library, random or replayed in order
- `okta/` - Okta System Log events
- `f5/` - F5 product-portfolio syslog
- `fix/` - FIX protocol messages
- `hostmetrics/` - Host metrics
- `traces/` - Distributed traces
- `flow/` - Network flow records
- `nop/` - No-op generator

### Outputs (`output/`)

Outputs send generated telemetry to destinations:
- `stdout/` - Standard output
- `tcp/` - TCP socket
- `udp/` - UDP socket
- `syslog/` - Syslog protocol
- `otlp_grpc/` - OpenTelemetry Protocol (gRPC)
- `file/` - File output with rotation
- `hec/` - Splunk HTTP Event Collector
- `flow/` - Flow records over UDP (NetFlow, sFlow, IPFIX)
- `promrw/` - Prometheus remote-write push
- `promscrape/` - Prometheus scrape endpoint
- `nop/` - No-op output

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
      BLITZ_GENERATOR_SYSLOG_RFC5424_WORKERS: ${BLITZ_WORKERS:-1}
      BLITZ_GENERATOR_SYSLOG_RFC5424_RATE: ${BLITZ_RATE:-1s}
      BLITZ_OUTPUT_TYPE: otlp-grpc
      BLITZ_OUTPUT_OTLP_GRPC_HOST: bdot-collector
      BLITZ_OUTPUT_OTLP_GRPC_PORT: "4317"
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
- Generators honor `generator.count`: take a permit with the tracker's `Acquire` before each record, then `Commit` it once the record is written or `Release` it when the write fails (see `generator/count`)
- Include metrics for new components
- Add tests for new functionality
