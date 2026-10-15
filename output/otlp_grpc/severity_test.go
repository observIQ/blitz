package otlpgrpc

import (
	"testing"

	"github.com/stretchr/testify/assert"
	logspb "go.opentelemetry.io/proto/otlp/logs/v1"
)

// TestMapSeverityNumber pins the level-to-SeverityNumber mapping, including
// the syslog NOTICE, CRITICAL, ALERT, and EMERGENCY levels (PAN-OS, F5,
// postgres), mapped per the OpenTelemetry log data model's syslog table
// (Appendix B).
func TestMapSeverityNumber(t *testing.T) {
	o := &OTLPGrpc{}
	for level, want := range map[string]logspb.SeverityNumber{
		"DEBUG":     logspb.SeverityNumber_SEVERITY_NUMBER_DEBUG,
		"INFO":      logspb.SeverityNumber_SEVERITY_NUMBER_INFO,
		"NOTICE":    logspb.SeverityNumber_SEVERITY_NUMBER_INFO2,
		"WARN":      logspb.SeverityNumber_SEVERITY_NUMBER_WARN,
		"ERROR":     logspb.SeverityNumber_SEVERITY_NUMBER_ERROR,
		"CRITICAL":  logspb.SeverityNumber_SEVERITY_NUMBER_ERROR2,
		"ALERT":     logspb.SeverityNumber_SEVERITY_NUMBER_ERROR3,
		"EMERGENCY": logspb.SeverityNumber_SEVERITY_NUMBER_FATAL,
		"FATAL":     logspb.SeverityNumber_SEVERITY_NUMBER_FATAL2,
		"other":     logspb.SeverityNumber_SEVERITY_NUMBER_INFO,
	} {
		assert.Equal(t, want, o.mapSeverityNumber(level), level)
	}
}
