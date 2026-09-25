# File Generator (filegen)

**Class:** Producer (embed-eligible; see [docs/embed.md](../embed.md))

The File generator reads log entries from files on disk and selects a random line from each file on each run. It automatically processes timestamp directives on the selected line. It supports reading from a single file, reading from all files in a directory, or reading from a pre-distributed package of sample logs. Timestamp directives in log entries (like `%c`, `%Y-%m-%dT%H:%M:%SZ`, etc.) are replaced with actual formatted timestamps when the line is selected.

## Features

- **Single file mode**: Read logs from a specified file path
- **Directory mode**: Read logs from all files in a directory
- **Glob pattern support**: Use wildcards to match multiple files or directories (e.g., `/var/log/*rfc5424*.log`, `/data/syslog_*/*.log`)
- **Data library mode**: Reference a named package from blitz's built-in `data_library/` (via bare name or explicit `package:` prefix)
- **Auto-detection**: Mode can auto-detect whether source is a file, directory, or library package
- **Flexible rate limiting**: Configurable log generation rate per worker
- **Multi-worker support**: Distribute file reading across multiple worker goroutines
- **Dynamic timestamp processing**: Automatic substitution of timestamp directives (`%c`, `%Y-%m-%dT%H:%M:%SZ`, `%Y-%m-%d`, etc.) with actual formatted times

## Source resolution — disk path vs data library

The `source` value is resolved in this order:

1. **Explicit `package:` prefix** (e.g. `source: package:syslog_generic`) — resolved only against the data library; no disk fallback. Use this to lock the interpretation and get clear errors on misspellings.
2. **Disk path / glob / directory** — resolved against the local filesystem. This covers absolute paths (`/var/log/app.log`), relative paths (`./logs`), and globs (`/data/syslog_*/*.log`).
3. **Bare name** (no path separator, no prefix; e.g. `source: syslog_generic`) — first tried against the data library; if the library has no matching entry, treated as a disk path. Preserves existing configs that reference data library packages by bare name.

When a bare name matches both a data-library entry AND a directory on disk relative to cwd, a startup warning is logged pointing at the explicit `package:` prefix as the way to disambiguate. The library entry wins; the warning surfaces the ambiguity so users can lock the meaning.

## Standalone CLI vs embedded library use

- **Standalone CLI**: the data library is read from disk at runtime (from `./data_library/` relative to cwd). Editing files there takes effect on the next blitz run — no recompile required. This is the default behavior; the CLI passes no embedded library to `filegen.New`.
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

### Journald (`journalctl -o json`)

The `journald` package emits one JSON object per line in the shape the
OpenTelemetry `journald` receiver consumes — raw journald field names
(`_PID`, `_EXE`, `MESSAGE`, `PRIORITY`, `SYSLOG_IDENTIFIER`, `_HOSTNAME`),
not OpenTelemetry-conventional names.

```
{"__REALTIME_TIMESTAMP":"1783087320123456","_HOSTNAME":"linux-host01.test","_TRANSPORT":"syslog","SYSLOG_IDENTIFIER":"sshd","SYSLOG_TIMESTAMP":"Jan 13 15:30:45","PRIORITY":"6","_COMM":"sshd","_EXE":"/usr/sbin/sshd","_PID":"1842","_SYSTEMD_UNIT":"ssh.service","MESSAGE":"Accepted publickey for deploy from 10.0.4.12 port 51322 ssh2: ED25519 SHA256:abcd1234"}
```

Note: journald's canonical timestamp, `__REALTIME_TIMESTAMP`, is epoch
microseconds, which has no ctime directive — those values are static in the
sample. Records carried over the syslog transport also carry
`SYSLOG_TIMESTAMP`, which does use a directive and advances per emission.

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

## Example Configurations

### File Mode (Single File)

Read logs from a single file:

```yaml
generator:
  type: filegen
  filegen:
    workers: 1
    rate: 1s
    source: /var/log/app.log
```

### Directory Mode (Auto-Detected)

Read logs from all files in a directory:

```yaml
generator:
  type: filegen
  filegen:
    workers: 2
    rate: 100ms
    source: /var/log/application
```

### Glob Pattern Mode (Auto-Detected)

Use glob patterns to read from matching files or directories:

```yaml
generator:
  type: filegen
  filegen:
    workers: 4
    rate: 50ms
    source: data_library/syslog_*/*.log
```

## Behavior

### Auto-Detection

The File generator automatically detects the source type:
- **If source is a file**: Reads the file sequentially line by line
- **If source is a directory**: Discovers all files in the directory and distributes them across workers
- **If source is a glob pattern**: Expands the pattern and processes matching files/directories accordingly

No explicit mode configuration is required; the source type is detected automatically at startup.

### Glob Patterns

