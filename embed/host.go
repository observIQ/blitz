package embed

import (
	"github.com/observiq/blitz/internal/datagen"
)

// Host is the bundle of consumers and ambient resources a host process
// supplies to an embedded blitz runner.
//
// Each consumer field is optional: nil means the host does not want
// records of that signal type. Modules that produce a signal the host
// does not consume are still permitted; their output is dropped.
type Host struct {
	// Logs is the destination for log records. Nil means logs are dropped.
	Logs LogConsumer

	// Metrics is the destination for metric points. Nil means metrics are dropped.
	Metrics MetricConsumer

	// Traces is the destination for spans. Nil means spans are dropped.
	Traces TraceConsumer

	// Resource is the per-session base resource attributes blitz applies
	// to every emitted record before module-level overrides merge on top.
	//
	// Read-only after Host is passed to Runner.Start. The runner clones
	// this map internally so concurrent worker goroutines read a
	// runtime-owned copy that no caller can race with; callers must still
	// treat their own reference as frozen once they hand the Host off.
	// See cloneResource in this package.
	Resource map[string]any

	// Telemetry carries the OTel providers and logger blitz routes its own
	// self-telemetry through: the zap Logger for internal diagnostics, and the
	// Meter, Tracer, and Logger providers blitz builds its metrics, spans, and
	// log bridge from. It is the single source for all three signals.
	//
	// The host builds one bundle and supplies the same value here (used by the
	// runner for the session-level runtime) and as EmbedOpts.Telemetry (used at
	// generator construction). Every field is optional with a nil-safe
	// fallback, so a zero-value bundle behaves as blitz did before providers
	// were injectable. See TelemetrySettings.
	Telemetry TelemetrySettings

	// Environment is the simulated datagen.Environment that generators draw
	// their host identities (host.name / os.type) from (PIPE-1036). Nil means
	// generators fall back to the running host's os.Hostname().
	//
	// Read-only for the lifetime of a single Runner.Start: workers only read
	// it, so they share the pointer without synchronization. It is not
	// deep-copied the way Resource is; copying the whole identity graph would
	// be costly and buys nothing, since the Environment is never mutated in
	// place. Reconfiguration swaps it by rebuilding the runner with a fresh
	// Host (Stop, New, Start), not by mutating the live value, so a caller
	// replaces Environments across a rebuild rather than underneath running
	// workers. Callers must treat their reference as read-only once they hand
	// the Host off.
	Environment *datagen.Environment
}

// cloneResource returns a defensive copy of m. Runner.Start uses it so
// worker goroutines reading the per-session resource attributes race
// neither with each other nor with a caller that retains a reference
// and later mutates its own copy.
func cloneResource(m map[string]any) map[string]any {
	if m == nil {
		return nil
	}
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
