# Terminology

The words blitz uses for its own internal components, so the code, docs, tickets, and roadmap all
mean the same thing by them. Two axes run through the vocabulary: a component's **capability class**
(what it fundamentally is) and its **concrete kind** (what you actually configure).

## Capability classes

These live in the `embed` package and are fixed at compile time. Every component is exactly one of
them.

- **Module**: the base lifecycle interface, with `Name()`, `Start()`, and `Stop()`. Every component
  is a Module, classified as either a Producer or an Effector. The classification is a property of
  the implementation, not a runtime flag.
- **Producer**: a Module that yields telemetry records to consumer interfaces. A Producer is
  embed-eligible, so a host process can register it and receive its records in-process. A module
  declares itself a Producer by embedding `ProducerMarker`.
- **Effector**: a Module whose effects land outside blitz's process, such as an OS event log, a
  listening socket, files on disk, or a served API. A host cannot observe those effects in-process,
  so Effectors are not embed-eligible. A module declares itself an Effector by embedding
  `EffectorMarker`.

## Concrete kinds

- **Generator**: the common Producer. It creates telemetry in a specific format and yields records.
  Examples are `json`, `paloalto`, `f5`, `flow`, `hostmetrics`, and `winevt`.
- **Output**: the push side. It sends generated telemetry to a destination and implements the
  consumer interfaces. Examples are `stdout`, `tcp`, `udp`, `syslog`, `otlp-grpc`, `file`, `hec`,
  `prometheus-remote-write`, and `prometheus-scrape`.
- **Simulator**: a kind of Effector. It is a thin API facade over a shared `Environment`, projecting
  modeled machine state into a vendor's API (REST, SOAP, XML). A simulator owns no inventory of its
  own.
- **Protocol server**: the other kind of Effector. It answers a real client over a raw protocol (for
  example S7comm or IEC 61850), rather than over a vendor management API.

## Supporting substrate

- **Signal type** (also **record type**): a blitz-internal, wire-format-agnostic value a Producer
  yields. The types are `LogRecord`, `MetricPoint`, `Span`, and `FlowRecord`. Wire encoding (OTLP and
  the rest) happens at the output boundary, not inside the record.
- **Consumer**: the interface a host implements to receive records in-process, as `LogConsumer`,
  `MetricConsumer`, `TraceConsumer`, and `FlowConsumer`. Outputs and embedding hosts implement these.
- **Host**: a modeled machine's telemetry surfaces, namely `Host.Logs`, `Host.Metrics`,
  `Host.Traces`, and `Host.Flows`.
- **embed**: the importable library seam. It carries the record types, the consumer interfaces, and
  the Producer/Effector classification, so a host process can consume blitz telemetry in-process.
  `embed/otelpdata` is the optional adapter that converts records to OTel pdata.
- **datagen**: the shared data-generation substrate of deterministic pools for IPs, MACs, hostnames,
  operating systems, and machine identities.
- **Environment**: the single source of truth for a simulated deployment's topology and identities.
  Machines compose into an Environment, and every simulator is a facade over it, so a whole
  deployment stays internally consistent.
- **Machine**: a modeled system inside an Environment, such as a Windows server, an HPE array, or a
  network device.
- **SeedConfig**: the determinism contract carrier for per-worker and per-component RNG. A negative
  seed randomizes; a zero or positive seed is deterministic.
- **Shared module**: a second sense of the word module. A raw-built protocol becomes a self-contained
  importable Go package, and a layer used by two or more protocols gets its own package and ticket,
  sequenced first (for example ISO-on-TCP under both IEC 61850 and S7comm).

## Overlapping words

Two terms carry two senses, so the layer matters:

- **Output vs Effector.** A normal Output pushes telemetry and is plumbing on the Producer side. A
  pull output such as `prometheus-scrape` hosts an HTTP server, so it causes an effect outside the
  process and is Effector-shaped even though it lives under `output/`. It is built as an output today
  and moves to the Effector server model later.
- **Module.** `embed.Module` is the lifecycle interface with its Producer/Effector split. A shared
  module is an importable Go package for a raw-built protocol. Same word, different layer.