The source path supports standard glob patterns with wildcards (`*`, `?`, `[...]`), allowing flexible file selection:

**Important:** To prevent shell expansion, quote the glob pattern when providing it via command line:

```bash
# Quote the pattern to let the Go program expand it
blitz --generator-type=filegen --generator-filegen-source='data_library/*'
```

**File glob examples:**
- `data_library/syslog_generic/unparsable.*` - Match files with `unparsable.` prefix
- `/var/log/*rfc5424*.log` - Match RFC 5424 log files in any subdirectory
- `/data/*.log` - Match all `.log` files in `/data`

**Directory glob examples:**
- `data_library/syslog_*/*.log` - Match all `.log` files in directories starting with `syslog_`
- `/var/log/*/access.log` - Match `access.log` files in any subdirectory of `/var/log`
- `/data/*_logs/*.log` - Match all `.log` files in directories ending with `_logs`
- `data_library/*` - Match all top-level directories in data_library

### Worker Distribution

Workers are assigned files in round-robin fashion. On each rate cycle, each worker:
1. Reads all lines from its assigned file
2. Selects one line at random from the file (skipping empty lines)
3. Writes the selected line to the output
4. Waits for the configured rate period before processing the next file

Workers cycle back to the beginning of the file list when all files are exhausted.

### Log Line Processing

On each work cycle:
1. A file is selected from the worker's assigned files
2. The entire file is read into memory
3. All non-empty lines are collected
4. A random line is selected from the collected lines
5. Timestamp directives in the selected line are processed
6. The line is written to the output with the configured rate limit applied
7. If a write fails, an error is recorded and the next file cycle begins

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
- Caching is **enabled by default** (`cache_enabled: true`)
- Each file's lines are cached in memory after the first read
- Cache entries can have an optional time-to-live (TTL) for automatic invalidation
  - **Default TTL is 0** (cache entries never expire)
  - Setting `cache_ttl` to a value (e.g., `1m`) will invalidate entries older than that duration
- Cache is **thread-safe** and allows concurrent access from multiple worker goroutines
- Cache uses LRU (Least Recently Used) eviction with a 1000-file limit
- Each file maintains its own cache entry independently

**Disabling Cache:**
- Set `cache_enabled: false` to disable caching entirely
- Useful when dealing with very large files or when file contents change frequently

**Example with Cache TTL (1 minute):**
```yaml
generator:
  type: filegen
  filegen:
    workers: 4
    rate: 100ms
    source: /var/log/app.log
    cache_enabled: true
    cache_ttl: 1m
```

**Example with Caching Disabled:**
```yaml
generator:
  type: filegen
  filegen:
    workers: 2
    rate: 500ms
    source: /large/files/directory
    cache_enabled: false
```

## Metrics

The File generator exposes the following metrics:

- **`blitz_generator_logs_generated_total`** (Counter): Total number of log lines written
- **`blitz_generator_workers_active`** (Gauge): Number of active worker goroutines
- **`blitz_generator_write_errors_total`** (Counter): Total number of write errors

All metrics include a `component` label set to `generator_file`.

## Error Handling

If a file cannot be read or a write operation fails:
- The error is logged
- The error counter is incremented
- Processing continues with the next file
- Workers remain active and continue processing

If no files are found in the specified source:
- The generator returns an error during startup
- The blitz process exits with a startup error

## Data Library

The following data library packages are included in the distribution at `data_library/`:

- **syslog_generic**: Standard RFC 3164 and RFC 5424 syslog formatted logs representing generic syslog events across various device types

To use data library packages, simply specify the directory path:

```bash
blitz --generator-type=filegen --generator-filegen-source=data_library/syslog_generic
```

Or use glob patterns to read from multiple data library packages:

```bash
# Read all files from all data library packages
blitz --generator-type=filegen --generator-filegen-source='data_library/*'

# Read from packages starting with 'a' (apache, ahnlab, akamai, etc.)
blitz --generator-type=filegen --generator-filegen-source='data_library/a*'

# Read from all Cisco-related packages
blitz --generator-type=filegen --generator-filegen-source='data_library/cisco*'
```

## Usage Examples

### Apache Web Server Logs to Stdout

Read Apache access logs from the data library and output to stdout:

```bash
blitz --generator-type=filegen \
  --generator-filegen-source=data_library/apache \
  --output-type=stdout
```

### Apache Web Server Logs to OTLP

Send Apache logs to an OpenTelemetry collector:

```bash
blitz --generator-type=filegen \
  --generator-filegen-source=data_library/apache \
  --output-type=otlp-grpc \
  --output-otlpgrpc-host=localhost \
  --output-otlpgrpc-port=4317
```

### Cisco Network Device Logs to Stdout

Generate logs from Cisco network devices with multiple workers:

