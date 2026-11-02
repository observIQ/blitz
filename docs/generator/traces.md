# Traces Generator

**Class:** Producer (embed-eligible; see [docs/embed.md](../embed.md))

The Traces generator produces synthetic distributed trace data. Each trace has an HTTP server root span and 1 to 4 child spans, for 2 to 5 spans in all.

## Telemetry Type

This generator produces **traces** (not logs). It must be paired with an output that supports traces, such as `otlp-grpc` or `stdout`.

## Trace Structure

Each generated trace contains a **root span** (Server): an HTTP request (e.g., `GET /api/users`) with method, path, status code, and host attributes.

Under it are 1 to 4 child spans, each of a different kind picked at random from:

- **Database query** (Client): a PostgreSQL query (e.g., `SELECT users`) with database system, statement, name, and peer attributes.
- **Cache lookup** (Client): a Redis operation with database system, operation, and peer attributes.
- **Internal processing** (Internal): a `process_request` step with a `processing.stage` attribute.
- **Downstream HTTP call** (Client): an HTTP request to another service with method, URL, and peer attributes.
- **Input validation** (Internal): a `validate_input` step with a `validation.scheme` attribute.

All spans within a trace share the same trace ID and have proper parent-child relationships.

## Configuration

| YAML Path                     | Flag Name                      | Environment Variable                | Default | Description                                         |
|-------------------------------|-------------------------------|-------------------------------------|---------|-----------------------------------------------------|
| `generator.type`              | `--generator-type`            | `BLITZ_GENERATOR_TYPE`              | `nop`   | Generator type. Set to `traces` to use this generator. |
| `generator.traces.workers`    | `--generator-traces-workers`  | `BLITZ_GENERATOR_TRACES_WORKERS`    | `1`     | Number of worker goroutines.                        |
| `generator.traces.rate`       | `--generator-traces-rate`     | `BLITZ_GENERATOR_TRACES_RATE`       | `1s`    | Rate at which traces are generated per worker.      |
| `generator.traces.hostname`   | (none, YAML only)             | (none)                              | `""`    | Simulated hostname for span resources. Empty uses the simulated host's hostname (see [environment.md](../environment.md)), or a generated name with no environment host. A set value replaces only the host's name. |
| `generator.traces.seed`       | `--generator-traces-seed`     | `BLITZ_GENERATOR_TRACES_SEED`       | `-1`    | RNG seed for span content. Negative randomizes; `0`+ is deterministic. TraceID/SpanID stay random. |

**Seed contract** (see [datagen](../datagen.md#seed-value-contract)): a negative seed randomizes each run, `0` or any positive value is a deterministic seed (`0` is a real seed, not "unset"), and an omitted seed defaults to `-1` (random). Host identity is not governed by this seed: the CLI and embedded loader always build the simulated environment, which picks the host from `environment.seed_config` (random unless set). The seed shapes the hostname only when the generator is constructed directly in Go with no `Identity`. A randomized run logs its resolved seed (the `seed` field of the start log line), so rerunning with that value reproduces it.

## Example Configuration

```yaml
generator:
  type: traces
  traces:
    workers: 1
    rate: 1s
output:
  type: otlp-grpc
  otlp-grpc:
    host: localhost
    port: 4317
```

## Example CLI Usage

```bash
blitz --generator-type traces --output-type otlp-grpc --output-otlp-grpc-host localhost --output-otlp-grpc-port 4317
```
