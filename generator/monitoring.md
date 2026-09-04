# Generator Metrics Reference

## Quick Reference

| Metric | Type | Unit | Description |
|--------|------|------|-------------|
| [`blitz.generator.active_workers`](#blitzgeneratoractive-workers) | Gauge | `{worker}` | number of active worker goroutines |
| [`blitz.generator.entries`](#blitzgeneratorentries) | Counter | `{entry}` | total number of telemetry entries generated |
| [`blitz.generator.write_errors`](#blitzgeneratorwrite-errors) | Counter | `{error}` | total number of write errors |

---

## Metrics Detail

### blitz.generator.active_workers

| Property | Value |
|----------|-------|
| **Type** | Gauge |
| **Unit** | `{worker}` |
| **Meter** | `generator` |
| **Stability** | Stable |
| **Description** | number of active worker goroutines |
| **Attributes** |`generator_type` |

**Attributes:**

| Name | Type | Required | Values |
|------|------|----------|--------|
| `generator_type` | string | ✓ | - |

**Usage:**
```go
// Type-safe wrapper with required attributes
blitzGeneratorActiveWorkersGauge.Add(ctx, 1, generatorTypeValue)
```

---

### blitz.generator.entries

| Property | Value |
|----------|-------|
| **Type** | Counter |
| **Unit** | `{entry}` |
| **Meter** | `generator` |
| **Stability** | Stable |
| **Description** | total number of telemetry entries generated |
| **Attributes** |`generator_type` |

**Attributes:**

| Name | Type | Required | Values |
|------|------|----------|--------|
| `generator_type` | string | ✓ | - |

**Usage:**
```go
// Type-safe wrapper with required attributes
blitzGeneratorEntriesCounter.Add(ctx, 1, generatorTypeValue)
```

---

### blitz.generator.write_errors

| Property | Value |
|----------|-------|
| **Type** | Counter |
| **Unit** | `{error}` |
| **Meter** | `generator` |
| **Stability** | Stable |
| **Description** | total number of write errors |
| **Attributes** |`generator_type`, `error_type` |

**Attributes:**

| Name | Type | Required | Values |
|------|------|----------|--------|
| `generator_type` | string | ✓ | - |
| `error_type` | enum | ○ | `unknown`, `timeout` |

**Usage:**
```go
// Type-safe wrapper with required attributes
blitzGeneratorWriteErrorsCounter.Add(ctx, 1, generatorTypeValue)
```

---



---

**Generated:** `make generate-o11y` | **Registry:** `generator/monitoring/` | **Templates:** `weaver/templates/`