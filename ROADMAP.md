# Roadmap

> **This roadmap is fluid. Everything here is subject to change, and nothing here carries an
> expected timeline or delivery date.** It is direction, not a schedule or a commitment. Rounds get
> reordered, rescoped, split, merged, and dropped as we learn.

Blitz aims to be the last, best, most magnificent telemetry generation/simulation tool anyone will
ever need. That means any signal, any protocol that carries telemetry, and any system that produces
or transports it. It ships to whatever consumes that telemetry, so OpenTelemetry is one important
consumer among many rather than the defining purpose.

## How to read this

Work is organized into numbered rounds. **The numbers are our chosen work order, not a strict
dependency graph.** Some rounds do unlock others. Round 6 is the foundation for the streaming
protocols, and Round 7 for the API simulators. The rest are sequenced because we chose to finish
the core before adding breadth.

Blitz is growing from a *generator* that emits telemetry into a *simulator* that stands in for the
systems telemetry comes from. A simulator here is a thin, faithful facade over shared synthetic
state. A real tool can talk to it and get consistent answers.

## Shipped

### Round 1: Foundation

What everything else builds on: finite generation, declaratively-defined metrics, shared synthetic
data generation, the embedded data library, and a normalized per-record metadata contract. This
round also added the embed seam and public embed API, so blitz can be consumed as a library.

## In progress

### Round 2: Bugfixes, tech debt, and standards

The maintenance and conformance round. It lands the fixes and tech-debt work that surfaced during
Round 1's review cycles, and brings the existing components up to one consistent standard.

## Planned

### Round 3: Additional data types and sources

NetFlow, sFlow, and IPFIX flow records. An F5 multi-product log generator spanning LTM, ASM, AFM,
APM, GTM, and AVR. A Prometheus output offering both a scrape endpoint and a remote-write client.

### Round 4: Simulating distributed environments

Take blitz from single-process generation to a coordinated simulation of a whole system of systems.
Two thrusts. First, enrich the environment model with the primitives that describe how machines
relate, fail, change, and sit in time and space. Second, coordinate multiple blitz instances into
one simulation.

### Round 5: Plugin and extensibility SDK

Make blitz extensible. Custom generators, written by anyone in any language, participate alongside
native generators in the same deployment, including full distributed topologies.

### Round 6: Push Core

One publish and fan-out engine with per-protocol wire adapters, in place of a separately-built
pub/sub stack per protocol. This is the shared foundation for every streaming, broker, and
push-based protocol that follows.

### Round 7: API scaffold and REST adapter

The generic, plugin-aware API simulator scaffold plus its first protocol adapter. It shares an HTTP
server, auth, and TLS substrate across every request/response protocol. REST proves the scaffold
before we fan out.

### Round 8: REST SaaS simulators

Stand-ins for SaaS vendor management and control-plane APIs: MongoDB Atlas, Meraki, SentinelOne, and
Akamai. Pipelines and any other consuming tool can be tested against synthetic vendor endpoints
backed by consistent state.

### Round 9: REST HPE storage suite

HPE storage-array control-plane simulators, built as one coherent vendor-family cluster. An
array's management API can be exercised end to end, not stubbed per endpoint.

### Round 10: Non-REST HTTP API simulators

The remaining HTTP request/response protocols, including SOAP and XML-over-HTTP. These share the
Round 7 transport substrate and differ mainly in dispatch and encoding.

### Round 11: Streaming API simulators

The persistent-connection APIs, built as adapters on Push Core. Covers Server-Sent Events and the
WebSocket family.

### Round 12: Industrial and IoT protocol simulation

ModBus and MQTT, each shipping as both a data generator and a real protocol server. A ModBus TCP
slave answers register reads from any master, and an MQTT broker serves any MQTT client. Sparkplug B
rides on the MQTT path. OPC-UA, BACnet, DLMS/COSEM, and IEC 61850 follow as full simulators. A YAML
device-profile library covers smart manufacturing, smart grid, and building automation.

### Round 13: Message bus simulation

Target the transport layer, not the systems producing telemetry. Kafka, RabbitMQ, NATS, Google Cloud
Pub/Sub, and ActiveMQ each run as a real broker any client can connect to. A blitz generator routes
into the broker. An external consumer reads from it over the real wire protocol, with no idea blitz
is on the other end.

### Round 14: Networking device simulation

Network-fabric devices (switches, routers, firewalls, load balancers) as connectable endpoints. Each
is reachable over the device's real management surfaces and backed by consistent state.

### Round 15: Cloud simulation

Cloud as a first-class entity in the synthetic data model. A simulated host's cloud placement and
image provenance become provider-correct instead of approximated.

### Round 16: User-configurable deployment tiers

Replace the current fixed deployment tiers with a user-configurable tier model, so the environments
blitz simulates match the ones you actually run.

## Contributing to the roadmap

Requests and ideas are welcome. Open an [issue](https://github.com/observiq/blitz/issues) to propose
a generator, an output, a protocol, or a system worth simulating. The scope test is broad: if it
produces, transports, or consumes telemetry, it is a candidate.
