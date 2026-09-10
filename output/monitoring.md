# Output Metrics Reference

## Quick Reference

| Metric | Type | Unit | Description |
|--------|------|------|-------------|
| [`blitz.output.active_workers`](#blitzoutputactive-workers) | Gauge | `{worker}` | number of active output worker goroutines |
| [`blitz.output.entries_received`](#blitzoutputentries-received) | Counter | `{entry}` | total number of telemetry entries received by the output |
| [`blitz.output.entry_rate`](#blitzoutputentry-rate) | Counter | `{entry}/s` | rate of telemetry entries processed per second |
| [`blitz.output.queue_size`](#blitzoutputqueue-size) | Gauge | `{entry}` | current number of entries in the output queue |
| [`blitz.output.request_latency`](#blitzoutputrequest-latency) | Histogram | `ms` | latency of output requests |
| [`blitz.output.request_size`](#blitzoutputrequest-size) | Histogram | `By` | size of output requests in bytes |
| [`blitz.output.send_errors`](#blitzoutputsend-errors) | Counter | `{error}` | total number of send errors |

---

## Metrics Detail

### blitz.output.active_workers

| Property | Value |
|----------|-------|
| **Type** | Gauge |
| **Unit** | `{worker}` |
| **Meter** | `output` |
| **Stability** | Stable |
| **Description** | number of active output worker goroutines |
| **Attributes** |`output_type` |

**Attributes:**

| Name | Type | Required | Values |
|------|------|----------|--------|
| `output_type` | string | ✓ | - |

**Usage:**
```go
// Type-safe wrapper with required attributes
blitzOutputActiveWorkersGauge.Add(ctx, 1, outputTypeValue)
```

---

### blitz.output.entries_received

| Property | Value |
|----------|-------|
| **Type** | Counter |
| **Unit** | `{entry}` |
| **Meter** | `output` |
| **Stability** | Stable |
| **Description** | total number of telemetry entries received by the output |
| **Attributes** |`output_type`, `telemetry_type` |

**Attributes:**

| Name | Type | Required | Values |
|------|------|----------|--------|
| `output_type` | string | ✓ | - |
| `telemetry_type` | string | ✓ | - |

**Usage:**
```go
// Type-safe wrapper with required attributes
blitzOutputEntriesReceivedCounter.Add(ctx, 1, outputTypeValue, telemetryTypeValue)
```

---

### blitz.output.entry_rate

| Property | Value |
|----------|-------|
| **Type** | Counter |
| **Unit** | `{entry}/s` |
| **Meter** | `output` |
| **Stability** | Stable |
| **Description** | rate of telemetry entries processed per second |
| **Attributes** |`output_type`, `telemetry_type` |

**Attributes:**

| Name | Type | Required | Values |
|------|------|----------|--------|
| `output_type` | string | ✓ | - |
| `telemetry_type` | string | ✓ | - |

**Usage:**
```go
// Type-safe wrapper with required attributes
blitzOutputEntryRateCounter.Add(ctx, 1, outputTypeValue, telemetryTypeValue)
```

---

### blitz.output.queue_size

| Property | Value |
|----------|-------|
| **Type** | Gauge |
| **Unit** | `{entry}` |
| **Meter** | `output` |
| **Stability** | Stable |
| **Description** | current number of entries in the output queue |
| **Attributes** |`output_type` |

**Attributes:**

| Name | Type | Required | Values |
|------|------|----------|--------|
| `output_type` | string | ✓ | - |

---

### blitz.output.request_latency

| Property | Value |
|----------|-------|
| **Type** | Histogram |
| **Unit** | `ms` |
| **Meter** | `output` |
| **Stability** | Stable |
| **Description** | latency of output requests |
| **Attributes** |`output_type`, `telemetry_type` |

**Attributes:**

| Name | Type | Required | Values |
|------|------|----------|--------|
| `output_type` | string | ✓ | - |
| `telemetry_type` | string | ✓ | - |

**Usage:**
```go
// Type-safe wrapper with required attributes
blitzOutputRequestLatencyHistogram.Record(ctx, 1, outputTypeValue, telemetryTypeValue)
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
| **Attributes** |`output_type`, `telemetry_type` |

**Attributes:**

| Name | Type | Required | Values |
|------|------|----------|--------|
| `output_type` | string | ✓ | - |
| `telemetry_type` | string | ✓ | - |

**Usage:**
```go
// Type-safe wrapper with required attributes
blitzOutputRequestSizeHistogram.Record(ctx, 1, outputTypeValue, telemetryTypeValue)
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
| **Attributes** |`output_type`, `telemetry_type` |

**Attributes:**

| Name | Type | Required | Values |
|------|------|----------|--------|
| `output_type` | string | ✓ | - |
| `telemetry_type` | string | ✓ | - |

**Usage:**
```go
// Type-safe wrapper with required attributes
blitzOutputSendErrorsCounter.Add(ctx, 1, outputTypeValue, telemetryTypeValue)
```

---



---

**Generated:** `make generate-o11y` | **Registry:** `output/monitoring/` | **Templates:** `weaver/templates/`