# Hec Metrics Reference

## Quick Reference

| Metric | Type | Unit | Description |
|--------|------|------|-------------|
| [`blitz.output.hec.ack_confirmed`](#blitzoutputhecack-confirmed) | Counter | `{ack}` | total number of ACKs confirmed by the server |
| [`blitz.output.hec.ack_dropped`](#blitzoutputhecack-dropped) | Counter | `{ack}` | total number of batches dropped after max retries |
| [`blitz.output.hec.ack_expired`](#blitzoutputhecack-expired) | Counter | `{ack}` | total number of ACKs that expired without confirmation |
| [`blitz.output.hec.ack_pending`](#blitzoutputhecack-pending) | Gauge | `{ack}` | number of ACKs currently pending confirmation |
| [`blitz.output.hec.ack_poll_latency`](#blitzoutputhecack-poll-latency) | Histogram | `ms` | latency of ACK polling requests |
| [`blitz.output.hec.ack_retried`](#blitzoutputhecack-retried) | Counter | `{ack}` | total number of batches retried due to ACK failure |
| [`blitz.output.hec.batch_size`](#blitzoutputhecbatch-size) | Histogram | `{entry}` | number of entries per HEC batch |

---

## Metrics Detail

### blitz.output.hec.ack_confirmed

| Property | Value |
|----------|-------|
| **Type** | Counter |
| **Unit** | `{ack}` |
| **Meter** | `hec` |
| **Stability** | Stable |
| **Description** | total number of ACKs confirmed by the server |

**Usage:**
```go
blitzOutputHecAckConfirmedCounter.Add(ctx, 1)
```

---

### blitz.output.hec.ack_dropped

| Property | Value |
|----------|-------|
| **Type** | Counter |
| **Unit** | `{ack}` |
| **Meter** | `hec` |
| **Stability** | Stable |
| **Description** | total number of batches dropped after max retries |

**Usage:**
```go
blitzOutputHecAckDroppedCounter.Add(ctx, 1)
```

---

### blitz.output.hec.ack_expired

| Property | Value |
|----------|-------|
| **Type** | Counter |
| **Unit** | `{ack}` |
| **Meter** | `hec` |
| **Stability** | Stable |
| **Description** | total number of ACKs that expired without confirmation |

**Usage:**
```go
blitzOutputHecAckExpiredCounter.Add(ctx, 1)
```

---

### blitz.output.hec.ack_pending

| Property | Value |
|----------|-------|
| **Type** | Gauge |
| **Unit** | `{ack}` |
| **Meter** | `hec` |
| **Stability** | Stable |
| **Description** | number of ACKs currently pending confirmation |

**Usage:**
```go
blitzOutputHecAckPendingGauge.Add(ctx, 1)
```

---

### blitz.output.hec.ack_poll_latency

| Property | Value |
|----------|-------|
| **Type** | Histogram |
| **Unit** | `ms` |
| **Meter** | `hec` |
| **Stability** | Stable |
| **Description** | latency of ACK polling requests |

**Usage:**
```go
blitzOutputHecAckPollLatencyHistogram.Record(ctx, 1)
```

---

### blitz.output.hec.ack_retried

| Property | Value |
|----------|-------|
| **Type** | Counter |
| **Unit** | `{ack}` |
| **Meter** | `hec` |
| **Stability** | Stable |
| **Description** | total number of batches retried due to ACK failure |

**Usage:**
```go
blitzOutputHecAckRetriedCounter.Add(ctx, 1)
```

---

### blitz.output.hec.batch_size

| Property | Value |
|----------|-------|
| **Type** | Histogram |
| **Unit** | `{entry}` |
| **Meter** | `hec` |
| **Stability** | Stable |
| **Description** | number of entries per HEC batch |

**Usage:**
```go
blitzOutputHecBatchSizeHistogram.Record(ctx, 1)
```

---



---

**Generated:** `make generate-o11y` | **Registry:** `hec/monitoring/` | **Templates:** `weaver/templates/`