# Traces Generator

**Class:** Producer (embed-eligible; see [docs/embed.md](../embed.md))

The Traces generator produces synthetic distributed trace data. Each trace consists of an HTTP server root span, a database query child span, and optionally a processing internal span.

## Telemetry Type

This generator produces **traces** (not logs). It must be paired with an output that supports traces, such as `otlp-grpc` or `stdout`.

## Trace Structure

Each generated trace contains:

1. **Root span** (Server): An HTTP request (e.g., `GET /api/users`) with method, path, status code, and host attributes.
2. **DB child span** (Client): A database query (e.g., `SELECT users`) with database system, statement, and connection attributes.
3. **Processing span** (Internal, 50% chance): An internal processing step with stage attributes.

All spans within a trace share the same trace ID and have proper parent-child relationships.

## Configuration

| YAML Path                     | Flag Name                      | Environment Variable                | Default | Description                                         |
|-------------------------------|-------------------------------|-------------------------------------|---------|-----------------------------------------------------|
| `generator.type`              | `--generator-type`            | `BLITZ_GENERATOR_TYPE`              | `nop`   | Generator type. Set to `traces` to use this generator. |
| `generator.traces.workers`    | `--generator-traces-workers`  | `BLITZ_GENERATOR_TRACES_WORKERS`    | `1`     | Number of worker goroutines.                        |
| `generator.traces.rate`       | `--generator-traces-rate`     | `BLITZ_GENERATOR_TRACES_RATE`       | `1s`    | Rate at which traces are generated per worker.      |

## Example Configuration

```yaml
generator:
  type: traces
  traces:
    workers: 1
    rate: 1s
output:
  type: otlp-grpc
  otlpGrpc:
    host: localhost
    port: 4317
```

## Example CLI Usage

```bash
blitz --generator-type traces --output-type otlp-grpc --output-otlpgrpc-host localhost --output-otlpgrpc-port 4317
```
