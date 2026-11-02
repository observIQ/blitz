# File Generator (filegen)

**Class:** Producer (embed-eligible; see [docs/embed.md](../embed.md))

The File generator reads log entries from files on disk. By default it selects a random line from each file on each run; in replay mode it emits every line once, in order, and then stops (see [Replay Mode](#replay-mode)). It automatically processes timestamp directives on the selected line. It supports reading from a single file, reading from all files in a directory, or reading from a pre-distributed package of sample logs. Timestamp directives in log entries (like `%c`, `%Y-%m-%dT%H:%M:%SZ`, etc.) are replaced with actual formatted timestamps when the line is selected.

## Features

- **Single file source**: Read logs from a specified file path
- **Directory source**: Read logs from all files in a directory
- **Glob pattern support**: Use wildcards to match multiple files or directories (e.g., `/var/log/*rfc5424*.log`, `/data/syslog_*/*.log`)
- **Data library source**: Reference a named package from blitz's built-in `data_library/` (via bare name or explicit `package:` prefix)
- **Source detection**: Detects whether `source` is a file, directory, glob, or library package, with no setting needed
- **Replay mode**: Emit every line of your own capture files once, in order, then stop, for byte-for-byte repeatable runs
- **Flexible rate limiting**: Configurable log generation rate per worker
- **Multi-worker support**: Distribute file reading across multiple worker goroutines
- **Dynamic timestamp processing**: Automatic substitution of timestamp directives (`%c`, `%Y-%m-%dT%H:%M:%SZ`, `%Y-%m-%d`, etc.) with actual formatted times

## Source resolution — disk path vs data library

The `source` value is resolved in this order:

1. **Explicit `package:` prefix** (e.g. `source: package:syslog_generic`) — resolved only against the data library; no disk fallback. Use this to lock the interpretation and get clear errors on misspellings.
2. **Disk path / glob / directory** — resolved against the local filesystem. This covers absolute paths (`/var/log/app.log`), relative paths (`./logs`), and globs (`/data/syslog_*/*.log`).
3. **Bare name** (no path separator, no prefix; e.g. `source: syslog_generic`) — first tried as a disk path; if nothing on disk matches, looked up in the data library. Preserves existing configs that reference data library packages by bare name.

A bare name that matches a file, directory, or glob on disk resolves to disk, even when the data library has an entry of the same name. Use the explicit `package:` prefix to force the library.

## Standalone CLI vs embedded library use

- **Standalone CLI**: the data library is read from disk at runtime, from the first of `$BLITZ_DATA_LIBRARY_DIR`, `./data_library/`, the in-repo `generator/filegen/embeddedlibrary/data_library/`, and `/usr/share/blitz/data_library/` that exists. Editing files there takes effect on the next blitz run — no recompile required. The CLI also passes `embeddedlibrary.FS()`, which is empty unless blitz is built with `-tags embed_library`; when both are present, a file on disk wins over the embedded copy of the same path.
- **Embedded library consumers** (e.g. the OTel `telemetrygeneratorreceiver`): import `github.com/observiq/blitz/generator/filegen/embeddedlibrary` and pass `embeddedlibrary.FS()` to `filegen.New`. The data library files are bundled into the consuming binary via `//go:embed` and travel with the import — no on-disk installation step required.

## Example Logs

### Apache Access Log
```
<86>Sun Jun 28 06:00:19 2026 apache.httpserver.test sshd[11148]: pam_vas: Authentication succeeded for Active Directory user
Oct 21 10:05:35 2026 apache.httpserver.test httpd: 10.100.100.101 172.16.210.237 - - [Wed Oct 21 10:05:35 2026] "HEAD / HTTP/1.0" 403 123 "-" "-"
```

### NGINX Access Log
```
Thu Jan 13 15:30:45 2026 nginx.webserver.test nginx: 192.168.1.100 - - [Thu Jan 13 15:30:45 2026] "GET /static/style.css HTTP/1.1" 200 5124 "http://example.com" "Mozilla/5.0"
Fri Jan 14 08:22:17 2026 nginx.webserver.test nginx: 10.20.30.40 - - [Fri Jan 14 08:22:17 2026] "POST /api/data HTTP/1.1" 201 256 "-" "curl/7.68.0"
```

### Palo Alto Threat Log
```
<180>Sun Jun 28 06:00:19 2026 paloalto.paseries.test LEEF:1.0|Palo Alto Networks|PAN-OS Syslog Integration|8.1.6|trojan/PDF.gen.eiez(268198686)|ReceiveTime=2026/06/28 06:00:19|SerialNumber=001801010877|cat=THREAT|Subtype=virus|devTime=Sun Jun 28 06:00:19 2026|src=10.2.75.41|dst=192.168.178.180
```

### Check Point Firewall Log
```
Mon Jan 13 10:15:23 2026 checkpoint.firewall.test Check Point: orig=192.168.1.100 Rule=allow_http Action=Accept Protocol=tcp src=192.168.1.100 dst=203.0.113.1 sport=54321 dport=80
```

## Timestamp Directives

The File generator automatically processes timestamp directives in log files, replacing them with actual formatted timestamps. Use these directives in your log files to enable dynamic timestamp generation:

| Directive | Format | Example |
|-----------|--------|---------|
| `%c` | Complete date and time | `Thu Jan 13 15:30:45 2026` |
| `%Y-%m-%dT%H:%M:%SZ` | ISO 8601 UTC | `2026-01-13T15:30:45Z` |
| `%Y-%m-%dT%H:%M:%S` | ISO 8601 local | `2026-01-13T15:30:45` |
| `%Y-%m-%d` | ISO date only | `2026-01-13` |
| `%H:%M:%S` | Time only | `15:30:45` |
| `%b %e %T` | BSD format | `Jan 13 15:30:45` |
| `%b %d %H:%M:%S` | Syslog format | `Jan 13 15:30:45` |
| `%Y/%m/%d %H:%M:%S` | Common log format | `2026/01/13 15:30:45` |
| `%EPOCH_S` | Unix epoch seconds | `1768318245` |
| `%EPOCH_MS` | Unix epoch milliseconds | `1768318245123` |
| `%EPOCH_US` | Unix epoch microseconds | `1768318245123456` |
| `%EPOCH_NS` | Unix epoch nanoseconds | `1768318245123456789` |

## Configuration

| YAML Path | Flag Name | Environment Variable | Default | Description |
|-----------|-----------|---------------------|---------|-------------|
| `generator.type` | `--generator-type` | `BLITZ_GENERATOR_TYPE` | `nop` | Generator type. Set to `filegen` to use this generator. |
| `generator.filegen.workers` | `--generator-filegen-workers` | `BLITZ_GENERATOR_FILEGEN_WORKERS` | `1` | Number of worker goroutines (must be ≥ 1) |
| `generator.filegen.rate` | `--generator-filegen-rate` | `BLITZ_GENERATOR_FILEGEN_RATE` | `1s` | Rate at which logs are written per worker (duration format) |
| `generator.filegen.source` | `--generator-filegen-source` | `BLITZ_GENERATOR_FILEGEN_SOURCE` | `` | File path, directory path, or glob pattern (auto-detected) |
| `generator.filegen.cache-enabled` | `--generator-filegen-cache-enabled` | `BLITZ_GENERATOR_FILEGEN_CACHE_ENABLED` | `true` | Enable in-memory file caching (true/false) |
| `generator.filegen.cache-ttl` | `--generator-filegen-cache-ttl` | `BLITZ_GENERATOR_FILEGEN_CACHE_TTL` | `0` | Cache entry time-to-live in duration format (0 = never expire) |
| `generator.filegen.mode` | `--generator-filegen-mode` | `BLITZ_GENERATOR_FILEGEN_MODE` | `random` | `random` picks a random line on each emission; `replay` emits every non-empty line once, in order (see [Replay Mode](#replay-mode) for how the run ends). Replay requires `workers: 1`. The old source-type values `file`, `directory` and `package` still load and run `random` |
| `generator.filegen.seed` | `--generator-filegen-seed` | `BLITZ_GENERATOR_FILEGEN_SEED` | `-1` | Random-mode RNG seed. Negative randomizes per run (the resolved seed is logged); 0 or positive repeats the same sequence of line picks with one worker. A failed send uses up its pick, so the lines delivered repeat only when every send succeeds. Worker N uses seed+N, so with several workers each worker's own picks repeat, but which of them a shared count takes does not. Replay ignores it. |

## Example Configurations

### Single File Source

Read logs from a single file:

```yaml
generator:
  type: filegen
  filegen:
    workers: 1
    rate: 1s
    source: /var/log/app.log
```

### Directory Source

Read logs from all files in a directory:

```yaml
generator:
  type: filegen
  filegen:
    workers: 2
    rate: 100ms
    source: /var/log/application
```

### Glob Pattern Source

Use glob patterns to read from matching files or directories:

```yaml
generator:
  type: filegen
  filegen:
    workers: 4
    rate: 50ms
    source: /data/syslog_*/*.log
```

## Replay Mode

Replay mode turns a capture file you supply into a repeatable run. Blitz reads the source once and emits every non-empty line exactly once, in order: a single file top to bottom, or a directory's files in lexical order, each top to bottom. When the last line is out, the run ends. A line whose send fails is retried after the pass, so it arrives out of order (see Ending the run below).

```yaml
generator:
  type: filegen
  filegen:
    source: /data/captures/incident-42.log
    mode: replay
    rate: 10ms
output:
  type: file
  file:
    path: /tmp/replayed.log
```

- **Byte for byte**: a line with no timestamp directives is emitted exactly as it appears in the file, without its line ending (a CRLF line loses its `\r` as well as its `\n`), so captures with hard-coded timestamps replay identically on every run. Directives, if a line has any, render at the current time as they do in random mode. Any `%` followed by a directive character is a directive, so `%%` renders as a single `%`; a line with a literal `%` before a directive character does not replay byte for byte.
- **Ordering**: replay is a single ordered stream, so it requires `workers: 1`. Any other value fails config validation.
- **One generator**: a replay must be the only generator in the run. A `generators:` list that holds a replay and any other entry fails config validation. To replay several captures, put them in one directory.
- **Ending the run**: the run ends once the last line is delivered.
  - Replay waits for each line's result before it sends the next. A line whose send fails goes to a retry queue and replay moves on; after the last line it retries the queued lines, each up to 3 times, logging each retry. A line that still fails is logged as an error and dropped, and the run's count is lowered by one so it can end. Retried lines arrive after the lines that followed them.
  - On a batching output (otlp-grpc, hec), each line waits up to the batch timeout (`batchTimeout`) for its batch to send, so lower the timeout for a faster replay.
  - With HEC ACKs on (the default), each line also waits until Splunk acknowledges its batch, checked every `output.hec.ackPollInterval` (10s by default) and resent after `output.hec.ackTimeout` (5m by default). Lower the interval, or set `output.hec.enableAck: false`, for a faster replay.
  - Each line also waits about one `rate` (1s by default) before the next. The wait is jittered by up to 50% either way.
  - On stdout, each line also waits for the next buffer flush, up to `output.stdout.flushInterval` (100ms by default). Over 100,000 lines that adds up to about three hours (about half that on average), so lower the interval for a faster replay.
  - With `generator.count` set lower than the line count, it ends once that many lines are delivered instead. A failed line then gives its place to the next line, so it can be left in the retry queue unsent.
  - `onFinish: idle` keeps blitz running after the last line. SIGUSR1 then replays again from the first line, including when it arrives mid-replay (see [Finite Generation](../finite-generation.md)).
  - Embedded hosts have no count, so there the generator stops emitting after one pass and keeps running until the host stops it. Without a count, replay also does not wait for each line's result, so an output with several workers can reorder lines.
- **Empty input**: a source with no non-empty lines fails at startup.
- **Unreadable file**: any file in the source that cannot be read, a glob match that cannot be stat'd (such as a dangling symlink), or a matched directory that cannot be listed fails at startup.
- **Directories**: subdirectories, and symlinks to directories, are skipped.
- **Long lines**: lines of any length are read whole, in both modes, so a file with no line breaks reads as one line.
- **Memory**: the whole input is read into memory before the first line is sent, so a capture must fit in RAM.

## Behavior

### Source Detection

The File generator automatically detects the source type:
- **If source is a file**: Reads lines from the file (a random line per run, or every line in order in replay mode)
- **If source is a directory**: Discovers all files in the directory and distributes them across workers
- **If source is a glob pattern**: Expands the pattern and processes matching files/directories accordingly

The source type needs no configuration; it is detected at startup. It is separate from `mode`, which picks random or replay.

### Glob Patterns

The source path supports standard glob patterns with wildcards (`*`, `?`, `[...]`), allowing flexible file selection. Globs match files on disk only; they do not expand inside the data library. To glob over library files, copy them to disk first with `blitz library extract`.

**Important:** To prevent shell expansion, quote the glob pattern when providing it via command line:

```bash
# Quote the pattern to let the Go program expand it
blitz --generator-type=filegen --generator-filegen-source='/data/*'
```

A `*` never matches a `/`, so each `*` covers one path level. A directory the pattern matches contributes the files directly inside it.

**File glob examples:**
- `/data/syslog/unparsable.*` - Match files with `unparsable.` prefix
- `/var/log/*rfc5424*.log` - Match RFC 5424 log files directly in `/var/log`
- `/data/*.log` - Match all `.log` files in `/data`

**Directory glob examples:**
- `/data/syslog_*/*.log` - Match all `.log` files in directories starting with `syslog_`
- `/var/log/*/access.log` - Match `access.log` in each directory directly under `/var/log`
- `/data/*_logs/*.log` - Match all `.log` files in directories ending with `_logs`
- `/data/*` - Match the files directly in `/data`, plus the files directly inside each of its subdirectories

### Worker Distribution

This section describes random mode. Replay uses a single worker that walks every file in order (see [Replay Mode](#replay-mode)).

Each worker starts at its own file (worker N at file N) and steps forward by the worker count on each cycle, wrapping at the end of the file list, so the workers together cover every file. On each rate cycle, each worker:
1. Reads all lines from its current file
2. Selects one line at random from the file (skipping empty lines)
3. Writes the selected line to the output
4. Waits for the configured rate period before moving to its next file

### Log Line Processing

On each work cycle:
1. A file is selected from the worker's assigned files
2. The entire file is read into memory
3. All non-empty lines are collected
4. A random line is selected from the collected lines, drawn from the worker's seeded RNG (`generator.filegen.seed`)
5. Timestamp directives in the selected line are processed
6. The line is written to the output with the configured rate limit applied
7. If the write fails, an error is recorded and the worker retries the same file after a backoff. If the read fails, or the file has no non-empty lines, the worker moves on to the following file in the list

### Rate Limiting

Each worker applies exponential backoff with initial interval set to the configured `rate`. This ensures logs are written at the specified rate while handling transient write failures gracefully.

## Timestamp Format Support

Log files should contain timestamps in ctime format (e.g., `Thu Jan 13 15:30:45 2026`). The generator can interpret timestamps using ctime-like format patterns through the internal `ctime` package.

For more information on ctime formatting, see the [ctime formatting guide](https://docs.bindplane.com/how-to-guides/ctime-formatting).

### Format Pattern Directives

Common ctime-like format directives supported:

- `%Y` - Year (2026)
- `%m` - Month (01-12)
- `%d` - Day (01-31)
- `%A` - Full weekday name (Monday, Tuesday, ...)
- `%a` - Abbreviated weekday name (Mon, Tue, ...)
- `%B` - Full month name (January, February, ...)
- `%b` - Abbreviated month name (Jan, Feb, ...)
- `%H` - Hour (00-23)
- `%M` - Minute (00-59)
- `%S` - Second (00-59)
- `%c` - Complete date and time (Mon Jan 13 15:30:45 2026)

Example log lines with various timestamp formats:
```
Thu Jan 13 15:30:45 2026              # %c format
2026-01-13 15:30:45                   # %Y-%m-%d %H:%M:%S format
Jan 13 15:30:45                       # %b %d %H:%M:%S format
```
## Caching

The File generator implements an in-memory cache to avoid reading files from disk on every log generation cycle. This significantly improves performance when working with large numbers of files or files with many lines.

**Cache Behavior:**
- Caching is **enabled by default** (`cache-enabled: true`)
- Each file's lines are cached in memory after the first read
- Cache entries can have an optional time-to-live (TTL) for automatic invalidation
  - **Default TTL is 0** (cache entries never expire)
  - Setting `cache-ttl` to a value (e.g., `1m`) will invalidate entries older than that duration
- Cache is **thread-safe** and allows concurrent access from multiple worker goroutines
- Cache uses LRU (Least Recently Used) eviction with a 1000-file limit
- Each file maintains its own cache entry independently

**Disabling Cache:**
- Set `cache-enabled: false` to disable caching entirely
- Useful when dealing with very large files or when file contents change frequently

**Example with Cache TTL (1 minute):**
```yaml
generator:
  type: filegen
  filegen:
    workers: 4
    rate: 100ms
    source: /var/log/app.log
    cache-enabled: true
    cache-ttl: 1m
```

**Example with Caching Disabled:**
```yaml
generator:
  type: filegen
  filegen:
    workers: 2
    rate: 500ms
    source: /large/files/directory
    cache-enabled: false
```

## Metrics

The File generator exposes the following metrics:

- **`blitz.generator.entries`** (Counter): Total number of log lines written
- **`blitz.generator.active_workers`** (Gauge): Number of active worker goroutines
- **`blitz.generator.write_errors`** (Counter): Total number of write errors

All metrics carry a `generator_type` attribute set to `file`. See [Metrics](../metrics.md) for the exported names.

## Error Handling

This section describes random mode. Replay reads its whole input at startup, so an unreadable file stops it there (see [Replay Mode](#replay-mode)).

If a write operation fails:
- The error is logged
- The error counter is incremented
- The worker retries the same file after an exponential backoff (from the rate up to 5s, or up to the rate when that is longer)
- Workers remain active and continue processing

If a file cannot be read, the error is logged and counted the same way, and the worker moves on to the following file in the list after the backoff. A file with no non-empty lines is skipped the same way, without an error. A path a glob matches but blitz cannot stat, or a matched directory it cannot list, is skipped at startup with a warning.

If no files are found in the specified source, or no file has a non-empty line:
- The generator returns an error during startup
- The blitz process exits with a startup error

The startup check skips files it cannot read, so a source whose files cannot be read still starts. The workers then report each read failure as described above.

## Data Library

The data library ships about 190 packages of sample logs, one per product (for example `apache`, `cisco-asa`, `kubernetes`, `palo-alto`, `syslog_generic`). The `blitz library` command inspects it:

```bash
blitz library ls                    # list every package
blitz library ls microsoft-windows  # list the files in one package
blitz library search firewall       # find packages by name
```

Use a package by its bare name, or name one file in it with the `package:` prefix:

```bash
blitz --generator-type=filegen --generator-filegen-source=syslog_generic
blitz --generator-type=filegen --generator-filegen-source=package:microsoft-windows/security.log
```

A source with a path separator and no `package:` prefix is always read from disk, so `data_library/syslog_generic` works only from a directory that holds a `data_library/` copy, such as an unpacked release tarball. Bare names work from anywhere. To read several packages at once, extract them and point a glob at the copy:

```bash
blitz library extract --all /tmp/blitz
blitz --generator-type=filegen --generator-filegen-source='/tmp/blitz/data_library/cisco*'
```

## Usage Examples

### Apache Web Server Logs to Stdout

Read Apache access logs from the data library and output to stdout:

```bash
blitz --generator-type=filegen \
  --generator-filegen-source=apache \
  --output-type=stdout
```

### Apache Web Server Logs to OTLP

Send Apache logs to an OpenTelemetry collector:

```bash
blitz --generator-type=filegen \
  --generator-filegen-source=apache \
  --output-type=otlp-grpc \
  --output-otlp-grpc-host=localhost \
  --output-otlp-grpc-port=4317
```

### Cisco Network Device Logs to Stdout

Generate logs from Cisco network devices with multiple workers:

```bash
blitz --generator-type=filegen \
  --generator-filegen-source=cisco \
  --generator-filegen-workers=2 \
  --output-type=stdout
```

### Cisco Network Device Logs to OTLP

Send Cisco device logs to a remote OTLP collector:

```bash
blitz --generator-type=filegen \
  --generator-filegen-source=cisco \
  --generator-filegen-workers=2 \
  --output-type=otlp-grpc \
  --output-otlp-grpc-host=otel-collector.example.com \
  --output-otlp-grpc-port=4317
```

### Kubernetes Cluster Logs to Stdout

Generate Kubernetes cluster logs with high throughput:

```bash
blitz --generator-type=filegen \
  --generator-filegen-source=kubernetes \
  --generator-filegen-workers=4 \
  --generator-filegen-rate=50ms \
  --output-type=stdout
```

### Kubernetes Cluster Logs to OTLP

Send Kubernetes logs to an OTLP collector with optimized batching:

```bash
blitz --generator-type=filegen \
  --generator-filegen-source=kubernetes \
  --generator-filegen-workers=4 \
  --generator-filegen-rate=50ms \
  --output-type=otlp-grpc \
  --output-otlp-grpc-host=collector.monitoring.svc \
  --output-otlp-grpc-port=4317 \
  --output-otlp-grpc-workers=2 \
  --output-otlp-grpc-batchtimeout=5s
```

### Fortinet Firewall Logs to Stdout

Generate logs from Fortinet firewalls with standard rate:

```bash
blitz --generator-type=filegen \
  --generator-filegen-source=fortinet \
  --output-type=stdout
```

### Fortinet Firewall Logs to OTLP

Send Fortinet firewall logs to an OpenTelemetry collector:

```bash
blitz --generator-type=filegen \
  --generator-filegen-source=fortinet \
  --output-type=otlp-grpc \
  --output-otlp-grpc-host=localhost \
  --output-otlp-grpc-port=4317 \
  --output-otlp-grpc-workers=2
```

### Palo Alto Firewall Logs to Stdout

Read Palo Alto firewall logs and output to stdout:

```bash
blitz --generator-type=filegen \
  --generator-filegen-source=palo-alto \
  --generator-filegen-workers=4 \
  --output-type=stdout
```

### Palo Alto Firewall Logs to OTLP

Send Palo Alto firewall logs to an OTLP collector:

```bash
blitz --generator-type=filegen \
  --generator-filegen-source=palo-alto \
  --generator-filegen-workers=4 \
  --output-type=otlp-grpc \
  --output-otlp-grpc-host=security-collector.example.com \
  --output-otlp-grpc-port=4317 \
  --output-otlp-grpc-maxexportbatchsize=500 \
  --output-otlp-grpc-workers=3
```

### Microsoft Windows Security Events (Syslog Format) to Stdout

Generate Windows security event logs in syslog format to stdout:

```bash
blitz --generator-type=filegen \
  --generator-filegen-source=package:microsoft-windows/security.log \
  --generator-filegen-workers=2 \
  --output-type=stdout
```

### Microsoft Windows Security Events (XML Format) to Stdout

Generate Windows security events in XML format to stdout:

```bash
blitz --generator-type=filegen \
  --generator-filegen-source=package:microsoft-windows/security_events.xml \
  --generator-filegen-workers=2 \
  --output-type=stdout
```

### Microsoft Windows Security Events (Syslog Format) to OTLP

Send Windows security events in syslog format to OTLP with TLS:

```bash
blitz --generator-type=filegen \
  --generator-filegen-source=package:microsoft-windows/security.log \
  --generator-filegen-workers=2 \
  --output-type=otlp-grpc \
  --output-otlp-grpc-host=secure-collector.example.com \
  --output-otlp-grpc-port=4317 \
  --output-otlp-grpc-enable-tls=true \
  --otlp-grpc-tls-insecure=false \
  --otlp-grpc-tls-cert=/path/to/client.crt \
  --otlp-grpc-tls-key=/path/to/client.key
```

### Microsoft Windows Security Events (XML Format) to OTLP

Send Windows security events in XML format to OTLP with TLS:

```bash
blitz --generator-type=filegen \
  --generator-filegen-source=package:microsoft-windows/security_events.xml \
  --generator-filegen-workers=2 \
  --output-type=otlp-grpc \
  --output-otlp-grpc-host=secure-collector.example.com \
  --output-otlp-grpc-port=4317 \
  --output-otlp-grpc-enable-tls=true \
  --otlp-grpc-tls-insecure=false \
  --otlp-grpc-tls-cert=/path/to/client.crt \
  --otlp-grpc-tls-key=/path/to/client.key
```
