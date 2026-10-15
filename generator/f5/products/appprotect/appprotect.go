// Package appprotect registers the NGINX App Protect (WAF on NGINX)
// product: security events in the "default" security-log format.
//
// The wire order, key names (is_truncated, not the documented attribute name
// is_truncated_bool), CSV-style doubled quotes inside values, the single-
// quoted violation_details XML, and the json_log payload all follow real
// default-format lines. The log_default profile logs illegal requests only,
// so every event is blocked or alerted.
//
// Sources (validated 2026-09-29):
//
//	f5devcentral/NGINX-Gateway-Fabric-Lab labs/11.waf/README.md at 00c6cae
//	(two real default-format lines; testdata/golden.log)
//	https://docs.nginx.com/waf/logging/security-logs/
//	(attribute list, log_default = log_illegal, json_log since 4.3)
package appprotect

import (
	"encoding/base64"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"github.com/observiq/blitz/generator/f5/catalog"
	"github.com/observiq/blitz/generator/f5/waf"
	"github.com/observiq/blitz/internal/datagen"
)

func init() {
	catalog.Register(catalog.Product{Name: "nginx-app-protect", Host: catalog.HostNGINX, Build: build})
}

// pri is local0.info: App Protect's logger sends every event at info.
var pri = catalog.PRI(catalog.Local0, 6)

// event is one security-log line's per-request values.
type event struct {
	Now       time.Time
	Host      string
	ClientIP  string
	SrcPort   int
	DestPort  int
	SupportID uint64
	S         waf.Scenario
}

func build(r *rand.Rand, c *catalog.Ctx) string {
	return render(event{
		Now:       c.Now,
		Host:      c.Hostname,
		ClientIP:  datagen.RandomPublicIPv4(r),
		SrcPort:   1024 + r.Intn(64512),
		DestPort:  80,
		SupportID: r.Uint64(),
		S:         waf.Scenarios[r.Intn(len(waf.Scenarios))],
	})
}

// render emits the default-format line in its real wire order.
func render(e event) string {
	s := e.S
	id := strconv.FormatUint(e.SupportID, 10)
	raw := s.RawRequest()
	kv := [][2]string{
		{"attack_type", s.AttackTypes},
		{"blocking_exception_reason", "N/A"},
		{"date_time", e.Now.Format("2006-01-02 15:04:05")},
		{"dest_port", strconv.Itoa(e.DestPort)},
		{"ip_client", e.ClientIP},
		{"is_truncated", "false"},
		{"method", s.Method},
		{"policy_name", s.Policy},
		{"protocol", "HTTP"},
		{"request_status", s.Status},
		{"response_code", strconv.Itoa(s.ResponseCode)},
		{"severity", s.Severity},
		{"sig_cves", s.SigCVEs},
		{"sig_ids", s.SigIDs},
		{"sig_names", s.SigNames},
		{"sig_set_names", s.SigSetNames},
		{"src_port", strconv.Itoa(e.SrcPort)},
		{"sub_violations", "N/A"},
		{"support_id", id},
		{"threat_campaign_names", "N/A"},
		{"unit_hostname", e.Host},
		{"uri", s.Path},
		{"violation_rating", strconv.Itoa(s.Rating)},
		{"vs_name", s.VSName},
		{"x_forwarded_for_header_value", "N/A"},
		{"outcome", s.Outcome},
		{"outcome_reason", s.OutcomeReason},
		{"violations", s.Violations},
		{"json_log", jsonLog(e, id, raw)},
		{"violation_details", s.ViolationDetails},
		{"bot_signature_name", s.BotSignature},
		{"bot_category", s.BotCategory},
		{"bot_anomalies", "N/A"},
		{"enforced_bot_anomalies", "N/A"},
		{"client_class", s.ClientClass},
		{"client_application", "N/A"},
		{"client_application_version", "N/A"},
		{"request", strings.ReplaceAll(raw, "\r\n", `\r\n`)},
		{"transport_protocol", "HTTP/1.1"},
	}

	var b strings.Builder
	b.WriteString(catalog.SyslogHeader(pri, e.Now, e.Host, "ASM", 0))
	for i, p := range kv {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(p[0])
		b.WriteString(`="`)
		// App Protect escapes a quote inside a value by doubling it.
		b.WriteString(strings.ReplaceAll(p[1], `"`, `""`))
		b.WriteByte('"')
	}
	return b.String()
}

// jsonLog builds the json_log payload: the scenario's violations and
// enforcement state wrapped in this request's envelope, key order as logged.
func jsonLog(e event, id, raw string) string {
	s := e.S
	b64 := base64.StdEncoding.EncodeToString
	return fmt.Sprintf(`{"id":"%s","violations":%s,"enforcementAction":"%s","method":"%s","clientPort":%d,"clientIp":"%s","host":"%s","responseCode":%d,"serverIp":"0.0.0.0","serverPort":%d,"requestStatus":"%s","url":"%s","virtualServerName":"%s","geolocationCountryCode":"N/A","enforcementState":%s,"requestDatetime":"%s","rawRequest":{"actualSize":%d,"httpRequest":"%s","isTruncated":false},"requestPolicy":{"fullPath":"%s"}}`,
		id, s.JSONViolations, s.EnforcementAction, s.Method, e.SrcPort, e.ClientIP, e.Host, s.ResponseCode,
		e.DestPort, s.Status, b64([]byte(s.Path)), s.VSName, s.JSONEnforcement,
		e.Now.UTC().Format("2006-01-02T15:04:05Z"), len(raw), b64([]byte(raw)), s.Policy)
}
