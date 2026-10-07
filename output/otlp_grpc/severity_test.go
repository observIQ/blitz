package otlpgrpc

import (
	"testing"

	logspb "go.opentelemetry.io/proto/otlp/logs/v1"
)

// Generators emit levels in several spellings. Each must map to its real
// severity, not fall through to INFO.
func TestMapSeverityNumber(t *testing.T) {
	o := &OTLPGrpc{}
	cases := map[string]logspb.SeverityNumber{
		// uppercase (existing behavior)
		"DEBUG": logspb.SeverityNumber_SEVERITY_NUMBER_DEBUG,
		"INFO":  logspb.SeverityNumber_SEVERITY_NUMBER_INFO,
		"WARN":  logspb.SeverityNumber_SEVERITY_NUMBER_WARN,
		"ERROR": logspb.SeverityNumber_SEVERITY_NUMBER_ERROR,
		"FATAL": logspb.SeverityNumber_SEVERITY_NUMBER_FATAL2,
		// lowercase (kubernetes, apache error)
		"debug": logspb.SeverityNumber_SEVERITY_NUMBER_DEBUG,
		"info":  logspb.SeverityNumber_SEVERITY_NUMBER_INFO,
		"warn":  logspb.SeverityNumber_SEVERITY_NUMBER_WARN,
		"error": logspb.SeverityNumber_SEVERITY_NUMBER_ERROR,
		"fatal": logspb.SeverityNumber_SEVERITY_NUMBER_FATAL2,
		// Windows Event Log level names
		"Verbose":     logspb.SeverityNumber_SEVERITY_NUMBER_TRACE,
		"Information": logspb.SeverityNumber_SEVERITY_NUMBER_INFO,
		"Warning":     logspb.SeverityNumber_SEVERITY_NUMBER_WARN,
		"Error":       logspb.SeverityNumber_SEVERITY_NUMBER_ERROR,
		"Critical":    logspb.SeverityNumber_SEVERITY_NUMBER_FATAL,
		// syslog-style (apache error)
		"notice": logspb.SeverityNumber_SEVERITY_NUMBER_INFO2,
		"crit":   logspb.SeverityNumber_SEVERITY_NUMBER_FATAL,
		// PostgreSQL
		"LOG":     logspb.SeverityNumber_SEVERITY_NUMBER_INFO,
		"WARNING": logspb.SeverityNumber_SEVERITY_NUMBER_WARN,
		"PANIC":   logspb.SeverityNumber_SEVERITY_NUMBER_FATAL4,
		// unknown
		"":      logspb.SeverityNumber_SEVERITY_NUMBER_INFO,
		"bogus": logspb.SeverityNumber_SEVERITY_NUMBER_INFO,
	}
	for level, want := range cases {
		if got := o.mapSeverityNumber(level); got != want {
			t.Errorf("mapSeverityNumber(%q) = %v, want %v", level, got, want)
		}
	}
}
