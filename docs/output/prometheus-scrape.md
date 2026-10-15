# Prometheus Scrape Output

The prometheus-scrape output is a metrics-only pull endpoint. It hosts an HTTP `/metrics` endpoint in the Prometheus exposition format, so a Prometheus server or the collector `prometheusreceiver` can scrape blitz directly. Unlike the push-based [remote-write output](prometheus-remote-write.md), the scraper controls timing: blitz holds the current value of each series and serves a snapshot on every scrape.

## Metric Mapping

The endpoint serves what the OpenTelemetry Collector's `prometheusexporter` (v0.161.0, default settings) would serve for the same points, built with the same Prometheus client library:

- Gauge and non-monotonic Sum become a gauge. Counter becomes a counter. Histogram becomes cumulative `_bucket` series with `le` labels, including `+Inf` from the point's count, plus `_sum` and `_count`. A histogram needs one more bucket count than bounds; `WriteMetric` rejects any other point.
- Names follow the exporter's default naming with suffixes: the unit becomes a suffix (`s` to `_seconds`, `By` to `_bytes`), counters end in `_total`, and gauges with unit `1` end in `_ratio`, so `system.memory.usage` (unit `By`) becomes `system_memory_usage_bytes`.
- Attribute names are sanitized to the Prometheus grammar, and empty values are kept. A point with two attributes that sanitize to one label name is not served, and the conversion error is logged.
- Every series carries `otel_scope_name`, `otel_scope_version`, and `otel_scope_schema_url`, empty because blitz sends an empty instrumentation scope.
- Every metric has a `# HELP` line, empty when the point has no description. The first description and type seen for a name are kept; a later point of another type under the same name is not served.

The endpoint keeps the latest point per series (name, type, attributes, and resource). A point older than the stored one is ignored. A series not updated within `metricExpiration` is dropped, so a host that stops reporting stops being exposed.

`job` comes from `service.namespace`/`service.name` and `instance` from `service.instance.id`, as in the exporter. When `service.name` is absent, `job` falls back to `telemetry.source`, and when `service.instance.id` is absent, `instance` falls back to `host.name`, which blitz generators set instead; this is the one difference from the exporter, and it keeps each simulated host's series distinct. As in the exporter, a present but empty `service.name` or `service.instance.id` gives an empty label with no fallback. A label is left off when its source is missing. Each target with both a `job` and an `instance` gets one `target_info` series (`# HELP target_info Target metadata`) carrying its other resource attributes; keys that sanitize to one label are joined with `;`. As in the exporter, a resource key that is no valid label name drops every `target_info` from that scrape.

By default no explicit timestamp is written, and the scraper stamps each sample at scrape time. Set `emitTimestamps: true` to append each point's millisecond timestamp, as the exporter's `send_timestamps` does. The response format and compression follow the scraper's `Accept` and `Accept-Encoding` headers.

## Configuration

| YAML Path                                   | Flag                                          | Environment Variable                              | Default        | Description                                                               |
|---------------------------------------------|-----------------------------------------------|---------------------------------------------------|----------------|---------------------------------------------------------------------------|
| `output.type`                               | `--output-type`                               | `BLITZ_OUTPUT_TYPE`                               | `nop`          | Set to `prometheus-scrape` to use this output.                            |
| `output.prometheus-scrape.listenAddress`    | `--output-prometheus-scrape-listenaddress`    | `BLITZ_OUTPUT_PROMETHEUS_SCRAPE_LISTENADDRESS`    | `0.0.0.0:9464` | Host:port the metrics endpoint binds to.                                  |
| `output.prometheus-scrape.metricsPath`      | `--output-prometheus-scrape-metricspath`      | `BLITZ_OUTPUT_PROMETHEUS_SCRAPE_METRICSPATH`      | `/metrics`     | URL path the exposition is served on.                                     |
| `output.prometheus-scrape.emitTimestamps`   | `--output-prometheus-scrape-emittimestamps`   | `BLITZ_OUTPUT_PROMETHEUS_SCRAPE_EMITTIMESTAMPS`   | `false`        | Append per-sample millisecond timestamps.                                 |
| `output.prometheus-scrape.metricExpiration` | `--output-prometheus-scrape-metricexpiration` | `BLITZ_OUTPUT_PROMETHEUS_SCRAPE_METRICEXPIRATION` | `5m`           | Drop a series not updated within this duration. `0` keeps series forever. |

The endpoint has no authentication or TLS. By default it listens on every
interface, since Prometheus usually scrapes from another host. Set
`listenAddress` to a loopback address such as `127.0.0.1:9464` to keep it
local.

## Example Configuration

```yaml
output:
  type: prometheus-scrape
  prometheus-scrape:
    listenAddress: 0.0.0.0:9464
    metricsPath: /metrics
```

Point a scraper at it, for example a Prometheus scrape config:

```yaml
scrape_configs:
  - job_name: blitz
    static_configs:
      - targets: ["blitz-host:9464"]
```

## Self-Telemetry

blitz records these instruments about the output's own operation and exports them through its telemetry pipeline. They are not the metrics the output exposes to scrapers.

- **`blitz.output.prometheus_scrape.scrapes`** (Counter): scrape requests served.
- **`blitz.output.prometheus_scrape.series_exposed`** (Gauge): series exposed at the last scrape, each histogram bucket, `_sum`, and `_count` counted (not counting `target_info`).
- **`blitz.output.prometheus_scrape.exposition_bytes`** (Histogram): bytes sent per scrape, compressed when the scraper accepts gzip.
- **`blitz.output.prometheus_scrape.scrape_latency`** (Histogram, ms): latency of serving a scrape request.

The shared `blitz.output.entries_received` counter also records each point the output receives, tagged `output_type=prometheus-scrape`.
