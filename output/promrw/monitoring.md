# Promrw Metrics Reference

## Quick Reference

| Metric | Type | Unit | Description |
|--------|------|------|-------------|
| [`blitz.output.prometheus_remote_write.batch_size`](#blitzoutputprometheus-remote-writebatch-size) | Histogram | `{series}` | number of series per remote-write batch |
| [`blitz.output.prometheus_remote_write.post_latency`](#blitzoutputprometheus-remote-writepost-latency) | Histogram | `ms` | latency of remote-write POST requests |
| [`blitz.output.prometheus_remote_write.posts_failed`](#blitzoutputprometheus-remote-writeposts-failed) | Counter | `{post}` | total number of remote-write POSTs that failed |
| [`blitz.output.prometheus_remote_write.series_sent`](#blitzoutputprometheus-remote-writeseries-sent) | Counter | `{series}` | total number of series successfully posted |

---

## Metrics Detail

### blitz.output.prometheus_remote_write.batch_size

| Property | Value |
|----------|-------|
| **Type** | Histogram |
| **Unit** | `{series}` |
| **Meter** | `promrw` |
| **Stability** | Stable |
| **Description** | number of series per remote-write batch |

**Usage:**
```go
blitzOutputPrometheusRemoteWriteBatchSizeHistogram.Record(ctx, 1)
```

---

### blitz.output.prometheus_remote_write.post_latency

| Property | Value |
|----------|-------|
| **Type** | Histogram |
| **Unit** | `ms` |
| **Meter** | `promrw` |
| **Stability** | Stable |
| **Description** | latency of remote-write POST requests |

**Usage:**
```go
blitzOutputPrometheusRemoteWritePostLatencyHistogram.Record(ctx, 1)
```

---

### blitz.output.prometheus_remote_write.posts_failed

| Property | Value |
|----------|-------|
| **Type** | Counter |
| **Unit** | `{post}` |
| **Meter** | `promrw` |
| **Stability** | Stable |
| **Description** | total number of remote-write POSTs that failed |

**Usage:**
```go
blitzOutputPrometheusRemoteWritePostsFailedCounter.Add(ctx, 1)
```

---

### blitz.output.prometheus_remote_write.series_sent

| Property | Value |
|----------|-------|
| **Type** | Counter |
| **Unit** | `{series}` |
| **Meter** | `promrw` |
| **Stability** | Stable |
| **Description** | total number of series successfully posted |

**Usage:**
```go
blitzOutputPrometheusRemoteWriteSeriesSentCounter.Add(ctx, 1)
```

---



---

**Generated:** `make generate-o11y` | **Registry:** `promrw/monitoring/` | **Templates:** `weaver/templates/`