```bash
blitz --generator-type=filegen \
  --generator-filegen-source=data_library/cisco \
  --generator-filegen-workers=2 \
  --output-type=stdout
```

### Cisco Network Device Logs to OTLP

Send Cisco device logs to a remote OTLP collector:

```bash
blitz --generator-type=filegen \
  --generator-filegen-source=data_library/cisco \
  --generator-filegen-workers=2 \
  --output-type=otlp-grpc \
  --output-otlpgrpc-host=otel-collector.example.com \
  --output-otlpgrpc-port=4317
```

### Kubernetes Cluster Logs to Stdout

Generate Kubernetes cluster logs with high throughput:

```bash
blitz --generator-type=filegen \
  --generator-filegen-source=data_library/kubernetes \
  --generator-filegen-workers=4 \
  --generator-filegen-rate=50ms \
  --output-type=stdout
```

### Kubernetes Cluster Logs to OTLP

Send Kubernetes logs to an OTLP collector with optimized batching:

```bash
blitz --generator-type=filegen \
  --generator-filegen-source=data_library/kubernetes \
  --generator-filegen-workers=4 \
  --generator-filegen-rate=50ms \
  --output-type=otlp-grpc \
  --output-otlpgrpc-host=collector.monitoring.svc \
  --output-otlpgrpc-port=4317 \
  --output-otlpgrpc-workers=2 \
  --output-otlpgrpc-batchtimeout=5s
```

### Fortinet Firewall Logs to Stdout

Generate logs from Fortinet firewalls with standard rate:

```bash
blitz --generator-type=filegen \
  --generator-filegen-source=data_library/fortinet \
  --output-type=stdout
```

### Fortinet Firewall Logs to OTLP

Send Fortinet firewall logs to an OpenTelemetry collector:

```bash
blitz --generator-type=filegen \
  --generator-filegen-source=data_library/fortinet \
  --output-type=otlp-grpc \
  --output-otlpgrpc-host=localhost \
  --output-otlpgrpc-port=4317 \
  --output-otlpgrpc-workers=2
```

### Multiple Checkpoint and Palo Alto Logs to Stdout (Glob Pattern)

Read from firewall packages using glob pattern and output to stdout:

```bash
blitz --generator-type=filegen \
  --generator-filegen-source='data_library/*firewall*' \
  --generator-filegen-workers=4 \
  --output-type=stdout
```

### Multiple Checkpoint and Palo Alto Logs to OTLP (Glob Pattern)

Send security logs from firewall packages to OTLP collector:

```bash
blitz --generator-type=filegen \
  --generator-filegen-source='data_library/*firewall*' \
  --generator-filegen-workers=4 \
  --output-type=otlp-grpc \
  --output-otlpgrpc-host=security-collector.example.com \
  --output-otlpgrpc-port=4317 \
  --output-otlpgrpc-maxexportbatchsize=500 \
  --output-otlpgrpc-workers=3
```

### Microsoft Windows Security Events (Syslog Format) to Stdout

Generate Windows security event logs in syslog format to stdout:

```bash
blitz --generator-type=filegen \
  --generator-filegen-source=data_library/microsoft-windows/security.log \
  --generator-filegen-workers=2 \
  --output-type=stdout
```

### Microsoft Windows Security Events (XML Format) to Stdout

Generate Windows security events in XML format to stdout:

```bash
blitz --generator-type=filegen \
  --generator-filegen-source=data_library/microsoft-windows/security_events.xml \
  --generator-filegen-workers=2 \
  --output-type=stdout
```

### Microsoft Windows Security Events (Syslog Format) to OTLP

Send Windows security events in syslog format to OTLP with TLS:

```bash
blitz --generator-type=filegen \
  --generator-filegen-source=data_library/microsoft-windows/security.log \
  --generator-filegen-workers=2 \
  --output-type=otlp-grpc \
  --output-otlpgrpc-host=secure-collector.example.com \
  --output-otlpgrpc-port=4317 \
  --output-otlpgrpc-enable-tls=true \
  --otlp-grpc-tls-insecure=false \
  --otlp-grpc-tls-cert=/path/to/client.crt \
  --otlp-grpc-tls-key=/path/to/client.key
```

### Microsoft Windows Security Events (XML Format) to OTLP

Send Windows security events in XML format to OTLP with TLS:

```bash
blitz --generator-type=filegen \
  --generator-filegen-source=data_library/microsoft-windows/security_events.xml \
  --generator-filegen-workers=2 \
  --output-type=otlp-grpc \
  --output-otlpgrpc-host=secure-collector.example.com \
  --output-otlpgrpc-port=4317 \
  --output-otlpgrpc-enable-tls=true \
  --otlp-grpc-tls-insecure=false \
  --otlp-grpc-tls-cert=/path/to/client.crt \
  --otlp-grpc-tls-key=/path/to/client.key
```
