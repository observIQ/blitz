# Apache Error Log Format Generator

**Class:** Producer (embed-eligible; see [docs/embed.md](../embed.md))

The Apache Error generator creates logs in Apache Error Log Format, used for logging server errors, warnings, and other diagnostic information. The format follows the specification: `[timestamp] [level] [pid:tid] [client] message`.

## Example Logs

```
[Mon Jan 15 10:30:45.123456 2024] [error] [pid 12345:tid 67890] [client 192.168.1.100:54321] File does not exist: /var/www/html/missing.html
[Mon Jan 15 10:30:46.234567 2024] [warn] [pid 12345:tid 67891] [client 10.0.0.5:12345] Timeout waiting for output from CGI script
[Mon Jan 15 10:30:47.345678 2024] [info] [pid 12345:tid 67892] [client 127.0.0.1:45678] Connection closed by client
```

## Configuration

| YAML Path | Flag Name | Environment Variable | Default | Description |
|-----------|-----------|---------------------|---------|-------------|
| `generator.type` | `--generator-type` | `BLITZ_GENERATOR_TYPE` | `nop` | Generator type. Set to `apache-error` to use this generator. |
| `generator.apache-error.workers` | `--generator-apache-error-workers` | `BLITZ_GENERATOR_APACHE_ERROR_WORKERS` | `1` | Number of Apache Error generator workers (must be ≥ 1) |
| `generator.apache-error.rate` | `--generator-apache-error-rate` | `BLITZ_GENERATOR_APACHE_ERROR_RATE` | `1s` | Rate at which logs are generated per worker (duration format) |

## Example Configuration

```yaml
generator:
  type: apache-error
  apache-error:
    workers: 5
    rate: 100ms
```

## Metrics

The Apache Error generator exposes the following metrics:

- **`blitz_generator_logs_generated_total`** (Counter): Total number of logs generated
- **`blitz_generator_workers_active`** (Gauge): Number of active worker goroutines
- **`blitz_generator_write_errors_total`** (Counter): Total number of write errors, labeled by `error_type` (`unknown` or `timeout`)

All metrics include a `component` label set to `generator_apache_error`.

