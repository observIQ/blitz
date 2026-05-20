# Embedding blitz as a library

Blitz can run as a library inside a host process that wants to consume the telemetry blitz produces, without giving up the standalone CLI. The `embed` package exposes a small contract any host can implement.

The first concrete consumer is the [bindplane-otel-contrib telemetrygeneratorreceiver](https://github.com/observIQ/bindplane-otel-contrib/tree/main/receiver/telemetrygeneratorreceiver), but the contract is generic — designed to serve any host equally well.

## Module classification — Producer vs Effector

Every blitz module falls into one of two disjoint classes based on where its effect lands:

- **Producer** — yields structured telemetry records the host can consume in-process. Embed-eligible.
- **Effector** — side-effects outside blitz's process (operating-system event log, listening sockets that external clients poll). Not embed-eligible: the host can't observe these effects.

The split is a fact about the module, not a config knob. Classes are declared at compile time:

```go
type Module interface {
    Name() string
    Start(ctx context.Context) error
    Stop(ctx context.Context) error
}

type ProducerModule interface {
    Module
    isProducer()   // unexported marker
}

type EffectorModule interface {
    Module
    isEffector()
}
```

A module declares its class by embedding `embed.ProducerMarker` or `embed.EffectorMarker` in its struct. `embed.Config.Modules` is typed `[]ProducerModule`, so passing an Effector is a compile error — the type system enforces the embed contract.

## Records and consumers

Blitz emits three signal types, defined in `embed`:

- `LogRecord` — a single log entry (message, optional parser callback, metadata)
- `MetricPoint` — a single metric data point (gauge / sum / counter / histogram)
- `Span` — a single trace span

Records are blitz-owned, wire-format-agnostic. OTLP encoding only happens at the OTLP-output boundary or in an OTel-pdata adapter; the embed contract itself does not pull `pdata`.

A host implements one or more consumer interfaces:

```go
type LogConsumer    interface { ConsumeLogs(ctx, []LogRecord) error }
type MetricConsumer interface { ConsumeMetrics(ctx, []MetricPoint) error }
type TraceConsumer  interface { ConsumeTraces(ctx, []Span) error }
```

Consumers receive batches. Producer modules push size-1 batches today; the framework allows larger batches if a module wants to coalesce.

## Constructing an embedded runner

```go
import (
    "context"

    "github.com/observiq/blitz/embed"
    "github.com/observiq/blitz/generator/apache"
)

func main() {
    logger := zap.NewNop()

    // 1. Host owns the consumers and ambient resources.
    host := embed.Host{
        Logs:    myLogConsumer,
        Logger:  logger,
        // Metrics / Traces / Resource also available.
    }

    // 2. Construct modules, passing the appropriate consumer from host.
    apacheGen, _ := apache.New(logger, /*workers*/ 1, /*rate*/ time.Second, host.Logs)

    // 3. Build the runner.
    runner, err := embed.New(embed.Config{
        Modules: []embed.ProducerModule{apacheGen},
    })
    if err != nil { /* ... */ }

    // 4. Start, run, stop.
    ctx := context.Background()
    if err := runner.Start(ctx, host); err != nil { /* ... */ }

    // ... host process runs, records flow through host.Logs.ConsumeLogs ...

    runner.Stop(ctx)
}
```

## Lifecycle

- **Start** launches every configured module. If any module's Start fails, modules already started are rolled back (Stop'd in reverse order) before Start returns the failure.
- **Stop** terminates modules in reverse start order. Stop continues on errors and returns the first error encountered.

`Start` may only be called once per runner; double-Start returns an error. Construct a fresh runner for a fresh lifecycle.

## Backpressure

Each consumer channel (Logs / Metrics / Traces) selects a backpressure mode at runner construction time:

- `BackpressureBlock` (default) — producer blocks until consumer accepts the batch.
- `BackpressureDrop` — drop the batch when the consumer is not ready. Dropped batches are counted in a `records_dropped` metric.
- `BackpressureBuffer{Size}` — queue in memory up to `Size`. When the buffer is full, behavior reverts to Block.

## Error semantics

Consumer errors are best-effort: blitz logs the error, increments a `consumer_errors` counter, and continues producing. A consumer that wants stricter semantics can return errors and observe them on the metric.

## Resource attributes

`Host.Resource` holds the per-session base resource (key-value map). Modules may attach module-specific attributes that merge on top of the base at emit time.

## Configuration

Embedded mode is programmatic-Go-only. YAML configuration is CLI-only. `embed.Config` is a Go struct populated by the host directly — no YAML loader is exposed for embed mode.

## Consuming via OTel pdata

For OTel hosts that want `pdata` rather than blitz records, the adapter lives in the receiver itself (e.g., the [telemetrygeneratorreceiver](https://github.com/observIQ/bindplane-otel-contrib/tree/main/receiver/telemetrygeneratorreceiver)). Implementing `LogConsumer` / `MetricConsumer` / `TraceConsumer` and converting to `plog.Logs` / `pmetric.Metrics` / `ptrace.Traces` inside `Consume*` is the recommended pattern. Keeping the converter on the receiver side avoids pulling the heavy `go.opentelemetry.io/collector/pdata` module into blitz core.

## Module classes

| Module                                            | Class    | Notes                                                   |
|---------------------------------------------------|----------|---------------------------------------------------------|
| apache, apache_combined, apache_error             | Producer | Common / Combined / Error log formats                   |
| filegen                                           | Producer | Replays lines from files; supports glob and directories |
| json                                              | Producer | Structured JSON logs; `default` and `pii` log types     |
| kubernetes                                        | Producer | CRI-O container log format                              |
| nginx                                             | Producer | NGINX Combined log format                               |
| nop                                               | Producer | No-op generator (testing helper)                        |
| okta                                              | Producer | Okta System Log format                                  |
| paloalto                                          | Producer | Palo Alto syslog                                        |
| postgres                                          | Producer | PostgreSQL log format                                   |
| hostmetrics                                       | Producer | Host metric scrapers (CPU, disk, memory, etc.)          |
| traces                                            | Producer | Synthetic distributed traces                            |
| winevt                                            | Producer | Windows Event XML mode (current). A future WEL Windows-API mode lands as an Effector via a separate constructor. |
| Future: WEL Windows-API mode (PIPE-928)           | Effector | Writes to actual Windows event log. Cannot be embedded.  |
| Future: REST simulators (PIPE-943)                | Effector | HTTP servers external clients poll. Cannot be embedded.  |
