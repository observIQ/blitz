# Kubernetes Container Log Generator

**Class:** Producer (embed-eligible; see [docs/embed.md](../embed.md))

The Kubernetes generator creates logs in Kubernetes container log format. Currently supports the CRI-O format, with support for additional formats (such as Docker) planned for the future.

## Description

The Kubernetes container log format follows the CRI-O specification: `<timestamp> <stream> <flag> <application_log>`. The timestamp is in RFC3339 format with nanosecond precision. The stream is randomly selected between `stdout` and `stderr`. The flag is always `F` (full, not partial). The application log content varies and includes JSON web application logs, database logs, and structured key-value logs.

## Example Logs

```
2025-11-10T21:11:47.71558575Z stdout F 21:11:47.715 request_id=GHbBizAYKNxBt5EAIz3x [info] Sent 200 in 1ms
2025-11-10T21:11:48.12345678Z stderr F {"timestamp":"2025-11-10T21:11:48Z","request_id":"AbCdEf123456","level":"error","method":"POST","status":500,"duration":"45.234ms","message":"Internal server error"}
2025-11-10T21:11:49.23456789Z stdout F 21:11:49.234 [LOG] duration: 12.345 ms  statement: SELECT * FROM users WHERE id = $1
2025-11-10T21:11:50.34567890Z stderr F 21:11:50.345 request_id=XyZ789AbC [warn] Rate limit exceeded
2025-11-10T21:11:51.45678901Z stdout F {"timestamp":"2025-11-10T21:11:51Z","request_id":"Def456Ghi","level":"info","method":"GET","status":200,"duration":"2.123ms","message":"Sent 200 in 2.123ms"}
2025-11-10T21:11:52.56789012Z stdout F 21:11:52.567 request_id=JkLmNoPqRs [debug] Cache miss for key
```

## Configuration

| YAML Path | Flag Name | Environment Variable | Default | Description |
|-----------|-----------|---------------------|---------|-------------|
| `generator.type` | `--generator-type` | `BLITZ_GENERATOR_TYPE` | `nop` | Generator type. Set to `kubernetes` to use this generator. |
| `generator.kubernetes.workers` | `--generator-kubernetes-workers` | `BLITZ_GENERATOR_KUBERNETES_WORKERS` | `1` | Number of Kubernetes generator workers (must be ≥ 1) |
| `generator.kubernetes.rate` | `--generator-kubernetes-rate` | `BLITZ_GENERATOR_KUBERNETES_RATE` | `1s` | Rate at which logs are generated per worker (duration format) |
| `generator.kubernetes.format` | `--generator-kubernetes-format` | `BLITZ_GENERATOR_KUBERNETES_FORMAT` | `cri-o` | Container log format. Valid values: `cri-o` |

## Example Configuration

```yaml
generator:
  type: kubernetes
  kubernetes:
    workers: 5
    rate: 100ms
    format: cri-o
```

## Metrics

The Kubernetes generator exposes the following metrics:

- **`blitz.generator.logs.generated`** (Counter): Total number of logs generated
- **`blitz.generator.workers.active`** (Gauge): Number of active worker goroutines
- **`blitz.generator.write.errors`** (Counter): Total number of write errors, labeled by `error_type` (`unknown` or `timeout`)

All metrics include a `component` label set to `generator_kubernetes`.

