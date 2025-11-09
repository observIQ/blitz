# NGINX Combined Log Format Generator

The NGINX generator creates logs in NGINX's default Combined Log Format. This format matches NGINX's standard `log_format combined` directive and includes the client IP address, remote user, timestamp, request line, status code, response size, referer, and user-agent.

## Description

The NGINX Combined Log Format follows the specification: `$remote_addr - $remote_user [$time_local] "$request" $status $body_bytes_sent "$http_referer" "$http_user_agent"`. This format is identical to Apache Combined Log Format and is widely used for web server access logging.

## Example Logs

```
127.0.0.1 - - [25/Dec/2023:10:15:30 -0800] "GET /index.html HTTP/1.1" 200 2326 "https://www.example.com/" "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"
192.168.1.100 - admin [25/Dec/2023:10:15:31 -0800] "POST /api/v1/users HTTP/1.1" 201 1543 "https://www.google.com/search" "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36"
10.0.0.5 - - [25/Dec/2023:10:15:32 -0800] "GET /health HTTP/1.0" 200 89 "-" "curl/7.68.0"
```

## Configuration

| YAML Path | Flag Name | Environment Variable | Default | Description |
|-----------|-----------|---------------------|---------|-------------|
| `generator.type` | `--generator-type` | `BLITZ_GENERATOR_TYPE` | `nop` | Generator type. Set to `nginx` to use this generator. |
| `generator.nginx.workers` | `--generator-nginx-workers` | `BLITZ_GENERATOR_NGINX_WORKERS` | `1` | Number of NGINX generator workers (must be ≥ 1) |
| `generator.nginx.rate` | `--generator-nginx-rate` | `BLITZ_GENERATOR_NGINX_RATE` | `1s` | Rate at which logs are generated per worker (duration format) |

## Example Configuration

```yaml
generator:
  type: nginx
  nginx:
    workers: 5
    rate: 100ms
```

## Metrics

The NGINX generator exposes the following metrics:

- **`blitz.generator.logs.generated`** (Counter): Total number of logs generated
- **`blitz.generator.workers.active`** (Gauge): Number of active worker goroutines
- **`blitz.generator.write.errors`** (Counter): Total number of write errors, labeled by `error_type` (`unknown` or `timeout`)

All metrics include a `component` label set to `generator_nginx`.

