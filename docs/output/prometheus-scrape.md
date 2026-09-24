# Prometheus Scrape Output

The prometheus-scrape output is a metrics-only pull endpoint. It hosts an HTTP `/metrics` endpoint in Prometheus text exposition format, so a Prometheus server or the collector `prometheusreceiver` can scrape blitz directly. Unlike the push-based [remote-write output](prometheus-remote-write.md), the scraper controls timing: blitz holds the current value of each series and serves a snapshot on every scrape.

## Metric Mapping

Generated metric points map to Prometheus series before encoding:

- Gauge and non-monotonic Sum become a gauge.
- Counter becomes a counter with a `_total` suffix.
- Histogram becomes cumulative `_bucket` series with `le` labels, including `+Inf`, plus `_sum` and `_count`.

Metric and label names are sanitized to the Prometheus grammar. Each series carries its name plus the metric's attributes as labels. The endpoint keeps the latest value per series (name, type, and label set), so repeated writes to the same series overwrite rather than accumulate — a faithful exporter model.

By default no explicit timestamp is written, and the scraper stamps each sample at scrape time (the idiomatic exporter behavior). Set `emitTimestamps: true` to append each sample's millisecond timestamp instead.

## Configuration

| YAML Path                                  | Flag                                          | Environment Variable                            | Default          | Description                                        |
|--------------------------------------------|-----------------------------------------------|-------------------------------------------------|------------------|----------------------------------------------------|
| `output.type`                              | `--output-type`                               | `BLITZ_OUTPUT_TYPE`                             | `nop`            | Set to `prometheus-scrape` to use this output.     |
| `output.prometheus-scrape.listenAddress`   | `--output-prometheus-scrape-listenaddress`    | `BLITZ_OUTPUT_PROMETHEUS_SCRAPE_LISTENADDRESS`  | `0.0.0.0:9464`   | Host:port the metrics endpoint binds to.           |
| `output.prometheus-scrape.metricsPath`     | `--output-prometheus-scrape-metricspath`      | `BLITZ_OUTPUT_PROMETHEUS_SCRAPE_METRICSPATH`    | `/metrics`       | URL path the exposition is served on.              |
| `output.prometheus-scrape.emitTimestamps`  | `--output-prometheus-scrape-emittimestamps`   | `BLITZ_OUTPUT_PROMETHEUS_SCRAPE_EMITTIMESTAMPS` | `false`          | Append per-sample millisecond timestamps.          |

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
- **`blitz.output.prometheus_scrape.series_exposed`** (Gauge): series exposed at the last scrape.
- **`blitz.output.prometheus_scrape.exposition_bytes`** (Histogram): exposition body size per scrape.
- **`blitz.output.prometheus_scrape.scrape_latency`** (Histogram, ms): latency of serving a scrape request.

The shared output instruments also record entries received and active workers, tagged `output_type=prometheus-scrape`.
