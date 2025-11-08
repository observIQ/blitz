# Syslog Output

The Syslog output formats messages per RFC 3164 or RFC 5424 and sends them via UDP or TCP (called "transport"). When using TCP transport, TLS can be enabled.

## Data Mutation

The Syslog output formats log records according to the selected RFC standard (3164 or 5424). The output adds syslog headers including facility, severity, timestamp, hostname, app name, process ID, and message ID. The original log message is included in the syslog message body.

### Example Transformation

**Input log:**
```
{"timestamp":"2024-01-15T10:30:45Z","level":"INFO","message":"User logged in"}
```

**Output (RFC 5424):**
```
<14>1 2024-01-15T10:30:45.000Z workstation blitz 12345 - - [exampleSDID@32473 iut="3" eventSource="Application"] {"timestamp":"2024-01-15T10:30:45Z","level":"INFO","message":"User logged in"}
```

## Configuration

| YAML Path | Flag Name | Environment Variable | Default | Description |
|-----------|-----------|---------------------|---------|-------------|
| `output.type` | `--output-type` | `BLITZ_OUTPUT_TYPE` | `nop` | Output type. Set to `syslog` to use this output. |
| `output.syslog.host` | `--output-syslog-host` | `BLITZ_OUTPUT_SYSLOG_HOST` | `""` | Syslog target host (IP address or hostname) |
| `output.syslog.port` | `--output-syslog-port` | `BLITZ_OUTPUT_SYSLOG_PORT` | `0` | Syslog target port (1-65535) |
| `output.syslog.transport` | `--output-syslog-transport` | `BLITZ_OUTPUT_SYSLOG_TRANSPORT` | `udp` | Transport: `udp` or `tcp` |
| `output.syslog.rfc` | `--output-syslog-rfc` | `BLITZ_OUTPUT_SYSLOG_RFC` | `5424` | Syslog format: `3164` or `5424` |
| `output.syslog.workers` | `--output-syslog-workers` | `BLITZ_OUTPUT_SYSLOG_WORKERS` | `1` | Number of Syslog output workers (must be ≥ 0) |
| `output.syslog.facility` | `--output-syslog-facility` | `BLITZ_OUTPUT_SYSLOG_FACILITY` | `1` | Syslog facility (0–23) |
| `output.syslog.appName` | `--output-syslog-appname` | `BLITZ_OUTPUT_SYSLOG_APPNAME` | `blitz` | App name used in syslog header |
| `output.syslog.hostname` | `--output-syslog-hostname` | `BLITZ_OUTPUT_SYSLOG_HOSTNAME` | `""` | Hostname used in syslog header |
| `output.syslog.procId` | `--output-syslog-procid` | `BLITZ_OUTPUT_SYSLOG_PROCID` | `""` | Process ID used in syslog header |
| `output.syslog.msgId` | `--output-syslog-msgid` | `BLITZ_OUTPUT_SYSLOG_MSGID` | `""` | Message ID used in syslog header |
| `output.syslog.maxDatagramBytes` | `--output-syslog-maxdatagrambytes` | `BLITZ_OUTPUT_SYSLOG_MAXDATAGRAMBYTES` | `0` | UDP safety limit in bytes; if > 0, messages are truncated to fit |

### TLS Configuration (TCP transport only)

TLS is disabled by default. To enable TLS for Syslog over TCP, set `enableTLS: true` and provide certificate and key.

| YAML Path | Flag Name | Environment Variable | Default | Description |
|-----------|-----------|---------------------|---------|-------------|
| `output.syslog.enableTLS` | `--output-syslog-enable-tls` | `BLITZ_OUTPUT_SYSLOG_ENABLE_TLS` | `false` | Enable TLS for Syslog over TCP |
| `output.syslog.tls.cert` | `--output-syslog-tls-cert` | `BLITZ_OUTPUT_SYSLOG_TLS_CERT` | `""` | Path to the TLS certificate file (PEM format) |
| `output.syslog.tls.key` | `--output-syslog-tls-key` | `BLITZ_OUTPUT_SYSLOG_TLS_KEY` | `""` | Path to the TLS private key file (PEM format) |
| `output.syslog.tls.ca` | `--output-syslog-tls-ca` | `BLITZ_OUTPUT_SYSLOG_TLS_CA` | `[]` | Paths to TLS CA certificate files (PEM format). Optional |
| `output.syslog.tls.skipVerify` | `--output-syslog-tls-skip-verify` | `BLITZ_OUTPUT_SYSLOG_TLS_SKIP_VERIFY` | `false` | Whether to skip TLS certificate verification (not recommended) |
| `output.syslog.tls.minVersion` | `--output-syslog-tls-min-version` | `BLITZ_OUTPUT_SYSLOG_TLS_MIN_VERSION` | `1.2` | Minimum TLS version. Valid values: `1.2`, `1.3` |

### Framing and Limitations

- TCP transport currently uses newline-delimited (non-transparent) framing; octet-counting per RFC 6587 is not supported. Many syslog servers accept newline-delimited framing, but if your receiver requires octet-counting, this output will not work as-is.
- UDP transport sends each formatted message as a single datagram. If `maxDatagramBytes` is set and the message would exceed it, the message is truncated to fit.
- To avoid breaking framing, embedded CR/LF in messages are replaced with spaces during formatting.

## Example Configuration

### Syslog over UDP (RFC 5424)

```yaml
output:
  type: syslog
  syslog:
    host: logs.example.com
    port: 514
    transport: udp
    rfc: "5424"
    workers: 2
    facility: 1
    appName: blitz
```

### Syslog over TCP with TLS (RFC 5424)

```yaml
output:
  type: syslog
  syslog:
    host: logs.example.com
    port: 6514
    transport: tcp
    rfc: "5424"
    workers: 2
    facility: 1
    appName: blitz
    hostname: workstation-01
    enableTLS: true
    tls:
      cert: /path/to/cert.pem
      key: /path/to/key.pem
      ca:
        - /path/to/ca.pem
      skipVerify: false
      minVersion: "1.2"
```

