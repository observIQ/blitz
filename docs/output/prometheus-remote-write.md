# Prometheus Remote-Write Output

The prometheus-remote-write output is a metrics-only push client. It POSTs snappy-compressed remote-write payloads to a configured endpoint, so blitz can load a Prometheus server or any remote-write receiver directly. Remote-write 1.0 and 2.0 are both supported.

## Metric Mapping

Generated metric points map to Prometheus series before encoding:

- Gauge and non-monotonic Sum become a gauge.
- Counter becomes a counter with a `_total` suffix.
- Histogram becomes cumulative `_bucket` series with `le` labels, including `+Inf`, plus `_sum` and `_count`.

Metric and label names are sanitized to the Prometheus grammar. Each series carries its `__name__` plus the metric's attributes as labels, with the value and millisecond timestamp from the point. Remote-write 2.0 de-duplicates label strings into a symbol table.

## Configuration

| YAML Path                                     | Flag                                            | Environment Variable                                | Default | Description                                          |
|-----------------------------------------------|-------------------------------------------------|-----------------------------------------------------|---------|------------------------------------------------------|
| `output.type`                                 | `--output-type`                                 | `BLITZ_OUTPUT_TYPE`                                 | `nop`   | Set to `prometheus-remote-write` to use this output. |
| `output.prometheus-remote-write.endpoint`     | `--output-prometheus-remote-write-endpoint`     | `BLITZ_OUTPUT_PROMETHEUS_REMOTE_WRITE_ENDPOINT`     | `""`    | Remote-write endpoint URL. Required, http or https.  |
| `output.prometheus-remote-write.version`      | `--output-prometheus-remote-write-version`      | `BLITZ_OUTPUT_PROMETHEUS_REMOTE_WRITE_VERSION`      | `1.0`   | Protocol version: `1.0` or `2.0`.                    |
| `output.prometheus-remote-write.batchSize`    | `--output-prometheus-remote-write-batchsize`    | `BLITZ_OUTPUT_PROMETHEUS_REMOTE_WRITE_BATCHSIZE`    | `500`   | Series buffered before a flush.                      |
| `output.prometheus-remote-write.batchTimeout` | `--output-prometheus-remote-write-batchtimeout` | `BLITZ_OUTPUT_PROMETHEUS_REMOTE_WRITE_BATCHTIMEOUT` | `5s`    | Maximum wait before flushing a partial batch.        |
| `output.prometheus-remote-write.timeout`      | `--output-prometheus-remote-write-timeout`      | `BLITZ_OUTPUT_PROMETHEUS_REMOTE_WRITE_TIMEOUT`      | `30s`   | Per-request HTTP timeout.                            |
| `output.prometheus-remote-write.headers`      | n/a                                             | n/a                                                 | `{}`    | Extra HTTP headers sent on every request. YAML only. |

## Example Configuration

### Remote-Write 1.0

```yaml
output:
  type: prometheus-remote-write
  prometheus-remote-write:
    endpoint: http://prometheus.example.com:9090/api/v1/write
    version: "1.0"
    batchSize: 500
    batchTimeout: 5s
    timeout: 30s
```

### Remote-Write 2.0 with an authorization header

```yaml
output:
  type: prometheus-remote-write
  prometheus-remote-write:
    endpoint: https://prometheus.example.com/api/v1/write
    version: "2.0"
    headers:
      Authorization: "Bearer <token>"
```

## Self-Telemetry

blitz records these instruments about the output's own operation and exports them through its telemetry pipeline. They are not the metrics the output writes to the remote-write endpoint.

- **`blitz.output.prometheus_remote_write.batch_size`** (Histogram): series per remote-write batch.
- **`blitz.output.prometheus_remote_write.series_sent`** (Counter): series successfully posted.
- **`blitz.output.prometheus_remote_write.posts_failed`** (Counter): remote-write POSTs that failed.
- **`blitz.output.prometheus_remote_write.post_latency`** (Histogram, ms): POST request latency.

The shared output instruments also record entries received and active workers, tagged `output_type=prometheus-remote-write`.
