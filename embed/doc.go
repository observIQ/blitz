// Package embed defines the contract for embedding blitz inside a host
// process that wants to consume the telemetry blitz produces.
//
// The package exposes three concerns:
//
//  1. Record types (LogRecord, MetricPoint, Span) — blitz-internal,
//     wire-format-agnostic. OTLP encoding happens at the OTLP-output
//     boundary or in the optional embed/otelpdata adapter.
//  2. Consumer interfaces (LogConsumer, MetricConsumer, TraceConsumer)
//     a host implements to receive records in-process.
//  3. Module classification (ProducerModule, EffectorModule) — modules
//     that yield records (Producer) versus modules whose effects land
//     outside blitz's process (Effector, not embed-eligible).
//
// Non-OTel hosts implement the consumer interfaces directly; OTel hosts
// can use the embed/otelpdata adapter to convert records to pdata.
package embed
