// Package appprotect registers the NGINX App Protect (WAF on NGINX)
// product: security events in the "default" security-log format.
//
// The field set and order are the documented "default" predefined format
// from F5's NGINX App Protect security-log reference — the comma-separated
// key="value" attributes the default format emits, in the documented order.
//
// Reference (validated 2026-09-25):
//
//	https://docs.nginx.com/waf/logging/security-logs/
//	(NGINX App Protect WAF Security Log — "Available Security Log
//	 Attributes", attributes included in the `default` format.)
package appprotect

import (
	"fmt"
	"math/rand"
	"strings"

	"github.com/observiq/blitz/generator/f5/catalog"
	"github.com/observiq/blitz/internal/datagen"
)

func init() { catalog.Register(catalog.Product{Name: "nginx-app-protect", Build: build}) }

// defaultFields is the ordered attribute set of the App Protect "default"
// security-log format, exactly as published in the security-log reference.
// The order here is the wire order build() emits.
var defaultFields = []string{
	"attack_type",
	"blocking_exception_reason",
	"bot_anomalies",
	"bot_category",
	"bot_signature_name",
	"client_class",
	"date_time",
	"dest_ip",
	"dest_port",
	"enforced_bot_anomalies",
	"ip_client",
	"is_truncated_bool",
	"json_log",
	"method",
	"outcome",
	"outcome_reason",
	"policy_name",
	"protocol",
	"request",
	"request_status",
	"response_code",
	"severity",
	"sig_cves",
	"sig_ids",
	"sig_names",
	"sig_set_names",
	"src_port",
	"sub_violations",
	"support_id",
	"threat_campaign_names",
	"unit_hostname",
	"uri",
	"username",
	"violation_details",
	"violation_rating",
	"violations",
	"vs_name",
	"x_forwarded_for_header_value",
	"transport_protocol",
	"client_application",
	"client_application_version",
}

// DefaultFields returns the documented default-format attribute order. Exposed
// for tests that assert the emitted field set/order matches the spec.
func DefaultFields() []string { return append([]string(nil), defaultFields...) }

var (
	attacks    = []string{"Non-browser Client", "SQL-Injection", "Cross Site Scripting (XSS)", "Abuse of Functionality"}
	sigNames   = []string{"XSS script tag end (Parameter)", "SQL-INJ UNION SELECT", "Automated client (cookie header)"}
	severities = []string{"Critical", "Error", "Warning"}
	statuses   = []string{"blocked", "alerted", "passed"}
	methods    = []string{"GET", "POST", "PUT"}
	policies   = []string{"app_protect_default_policy", "strict_owasp", "api_policy"}
	botCats    = []string{"N/A", "Untrusted Bot", "Trusted Bot", "Malicious Bot"}
	clientClas = []string{"Untrusted Bot", "Browser", "Trusted Bot", "Suspicious Browser"}
)

func build(r *rand.Rand, c *catalog.Ctx) string {
	status := statuses[r.Intn(len(statuses))]
	outcome, outcomeReason, respCode := "PASSED", "SECURITY_WAF_OK", 200
	if status == "blocked" {
		outcome, outcomeReason, respCode = "REJECTED", "SECURITY_WAF_VIOLATION", 0
	}
	attack := attacks[r.Intn(len(attacks))]

	// vals maps each documented attribute to its emitted value. Optional
	// attributes with no value emit empty per spec (e.g. sig_cves="N/A").
	vals := map[string]string{
		"attack_type":                  attack,
		"blocking_exception_reason":    "N/A",
		"bot_anomalies":                "N/A",
		"bot_category":                 botCats[r.Intn(len(botCats))],
		"bot_signature_name":           "N/A",
		"client_class":                 clientClas[r.Intn(len(clientClas))],
		"date_time":                    c.Now.Format("2006-01-02 15:04:05"),
		"dest_ip":                      datagen.RandomPrivateIPv4(r),
		"dest_port":                    "443",
		"enforced_bot_anomalies":       "N/A",
		"ip_client":                    datagen.RandomPublicIPv4(r),
		"is_truncated_bool":            "false",
		"json_log":                     "N/A",
		"method":                       methods[r.Intn(len(methods))],
		"outcome":                      outcome,
		"outcome_reason":               outcomeReason,
		"policy_name":                  policies[r.Intn(len(policies))],
		"protocol":                     "HTTPS",
		"request":                      "GET /api/v1/orders HTTP/1.1",
		"request_status":               status,
		"response_code":                fmt.Sprintf("%d", respCode),
		"severity":                     severities[r.Intn(len(severities))],
		"sig_cves":                     "N/A",
		"sig_ids":                      fmt.Sprintf("%d", 200000000+r.Intn(99999999)),
		"sig_names":                    sigNames[r.Intn(len(sigNames))],
		"sig_set_names":                "{Automated Threats;High Accuracy Signatures}",
		"src_port":                     fmt.Sprintf("%d", 1024+r.Intn(64000)),
		"sub_violations":               "N/A",
		"support_id":                   fmt.Sprintf("%d", 1000000000000000+r.Int63n(8999999999999999)),
		"threat_campaign_names":        "N/A",
		"unit_hostname":                c.Hostname,
		"uri":                          "/api/v1/orders",
		"username":                     "N/A",
		"violation_details":            "<?xml version=\"1.0\"?><BAD_MSG></BAD_MSG>",
		"violation_rating":             fmt.Sprintf("%d", 1+r.Intn(5)),
		"violations":                   "Illegal meta character in value",
		"vs_name":                      "/Common/app.example.com",
		"x_forwarded_for_header_value": datagen.RandomPublicIPv4(r),
		"transport_protocol":           "TCP",
		"client_application":           "N/A",
		"client_application_version":   "N/A",
	}

	header := catalog.SyslogHeader(134, c.Now, c.Hostname, "app_protect", 0)
	var b strings.Builder
	b.WriteString(header)
	b.WriteByte(' ')
	for i, name := range defaultFields {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(name)
		b.WriteString(`="`)
		b.WriteString(vals[name])
		b.WriteByte('"')
	}
	return b.String()
}
