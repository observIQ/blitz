package embed

import "go.uber.org/zap"

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

	// Logger is the zap logger blitz uses for internal diagnostics. Nil
	// means blitz constructs a no-op logger.
	Logger *zap.Logger

	// Resource is the per-session base resource attributes blitz applies
	// to every emitted record before module-level overrides merge on top.
	//
	// Read-only after Host is passed to Runner.Start. The runner clones
	// this map internally so concurrent worker goroutines read a
	// runtime-owned copy that no caller can race with; callers must still
	// treat their own reference as frozen once they hand the Host off.
	Resource map[string]string
}
