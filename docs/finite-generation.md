# Finite Generation

Blitz supports generating a finite number of telemetry entries instead of running indefinitely. This is useful for testing scenarios where a specific volume of data is needed, benchmarking, or scripted test workflows.

## Configuration

### Count

The `generator.count` setting controls how many total telemetry entries to generate across all workers. When set to `0` (the default), generation runs indefinitely.

**YAML:**

```yaml
generator:
  type: json
  count: 10000
  json:
    workers: 4
    rate: 100ms
```

**Flag:**

```bash
blitz --generator-type json --generator-count 10000
```

**Environment Variable:**

```bash
BLITZ_GENERATOR_COUNT=10000
```

The count is shared across all workers. For example, with 4 workers and a count of 10000, each worker competes to acquire permits from the shared pool. The total number of entries accepted by the output will be exactly 10000.

### Delivery

The count is of entries the output accepts, not attempts. When a write fails, its permit goes back to the pool and a later write takes it, so the run ends, and `onFinish` applies, only after the output has accepted the last entry. Most outputs (tcp, udp, syslog, file, stdout, hec, otlp-grpc) accept an entry by queuing it and send it afterwards, so an entry that fails on the wire after it was queued is not retried and still counts. On exit, entries still queued are flushed to the destination before blitz stops.

### On Finish

The `onFinish` setting controls what happens when the generation count is reached.

| Value  | Behavior |
|--------|----------|
| `exit` | (Default) Blitz shuts down gracefully after the count is reached. |
| `idle` | Blitz stops generating but stays running. Generation can be restarted with SIGUSR1. |

**YAML:**

```yaml
onFinish: idle
generator:
  type: json
  count: 5000
```

**Flag:**

```bash
blitz --generator-type json --generator-count 5000 --onfinish idle
```

**Environment Variable:**

```bash
BLITZ_ONFINISH=idle
```

### File generator replay

A filegen generator in replay mode (`generator.filegen.mode: replay`) is finite without a count: its count is the number of lines in its input, so the run ends at the end of the input. A lower `count` ends it early. A run-level `generator.count` takes precedence over the generator's own `count`, as for any generator, so a lower generator `count` has no effect when a run-level count is set. A replay must be the only generator. See [Replay Mode](generator/filegen.md#replay-mode).

## Restarting Generation (SIGUSR1)

When using `onFinish: idle`, you can restart generation by sending SIGUSR1 to the Blitz process:

```bash
kill -USR1 $(pidof blitz)
```

This resets the count tracker to the original count value, allowing another round of generation. This can be repeated as many times as needed. An entry in flight when the reset lands does not count toward the new round.

**Note:** SIGUSR1 is only supported on Unix-like systems (Linux, macOS). On Windows, the signal handler is a no-op and a warning is logged at startup.

## Per-Telemetry-Type Semantics

The count represents the total number of telemetry entries accepted by the output, regardless of generator type. Each entry the output accepts consumes one permit from the shared count tracker, with two exceptions: a `traces` permit is one trace, counted once all of its spans are accepted, and a `hostmetrics` permit is one scrape, counted once every scraper's batch is.

For generators with multiple workers, the permits are distributed on a first-come, first-served basis. This means the distribution across workers is not guaranteed to be even, but the total count will always be exact.

## Examples

### Generate exactly 1000 JSON logs and exit

```bash
blitz --generator-type json --generator-count 1000 --output-type stdout
```

### Generate 5000 logs in idle mode, restart with SIGUSR1

```bash
blitz --generator-type apache-common --generator-count 5000 --onfinish idle --output-type stdout
```

In another terminal:

```bash
# Wait for generation to complete, then restart
kill -USR1 $(pidof blitz)
```

### Use in a script with finite output to file

```bash
blitz --generator-type json \
  --generator-count 10000 \
  --generator-json-workers 4 \
  --generator-json-rate 10ms \
  --output-type file \
  --output-file-path /tmp/test-logs.json
```
