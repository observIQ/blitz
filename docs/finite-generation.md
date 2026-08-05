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

The count is shared across all workers. For example, with 4 workers and a count of 10000, each worker competes to acquire permits from the shared pool. The total number of entries generated will be exactly 10000.

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

## Restarting Generation (SIGUSR1)

When using `onFinish: idle`, you can restart generation by sending SIGUSR1 to the Blitz process:

```bash
kill -USR1 $(pidof blitz)
```

This resets the count tracker to the original count value, allowing another round of generation. This can be repeated as many times as needed.

**Note:** SIGUSR1 is only supported on Unix-like systems (Linux, macOS). On Windows, the signal handler is a no-op and a warning is logged at startup.

## Per-Telemetry-Type Semantics

The count represents the total number of individual telemetry entries generated, regardless of generator type. Each call to the generator's write function that produces an entry consumes one permit from the shared count tracker.

For generators with multiple workers, the permits are distributed on a first-come, first-served basis using atomic operations. This means the distribution across workers is not guaranteed to be even, but the total count will always be exact.

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
