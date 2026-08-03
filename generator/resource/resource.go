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
func Default(source string, extras ...string) map[string]string {
	return WithHost(Hostname(), source, extras...)
}

// WithHost returns a Resource map like Default but with an explicit host.name,
// for generators whose hostname comes from a resolved datagen SystemIdentity
// (PIPE-1036) rather than the process's os.Hostname(). extras follow the same
// key/value convention as Default.
func WithHost(hostname, source string, extras ...string) map[string]string {
	r := map[string]string{
		"host.name":        hostname,
		"telemetry.source": source,
	}
	for i := 0; i+1 < len(extras); i += 2 {
		r[extras[i]] = extras[i+1]
	}
	return r
}
