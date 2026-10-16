package dispatch

import (
	"github.com/observiq/blitz/internal/config"
	"github.com/observiq/blitz/telemetry"
)

// SignalsFor returns the telemetry signals a generator type emits. It is
// the single source of truth for the generator-to-signal mapping that the
// per-type consumer requirements in ForEmbed enforce, and that startup
// signal-compatibility validation consults. An unknown or signal-less type
// (nop) returns nil.
func SignalsFor(t config.GeneratorType) []telemetry.Type {
	switch t {
	case config.GeneratorTypeHostMetrics:
		return []telemetry.Type{telemetry.Metrics}
	case config.GeneratorTypeTraces:
		return []telemetry.Type{telemetry.Traces}
	case config.GeneratorTypeNop:
		return nil
	case config.GeneratorTypeJSON,
		config.GeneratorTypePaloAlto,
		config.GeneratorTypeApache,
		config.GeneratorTypeApacheCombined,
		config.GeneratorTypeApacheError,
		config.GeneratorTypeNginx,
		config.GeneratorTypePostgres,
		config.GeneratorTypeKubernetes,
		config.GeneratorTypeFile,
		config.GeneratorTypeOkta,
		config.GeneratorTypeWel,
		config.GeneratorTypeFIX,
		config.GeneratorTypeWinevt:
		return []telemetry.Type{telemetry.Logs}
	default:
		return nil
	}
}
