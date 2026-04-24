# Output Metrics Reference

## Quick Reference

| Metric | Type | Unit | Description |
|--------|------|------|-------------|
| [`blitz.output.entries_received`](#blitzoutputentries-received) | Counter | `{entry}` | total number of log entries received by the output |
| [`blitz.output.active_workers`](#blitzoutputactive-workers) | Gauge | `{worker}` | number of active output worker goroutines |
| [`blitz.output.entry_rate`](#blitzoutputentry-rate) | Counter | `{entry}/s` | rate of log entries processed per second |
| [`blitz.output.request_size`](#blitzoutputrequest-size) | Histogram | `By` | size of output requests in bytes |
| [`blitz.output.request_latency`](#blitzoutputrequest-latency) | Histogram | `s` | latency of output requests |
| [`blitz.output.send_errors`](#blitzoutputsend-errors) | Counter | `{error}` | total number of send errors |
| [`blitz.output.queue_size`](#blitzoutputqueue-size) | Gauge | `{entry}` | current number of entries in the output queue |

---

## Metrics Detail

### blitz.output.entries_received

| Property | Value |
|----------|-------|
| **Type** | Counter |
| **Unit** | `{entry}` |
| **Meter** | `output` |
| **Stability** | Stable |
| **Description** | total number of log entries received by the output |
| **Attributes** | `output_type` |

**Attributes:**

| Name | Type | Required | Values |
|------|------|----------|--------|
| `output_type` | string | ✓ | - |

**Usage:**
```go
output.BlitzOutputEntriesReceivedCounter.Add(ctx, 1, outputTypeValue)
```

---

### blitz.output.active_workers

| Property | Value |
|----------|-------|
| **Type** | Gauge |
| **Unit** | `{worker}` |
| **Meter** | `output` |
| **Stability** | Stable |
| **Description** | number of active output worker goroutines |
| **Attributes** | `output_type` |

**Attributes:**

| Name | Type | Required | Values |
|------|------|----------|--------|
| `output_type` | string | ✓ | - |

**Usage:**
```go
output.BlitzOutputActiveWorkersGauge.Record(ctx, 1, outputTypeValue)
```

---

### blitz.output.entry_rate

| Property | Value |
|----------|-------|
| **Type** | Float64 Counter |
| **Unit** | `{entry}/s` |
| **Meter** | `output` |
| **Stability** | Stable |
| **Description** | rate of log entries processed per second |
| **Attributes** | `output_type` |

**Attributes:**

| Name | Type | Required | Values |
|------|------|----------|--------|
| `output_type` | string | ✓ | - |

**Usage:**
```go
output.BlitzOutputEntryRateCounter.Add(ctx, 1.0, outputTypeValue)
```

---

### blitz.output.request_size

| Property | Value |
|----------|-------|
| **Type** | Histogram |
| **Unit** | `By` |
| **Meter** | `output` |
| **Stability** | Stable |
| **Description** | size of output requests in bytes |
| **Attributes** | `output_type` |

**Attributes:**

| Name | Type | Required | Values |
|------|------|----------|--------|
| `output_type` | string | ✓ | - |

**Usage:**
```go
output.BlitzOutputRequestSizeHistogram.Record(ctx, 1, outputTypeValue)
```

---

### blitz.output.request_latency

| Property | Value |
|----------|-------|
| **Type** | Float64 Histogram |
| **Unit** | `s` |
| **Meter** | `output` |
| **Stability** | Stable |
| **Description** | latency of output requests |
| **Attributes** | `output_type` |

**Attributes:**

| Name | Type | Required | Values |
|------|------|----------|--------|
| `output_type` | string | ✓ | - |

**Usage:**
```go
output.BlitzOutputRequestLatencyHistogram.Record(ctx, 0.5, outputTypeValue)
```

---

### blitz.output.send_errors

| Property | Value |
|----------|-------|
| **Type** | Counter |
| **Unit** | `{error}` |
| **Meter** | `output` |
| **Stability** | Stable |
| **Description** | total number of send errors |
| **Attributes** | `output_type` |

**Attributes:**

| Name | Type | Required | Values |
|------|------|----------|--------|
| `output_type` | string | ✓ | - |

**Usage:**
```go
output.BlitzOutputSendErrorsCounter.Add(ctx, 1, outputTypeValue)
```

---

### blitz.output.queue_size

| Property | Value |
|----------|-------|
| **Type** | Observable Gauge |
| **Unit** | `{entry}` |
| **Meter** | `output` |
| **Stability** | Stable |
| **Description** | current number of entries in the output queue |
| **Attributes** | `output_type` |

**Attributes:**

| Name | Type | Required | Values |
|------|------|----------|--------|
| `output_type` | string | ✓ | - |

---


---

**Generated:** `make generate-o11y` | **Registry:** `output/monitoring/` | **Templates:** `weaver/templates/`
