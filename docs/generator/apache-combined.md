# Apache Combined Log Format Generator

The Apache Combined generator creates logs in Apache Combined Log Format, which extends the Common Log Format by adding the Referer and User-Agent headers. The format follows the specification: `remotehost rfc931 authuser [date] "request" status bytes "referer" "user-agent"`.

## Example Logs

```
127.0.0.1 - - [15/Jan/2024:10:30:45 -0500] "GET /api/v1/users HTTP/1.1" 200 2326 "https://example.com/dashboard" "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"
192.168.1.100 - - [15/Jan/2024:10:30:46 -0500] "POST /api/v1/orders HTTP/1.1" 201 1543 "https://example.com/cart" "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36"
10.0.0.5 - - [15/Jan/2024:10:30:47 -0500] "GET /health HTTP/1.0" 200 89 "-" "curl/7.68.0"
```

## Configuration

| YAML Path | Flag Name | Environment Variable | Default | Description |
|-----------|-----------|---------------------|---------|-------------|
| `generator.type` | `--generator-type` | `BLITZ_GENERATOR_TYPE` | `nop` | Generator type. Set to `apache-combined` to use this generator. |
| `generator.apache-combined.workers` | `--generator-apache-combined-workers` | `BLITZ_GENERATOR_APACHE_COMBINED_WORKERS` | `1` | Number of Apache Combined generator workers (must be ≥ 1) |
| `generator.apache-combined.rate` | `--generator-apache-combined-rate` | `BLITZ_GENERATOR_APACHE_COMBINED_RATE` | `1s` | Rate at which logs are generated per worker (duration format) |

## Example Configuration

```yaml
generator:
  type: apache-combined
  apache-combined:
    workers: 5
    rate: 100ms
```

## Metrics

The Apache Combined generator exposes the following metrics:

- **`blitz_generator_logs_generated_total`** (Counter): Total number of logs generated
- **`blitz_generator_workers_active`** (Gauge): Number of active worker goroutines
- **`blitz_generator_write_errors_total`** (Counter): Total number of write errors, labeled by `error_type` (`unknown` or `timeout`)

All metrics include a `component` label set to `generator_apache_combined`.

