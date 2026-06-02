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

### Generators by signal type

| Signal  | Generators wired today                                                                   | Consumer field on `embed.Host` |
|---------|-------------------------------------------------------------------------------------------|--------------------------------|
| Logs    | `apache`, `apache_combined`, `apache_error`, `filegen`, `fix`, `json`, `kubernetes`, `nginx`, `okta`, `paloalto`, `postgres`, `wel` | `Host.Logs`                    |
| Metrics | `hostmetrics`                                                                             | `Host.Metrics`                 |
| Traces  | `traces` *(not yet wired through embed — see PIPE-1024)*                                  | `Host.Traces`                  |

Embedded hosts that load configuration via `config.LoadModules` must populate the relevant `LogConsumer` / `MetricConsumer` / `TraceConsumer` field on `EmbedOpts` for whichever signal types their generators yield; missing consumers surface as a clear construction-time error rather than a runtime no-op.

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
        Metrics: myMetricConsumer, // optional — required when a metric generator is wired
        Logger:  logger,
        // Traces / Resource also available.
    }

    // 2. Construct modules, passing the appropriate consumer from host.
    apacheGen, _ := apache.New(logger, /*workers*/ 1, /*rate*/ time.Second, host.Logs)
    hmGen, _ := hostmetrics.New(hostmetrics.Config{
        Logger:   logger,
        Workers:  1,
        Rate:     10 * time.Second,
        OS:       "linux",
        Consumer: host.Metrics,
    })

    // 3. Build the runner.
    runner, err := embed.New(embed.Config{
        Modules: []embed.ProducerModule{apacheGen, hmGen},
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
- `BackpressureBuffer` — queue in memory up to a configured size. When the buffer is full, behavior reverts to Block.

Mode and buffer size are set via `embed.ConsumerBackpressure` on each signal channel of `embed.Config`:

```go
embed.Config{
    Modules: []embed.ProducerModule{...},
    Logs:    embed.ConsumerBackpressure{Mode: embed.BackpressureBuffer, BufferSize: 1024},
    Metrics: embed.ConsumerBackpressure{Mode: embed.BackpressureDrop},
    Traces:  embed.ConsumerBackpressure{}, // zero value = Block
}
```

## Error semantics

Consumer errors are best-effort: blitz logs the error, increments a `consumer_errors` counter, and continues producing. A consumer that wants stricter semantics can return errors and observe them on the metric.

## Resource attributes

Every blitz record carries a per-record `Metadata.Resource` map describing the entity that emitted it (host, module, format, version). The three signal types follow a parallel shape:

- `LogRecord.Metadata.Resource` — `map[string]string`
- `MetricPoint.Metadata.Resource` — `map[string]string`
- `Span.Metadata.Resource` — `map[string]string`

Similarly for `Metadata.Attributes`:

- `LogRecord.Metadata.Attributes` — `map[string]any` (logs and spans allow richly-typed values)
- `MetricPoint.Metadata.Attributes` — `map[string]string` (OTel metric-attribute convention is string-typed)
- `Span.Metadata.Attributes` — `map[string]any`

### Host base vs per-record Resource — merge semantics

Two layers of Resource are in play at consumption time:

1. **`embed.Host.Resource`** — host-wide ambient values that no individual module should hardcode (deployment ID, environment name, cluster identifier, etc.). The host populates this once at runner construction.
2. **`Metadata.Resource`** — per-record values the generator knows internally at emit time (the hostname the log line semantically describes, the module identifier, format flavor, protocol version, etc.).

Recommended merge: `embed.Host.Resource` is the base; per-record `Metadata.Resource` entries take precedence on key conflict. Hosts that want different semantics can override.

### Resource conventions populated by built-in Producers

Every shipped Producer populates at least:

- `host.name` — the hostname the record semantically describes (defaults to `os.Hostname()`, falls back to `blitz`).
- `telemetry.source` — the module identifier (`apache`, `nginx`, `paloalto`, `fix`, `wel`, …).

Some Producers populate additional dimensions:

| Source                | Extra Resource keys                                                   |
|-----------------------|-----------------------------------------------------------------------|
| `apache` (all 3)      | `apache.format` — `common` / `combined` / `error`                     |
| `json`                | `json.type` — `default` / `pii`                                       |
| `kubernetes`          | `kubernetes.format` — currently `cri-o` (only supported format today) |
| `filegen`             | `filegen.source` — the file / package / glob the line came from       |
| `wel`                 | `wel.channel`, `wel.computer`, `wel.domain`, `wel.role`               |
| `fix` (when it lands) | `fix.version` — `FIX.4.2` / `FIX.4.4` / `FIX.5.0SP2`                  |

Generators MUST NOT carry secret or per-deployment-specific values they don't already legitimately know — that remains the host's concern via `embed.Host.Resource`.

### Building a Resource for a new Producer

Use the helper in `github.com/observiq/blitz/generator/resource`:

```go
import "github.com/observiq/blitz/generator/resource"

// In your emit path:
Metadata: embed.LogRecordMetadata{
    Severity: "INFO",
    Resource: resource.Default("my-module",
        "my-module.format", formatName,
        "my-module.version", versionString,
    ),
},
```

`resource.Default(source, extras...)` returns a fresh map per call (so consumers can mutate safely without affecting subsequent emissions) and memoizes `os.Hostname()` once per process.

## Configuration

Two supported paths:

- **Programmatic Go** — host populates `embed.Config` directly, constructs each generator with the appropriate `embed.LogConsumer`, passes the slice to `embed.New`. This is the lowest-level entry point and is shown in the "Constructing an embedded runner" example above.
- **Blitz YAML via the public `config` package** — for hosts that want their users to drop a blitz YAML config (the same shape the standalone CLI accepts) and have blitz construct the modules. Import `github.com/observiq/blitz/config`:

  ```go
  import (
      blitzconfig "github.com/observiq/blitz/config"
      "github.com/observiq/blitz/embed"
  )

  // Parse + construct in one call.
  modules, err := blitzconfig.LoadModules(yamlBytes, blitzconfig.EmbedOpts{
      Logger:      logger,
      LogConsumer: myLogConsumer,
      // FileGenLibrary: embeddedlibrary.FS(), // optional; nil = ./data_library/ on disk
  })
  if err != nil { /* ... */ }

  runner, err := embed.New(embed.Config{Modules: modules})
  ```

  Or use the lower-level `blitzconfig.Load(yamlBytes, blitzconfig.LoadOpts{})` if the host wants the parsed `*Config` to do its own module construction (e.g., a host that only wants to expose a subset of generators).

### Environment overlay (host-driven)

The embedded loader does **not** read `os.Environ()` directly. The standalone CLI binds `BLITZ_*` env vars automatically via viper; embedded mode requires the host to do its own env-variable scanning (so the host's prefix conventions, secret resolution, and env-loading order aren't bypassed) and pass the resolved values to blitz as a YAML-path → value map.

```go
// Host scans its process env for BLITZ_*-prefixed vars and translates each
// to its YAML-dotted-path form. For example BLITZ_GENERATOR_APACHE-COMMON_WORKERS=4
// becomes the entry "generator.apache-common.workers": "4". The host applies
// whatever prefix/normalization conventions its framework expects.
envOverrides := host.CollectEnvOverridesForBlitz(os.Environ())

modules, err := blitzconfig.LoadModules(yamlBytes, blitzconfig.EmbedOpts{
    Logger:       logger,
    LogConsumer:  myLogConsumer,
    EnvOverrides: envOverrides,
})
```

`EnvOverrides` overlays on top of the parsed YAML, matching the precedence the CLI gives `BLITZ_*` env vars over YAML values.

CLI cobra-flag bindings (`--generator-type`, `--output-otlpgrpc-host`, etc.) are **out of scope for embedded mode** and are NOT honored via this map. Embedded hosts have no flags to bind — they're a library being called by another program. Hosts that want flag-like overrides translate them into YAML paths themselves.

### Distributing the `data_library/` files for embedding

The `filegen` generator references files under `data_library/` for the `package:`-prefixed source syntax and bare-name library fallback. The canonical location is `generator/filegen/embeddedlibrary/data_library/` — there is exactly one copy in the repo. Embedding hosts have two options for getting the files to the running process:

- **Disk** — leave `FileGenLibrary` nil in `EmbedOpts`. The generator probes a fixed sequence of disk locations, in order: `$BLITZ_DATA_LIBRARY_DIR` (host-supplied override), `./data_library/` (cwd-relative, where release tarballs unpack), `./generator/filegen/embeddedlibrary/data_library/` (in-repo canonical for `./blitz` from a fresh clone), `/usr/share/blitz/data_library/` (the nfpm `deb`/`rpm` install path). First match wins; the env override is the recommended path for containers and custom install prefixes.
- **Embedded snapshot** — import `github.com/observiq/blitz/generator/filegen/embeddedlibrary` and pass `embeddedlibrary.FS()` as `FileGenLibrary`. The blitz module ships the `data_library/` files inside the embeddedlibrary package, so `go get github.com/observiq/blitz/generator/filegen/embeddedlibrary` fetches them as part of the module cache — no separate file staging by the host. The package is gated by the `//go:build embed_library` tag so default builds skip the embed; tagged builds get the files baked into the binary.

`LoadModules` only constructs Producer-class **log** generators in this release. Generator types that should logically be in the embed contract but aren't yet return an explicit error rather than being silently dropped, so the host's parsed config can't lose generators the user expected.

### Not yet in the embed contract (slated for v0.17.0)

The following generators are valid in the standalone CLI but are intentionally rejected by `LoadModules` today. The rejections are documented as roadmap, not as long-term design:

- **`hostmetrics`** — produces metrics, not logs. The embed contract has `MetricConsumer` for this and the generator class fits naturally as a Producer wired to it, but `hostmetrics.New` has not been migrated to take a `MetricConsumer` at construction time. Migration is planned alongside the v0.17.0 release.
- **`traces`** — produces spans, not logs. Same story: `TraceConsumer` exists in the contract; `tracesgen.New` has not been migrated. Planned for v0.17.0.
- **`winevt`** — the legacy single-template Windows Event XML generator. Replaced by the new multi-channel `wel` generator landing in the PIPE-928 stack (the WEL stack is being merged ahead of v0.17.0). `winevt` will be deprecated when `wel` lands rather than being migrated to embed.

The v0.17.0 release sequence is: (1) land the WEL stack; (2) land a follow-up stack that migrates `hostmetrics` and `traces` to the embed contract; (3) deprecate `winevt`; (4) cut v0.17.0. After v0.17.0, `LoadModules` will return a `[]embed.ProducerModule` that contains log, metric, AND trace producers, each wired to the appropriate consumer the host supplied.

### Not embed-eligible at all (by design)

- **`nop`** — does nothing; never produces records. Excluded from `LoadModules` because there's no Consumer for it to push to and an embedded host has no reason to instantiate a generator that yields nothing.
- **WEL Windows-API mode (PIPE-928, future)** — writes to the actual Windows Event Log via the OS API. Effector-class. The whole point of Effectors is that their side effects land outside blitz's process, so an embedded host can't observe them. Excluded by design.
- **REST simulators (PIPE-943, future)** — HTTP servers external clients poll. Effector-class for the same reason.

## Consuming via OTel pdata

For OTel hosts that want `pdata` rather than blitz records, the adapter lives in the receiver itself (e.g., the [telemetrygeneratorreceiver](https://github.com/observIQ/bindplane-otel-contrib/tree/main/receiver/telemetrygeneratorreceiver)). Implementing `LogConsumer` / `MetricConsumer` / `TraceConsumer` and converting to `plog.Logs` / `pmetric.Metrics` / `ptrace.Traces` inside `Consume*` is the recommended pattern. Keeping the converter on the receiver side avoids pulling the heavy `go.opentelemetry.io/collector/pdata` module into blitz core.

## Module classes

| Module                                            | Class    | Notes                                                   |
|---------------------------------------------------|----------|---------------------------------------------------------|
| apache-common, apache-combined, apache-error      | Producer | Common / Combined / Error log formats                   |
| filegen                                           | Producer | Replays lines from files; supports glob and directories |
| json                                              | Producer | Structured JSON logs; `default` and `pii` log types     |
| kubernetes                                        | Producer | CRI-O container log format                              |
| nginx                                             | Producer | NGINX Combined log format                               |
| nop                                               | Producer | No-op generator (testing helper)                        |
| okta                                              | Producer | Okta System Log format                                  |
| palo-alto                                         | Producer | Palo Alto syslog                                        |
| postgres                                          | Producer | PostgreSQL log format                                   |
| hostmetrics                                       | Producer | Host metric scrapers (CPU, disk, memory, etc.)          |
| traces                                            | Producer | Synthetic distributed traces                            |
| winevt                                            | Producer | Windows Event XML mode (current). A future WEL Windows-API mode lands as an Effector via a separate constructor. |
| Future: WEL Windows-API mode (PIPE-928)           | Effector | Writes to actual Windows event log. Cannot be embedded.  |
| Future: REST simulators (PIPE-943)                | Effector | HTTP servers external clients poll. Cannot be embedded.  |
