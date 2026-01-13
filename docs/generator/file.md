# File Generator

The File generator reads log entries from files on disk. It supports three operational modes: reading from a single file, reading from all files in a directory, or reading from a pre-distributed package of sample logs. All timestamps in the log files should be in ctime format (e.g., `Thu Jan 13 15:30:45 2026`) or other standard formats that can be interpreted using ctime-like format patterns.

## Features

- **Single file mode**: Read logs from a specified file path
- **Directory mode**: Read logs from all files in a directory with optional glob pattern filtering
- **Package mode**: Read logs from pre-distributed sample packages (15+ devices: Apache, NGINX, Palo Alto, Check Point, Fortinet, Cisco ASA, F5 BIG-IP, Linux IPtables, ISC BIND, PostFix, Squid, SonicWALL, Kubernetes, Okta, SNORT, and more)
- **Flexible rate limiting**: Configurable log generation rate per worker
- **Multi-worker support**: Distribute file reading across multiple worker goroutines
- **Ctime timestamp compatibility**: Support for logs with ctime-formatted timestamps and directives (%c, %Y-%m-%d, %H:%M:%S, etc.)

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

## Configuration

| YAML Path | Flag Name | Environment Variable | Default | Description |
|-----------|-----------|---------------------|---------|-------------|
| `generator.type` | `--generator-type` | `BLITZ_GENERATOR_TYPE` | `nop` | Generator type. Set to `file` to use this generator. |
| `generator.file.workers` | `--generator-file-workers` | `BLITZ_GENERATOR_FILE_WORKERS` | `1` | Number of worker goroutines (must be ≥ 1) |
| `generator.file.rate` | `--generator-file-rate` | `BLITZ_GENERATOR_FILE_RATE` | `1s` | Rate at which logs are written per worker (duration format) |
| `generator.file.mode` | `--generator-file-mode` | `BLITZ_GENERATOR_FILE_MODE` | `file` | File reading mode: `file`, `directory`, or `package` |
| `generator.file.source` | `--generator-file-source` | `BLITZ_GENERATOR_FILE_SOURCE` | `` | File path, directory path, or package name depending on mode |
| `generator.file.pattern` | `--generator-file-pattern` | `BLITZ_GENERATOR_FILE_PATTERN` | `*` | Glob pattern for directory mode (optional) |

## Example Configurations

### Single File Mode

Read logs from a single file:

```yaml
generator:
  type: file
  file:
    workers: 2
    rate: 100ms
    mode: file
    source: /var/log/app.log
```

### Directory Mode

Read logs from all `.log` files in a directory:

```yaml
generator:
  type: file
  file:
    workers: 2
    rate: 100ms
    mode: directory
    source: /var/log/application
    pattern: "*.log"
```

### Package Mode

Read logs from a pre-distributed sample package:

```yaml
generator:
  type: file
  file:
    workers: 2
    rate: 100ms
    mode: package
    source: apache
```

Available packages:
- `apache` - Apache HTTP Server access logs
- `nginx` - NGINX HTTP Server access logs
- `palo-alto` - Palo Alto Networks threat and traffic logs
- `checkpoint` - Check Point firewall logs
- `fortinet` - Fortinet FortiGate security appliance logs
- `cisco-asa` - Cisco ASA (Adaptive Security Appliance) VPN and firewall logs
- `f5-bigip` - F5 Networks BIG-IP ASM (Application Security Manager) attack and violation logs

## Behavior

### File Discovery

- **File mode**: Reads the specified file sequentially line by line
- **Directory mode**: Discovers all files matching the pattern (default: `*`) and distributes them across workers
- **Package mode**: Loads pre-packaged data library files from `package/data_library/<package-name>/`

### Worker Distribution

Workers read files sequentially and cycle back to the beginning when all files are exhausted. Each worker processes files in order, with rate limiting applied between each log line write.

### Log Line Processing

Each line in a file is treated atomically:
1. The line is read from the file
2. Empty lines are skipped
3. The line is written to the output with the configured rate limit applied
4. If a write fails, an error is recorded and the next line is processed

### Rate Limiting

Each worker applies exponential backoff with initial interval set to the configured `rate`. This ensures logs are written at the specified rate while handling transient write failures gracefully.

## Timestamp Format Support

Log files should contain timestamps in ctime format (e.g., `Thu Jan 13 15:30:45 2026`). The generator can interpret timestamps using ctime-like format patterns through the internal `ctime` package.

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

Pre-configured data library packages are available at `data_library/`:

- **syslog_generic**: Standard RFC 3164 and RFC 5424 syslog formatted logs representing generic syslog events across various device types
