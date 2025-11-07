# Apache Common Log Format Generator

The Apache Common generator creates logs in Apache Common Log Format (CLF), a standard format used by Apache HTTP Server and many other web servers. The format follows the specification: `remotehost rfc931 authuser [date] "request" status bytes`.

## Example Logs

```
127.0.0.1 - - [15/Jan/2024:10:30:45 -0500] "GET /api/v1/users HTTP/1.1" 200 2326
192.168.1.100 - - [15/Jan/2024:10:30:46 -0500] "POST /api/v1/orders HTTP/1.1" 201 1543
10.0.0.5 - - [15/Jan/2024:10:30:47 -0500] "GET /health HTTP/1.0" 200 89
```

## Configuration

| YAML Path | Flag Name | Environment Variable | Default | Description |
|-----------|-----------|---------------------|---------|-------------|
| `generator.type` | `--generator-type` | `BLITZ_GENERATOR_TYPE` | `nop` | Generator type. Set to `apache-common` to use this generator. |
| `generator.apache-common.workers` | `--generator-apache-common-workers` | `BLITZ_GENERATOR_APACHE_COMMON_WORKERS` | `1` | Number of Apache Common generator workers (must be ≥ 1) |
| `generator.apache-common.rate` | `--generator-apache-common-rate` | `BLITZ_GENERATOR_APACHE_COMMON_RATE` | `1s` | Rate at which logs are generated per worker (duration format) |

## Example Configuration

```yaml
generator:
  type: apache-common
  apache-common:
    workers: 5
    rate: 100ms
```

## Metrics

The Apache Common generator exposes the following metrics:

- **`blitz_generator_logs_generated_total`** (Counter): Total number of logs generated
- **`blitz_generator_workers_active`** (Gauge): Number of active worker goroutines
- **`blitz_generator_write_errors_total`** (Counter): Total number of write errors, labeled by `error_type` (`unknown` or `timeout`)

All metrics include a `component` label set to `generator_apache`.

