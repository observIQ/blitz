// Package resource builds the per-record Resource map every blitz
// Producer attaches to its emitted records (LogRecord.Metadata.Resource,
// MetricPointMetadata.Resource, SpanMetadata.Resource).
//
// Every Producer SHOULD populate at minimum:
//
//   - host.name — the hostname the record semantically describes
//     (defaults to the process's os.Hostname(), or "blitz" if that
//     fails)
//   - telemetry.source — the module identifier (apache, paloalto,
//     fix, wel, etc.)
//
// Producers MAY add additional keys via the extras parameter:
// version, format flavor, channel, etc. — anything the generator
// knows internally that downstream consumers (telgenrec, OTel
// pipelines) would want to pivot on without parsing the message body.
package resource

import (
	"os"
	"sync"
)

var (
	hostnameOnce sync.Once
	hostname     string
)

// Hostname returns the current host's name, cached after the first
// call. Falls back to "blitz" if os.Hostname returns an error or empty
// string. Safe for concurrent calls.
func Hostname() string {
	hostnameOnce.Do(func() {
		h, _ := os.Hostname()
		if h == "" {
			h = "blitz"
		}
		hostname = h
	})
	return hostname
}

// Default returns a fresh Resource map for an emitted record,
// populated with host.name and telemetry.source = source. extras are
// key/value pairs appended to the map; pass an even number of
// strings. Each call returns a new map so consumers can mutate
// without affecting subsequent emissions.
//
// Example:
//
//	resource.Default("apache", "apache.format", "common")
//	// → {"host.name": "<host>", "telemetry.source": "apache", "apache.format": "common"}
func Default(source string, extras ...string) map[string]any {
	return WithHost(Hostname(), source, extras...)
}

// WithHost returns a Resource map like Default but with an explicit host.name,
// for generators whose hostname comes from a resolved datagen SystemIdentity
// (PIPE-1036) rather than the process's os.Hostname(). extras follow the same
// key/value convention as Default.
func WithHost(hostname, source string, extras ...string) map[string]any {
	r := map[string]any{
		"host.name":        hostname,
		"telemetry.source": source,
	}
	for i := 0; i+1 < len(extras); i += 2 {
		r[extras[i]] = extras[i+1]
	}
	return r
}

// StaticResources is an immutable set of resource attributes that stay constant
// for a generator worker's lifetime: the host-identity projection (host.name,
// os.type, ...) plus per-generator constants (telemetry.source, format flavor,
// version). Build it once at construction and reuse it for every record the
// worker emits (PIPE-1036).
//
// The model is: Static + Dynamic (per record) = Record. Static carries the
// fields that never change for the worker; Record merges in the few that do.
type StaticResources struct {
	attrs map[string]any
}

// NewStaticResources builds a StaticResources from a base attribute set. The
// map is copied, so a caller that retains and later mutates attrs does not
// affect the constructed value.
func NewStaticResources(attrs map[string]any) *StaticResources {
	cp := make(map[string]any, len(attrs))
	for k, v := range attrs {
		cp[k] = v
	}
	return &StaticResources{attrs: cp}
}

// Record returns the resource attributes for a single emitted record: the
// static set merged with the given dynamic key/value pairs (same even-length
// convention as Default's extras).
//
// When no dynamic pairs are supplied — the common case, since most generators
// vary nothing per record — Record returns the shared static map with no
// allocation. Callers MUST treat that returned map as read-only; mutating it
// corrupts every other record and races concurrent workers. When dynamic pairs
// are supplied, Record returns a fresh merged map that is safe to mutate and
// leaves the static set untouched.
func (s *StaticResources) Record(dynamicKV ...string) map[string]any {
	if len(dynamicKV) < 2 {
		return s.attrs
	}
	out := make(map[string]any, len(s.attrs)+len(dynamicKV)/2)
	for k, v := range s.attrs {
		out[k] = v
	}
	for i := 0; i+1 < len(dynamicKV); i += 2 {
		out[dynamicKV[i]] = dynamicKV[i+1]
	}
	return out
}
