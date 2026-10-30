# Finite Generation

Blitz supports generating a finite number of telemetry entries instead of running indefinitely. This is useful for testing scenarios where a specific volume of data is needed, benchmarking, or scripted test workflows.

## Configuration

### Count

The `generator.count` setting controls how many total telemetry entries to deliver across all workers. When set to `0` (the default), generation runs indefinitely.

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

The count is shared across all workers. For example, with 4 workers and a count of 10000, each worker competes to acquire permits from the shared pool. The run counts exactly 10000 delivered entries. The destination can receive a few more, since a replacement can duplicate an entry that was in fact delivered (see [Delivery](#delivery)).

### Delivery

The count is of entries delivered to the destination, not attempts. Most outputs queue an entry and send it later, so an entry counts only once its send succeeds. When a send fails, or an output drops a queued entry, its permit goes back to the pool and the generator makes a replacement. The run ends, and `onFinish` applies, only after the last entry is delivered, so a run against a destination that never accepts keeps retrying until blitz is stopped.

Delivery is at least once. When blitz cannot tell which entries of a failed send arrived, it replaces all of them, so the destination may receive duplicates but never fewer entries than the count.

Each output counts an entry as delivered at this point:

| Output                    | Delivered when                                                                   |
|---------------------------|----------------------------------------------------------------------------------|
| `tcp`, `syslog` over TCP  | the socket write returns                                                         |
| `udp`, `syslog` over UDP  | the datagram is sent (UDP has no acknowledgement)                                |
| `file`                    | the line is written to the file                                                  |
| `stdout`                  | the buffer holding the record is flushed                                         |
| `hec`                     | Splunk acknowledges the batch, or an HTTP 2xx with ACKs off or no ackId returned |
| `otlp-grpc`               | the export call for the batch succeeds with nothing rejected                     |
| `prometheus-remote-write` | the endpoint accepts the batch                                                   |
| `prometheus-scrape`       | the sample is stored for scraping                                                |
| `flow`                    | the packet is sent                                                               |
| `nop`                     | the write returns                                                                |

A TCP write returns once the kernel has the data, so a destination that resets the connection can still lose the entries written just before the reset.

With several `outputs:`, an entry counts once every output has delivered it. If any output fails, the replacement goes to all of them, so the outputs that succeeded receive a duplicate. HEC with ACKs on resends a batch whose acknowledgement times out, including when the ACK endpoint cannot be queried, so Splunk can index that batch twice. An otlp-grpc collector that rejects part of a batch (a partial success) does not say which entries it rejected, so the whole batch is replaced, and the entries it accepted arrive twice.

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

The count represents the total number of telemetry entries delivered, regardless of generator type. Each delivered entry consumes one permit from the shared count tracker, with two exceptions: a `traces` permit is one trace, counted once all of its spans are delivered, and a `hostmetrics` permit is one scrape, counted once every scraper's batch is. A failed batch replaces the whole scrape, so the scrapers that succeeded send a duplicate.

For generators with multiple workers, the permits are distributed on a first-come, first-served basis. This means the distribution across workers is not guaranteed to be even, but the run always ends once exactly the count has been delivered (see [Delivery](#delivery) for the extras a destination can still receive).

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
