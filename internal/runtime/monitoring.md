# Runtime Metrics Reference

## Quick Reference

| Metric | Type | Unit | Description |
|--------|------|------|-------------|
| [`blitz.module.startup.duration`](#blitzmodulestartupduration) | Histogram | `ms` | time to start a single module during session startup |
| [`blitz.session.startup.duration`](#blitzsessionstartupduration) | Histogram | `ms` | time to start all modules in the session |
| [`blitz.startup.duration`](#blitzstartupduration) | Histogram | `ms` | time from process start to the service running |

---

## Metrics Detail

### blitz.module.startup.duration

| Property | Value |
|----------|-------|
| **Type** | Histogram |
| **Unit** | `ms` |
| **Meter** | `runtime` |
| **Stability** | Stable |
| **Description** | time to start a single module during session startup |
| **Attributes** |`module_name` |

**Attributes:**

| Name | Type | Required | Values |
|------|------|----------|--------|
| `module_name` | string | ✓ | - |

**Usage:**
```go
// Type-safe wrapper with required attributes
blitzModuleStartupDurationHistogram.Record(ctx, 1, moduleNameValue)
```

---

### blitz.session.startup.duration

| Property | Value |
|----------|-------|
| **Type** | Histogram |
| **Unit** | `ms` |
| **Meter** | `runtime` |
| **Stability** | Stable |
| **Description** | time to start all modules in the session |

**Usage:**
```go
blitzSessionStartupDurationHistogram.Record(ctx, 1)
```

---

### blitz.startup.duration

| Property | Value |
|----------|-------|
| **Type** | Histogram |
| **Unit** | `ms` |
| **Meter** | `runtime` |
| **Stability** | Stable |
| **Description** | time from process start to the service running |

**Usage:**
```go
blitzStartupDurationHistogram.Record(ctx, 1)
```

---



---

**Generated:** `make generate-o11y` | **Registry:** `runtime/monitoring/` | **Templates:** `weaver/templates/`