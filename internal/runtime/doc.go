// Package runtime is the shared lifecycle core that the CLI and embed
// runners wrap: it starts every Module, stops each on shutdown, and emits
// the session's self-telemetry. Process-level concerns (signals, YAML
// config) and output wiring stay with the callers.
package runtime
