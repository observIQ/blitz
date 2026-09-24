# Promscrape Metrics Reference

## Quick Reference

| Metric | Type | Unit | Description |
|--------|------|------|-------------|
| [`blitz.output.prometheus_scrape.exposition_bytes`](#blitzoutputprometheus-scrapeexposition-bytes) | Histogram | `By` | size of the exposition body per scrape |
| [`blitz.output.prometheus_scrape.scrape_latency`](#blitzoutputprometheus-scrapescrape-latency) | Histogram | `ms` | latency of serving a scrape request |
| [`blitz.output.prometheus_scrape.scrapes`](#blitzoutputprometheus-scrapescrapes) | Counter | `{scrape}` | total number of scrape requests served |
| [`blitz.output.prometheus_scrape.series_exposed`](#blitzoutputprometheus-scrapeseries-exposed) | Gauge | `{series}` | number of series exposed at the last scrape |

---

## Metrics Detail

### blitz.output.prometheus_scrape.exposition_bytes

| Property | Value |
|----------|-------|
| **Type** | Histogram |
| **Unit** | `By` |
| **Meter** | `promscrape` |
| **Stability** | Stable |
| **Description** | size of the exposition body per scrape |

**Usage:**
```go
blitzOutputPrometheusScrapeExpositionBytesHistogram.Record(ctx, 1)
```

---

### blitz.output.prometheus_scrape.scrape_latency

| Property | Value |
|----------|-------|
| **Type** | Histogram |
| **Unit** | `ms` |
| **Meter** | `promscrape` |
| **Stability** | Stable |
| **Description** | latency of serving a scrape request |

**Usage:**
```go
blitzOutputPrometheusScrapeScrapeLatencyHistogram.Record(ctx, 1)
```

---

### blitz.output.prometheus_scrape.scrapes

| Property | Value |
|----------|-------|
| **Type** | Counter |
| **Unit** | `{scrape}` |
| **Meter** | `promscrape` |
| **Stability** | Stable |
| **Description** | total number of scrape requests served |

**Usage:**
```go
blitzOutputPrometheusScrapeScrapesCounter.Add(ctx, 1)
```

---

### blitz.output.prometheus_scrape.series_exposed

| Property | Value |
|----------|-------|
| **Type** | Gauge |
| **Unit** | `{series}` |
| **Meter** | `promscrape` |
| **Stability** | Stable |
| **Description** | number of series exposed at the last scrape |

**Usage:**
```go
blitzOutputPrometheusScrapeSeriesExposedGauge.Add(ctx, 1)
```

---



---

**Generated:** `make generate-o11y` | **Registry:** `promscrape/monitoring/` | **Templates:** `weaver/templates/`