// Package asm registers the BIG-IP ASM (Application Security Manager)
// product: WAF security events from a remote logging profile.
//
// Lines follow the key-value remote format F5's own telemetry-streaming
// fixtures carry: "ASM:" directly followed by the first key (no space), the
// 47 keys in their real order, the full request with CRLFs escaped as \r\n,
// and support_id as a 64-bit decimal. Attack events reuse the shared waf
// scenarios, so the request line, violations, and signatures agree.
//
// Sources (validated 2026-09-29):
//
//	F5Networks/f5-telemetry-streaming test/unit/tests/data/messageStreamTestsData.js
//	(real remote-logging line; testdata/golden.log)
//	layer-3-communications/f5-syslog common/Sample.hs
//	(passed requests logging response_code 200)
package asm

import (
	"math/rand"
	"strconv"
	"strings"
	"time"

	"github.com/observiq/blitz/generator/f5/catalog"
	"github.com/observiq/blitz/generator/f5/waf"
	"github.com/observiq/blitz/internal/datagen"
)

func init() { catalog.Register(catalog.Product{Name: "asm", Build: build}) }

// pri is local0.info, the remote logging profile's default facility.
var pri = catalog.PRI(catalog.Local0, 6)

// event is one ASM line's values.
type event struct {
	Now                          time.Time
	HeaderHost, UnitHost, MgmtIP string
	Policy, PolicyApplyDate      string
	SupportID                    uint64
	Status                       string
	ResponseCode                 int
	ClientIP, Method, Protocol   string
	Path, Query                  string
	DateTime                     time.Time
	Violations, SigIDs, SigNames string
	Severity, AttackType, Geo    string
	SessionID                    string
	SrcPort, DestPort            int
	DestIP                       string
	Rating                       int
	VSName, HostHeader           string
	// Headers are the request headers after Host, as "Name: value".
	Headers []string
}

var (
	policies = []string{"/Common/prod_waf_policy", "/Common/api_protection", "/Common/owasp_top10"}
	vsNames  = []string{"/Common/vs_https_443", "/Common/vs_app", "/Common/vs_api"}
	geos     = []string{"N/A", "US", "GB", "DE"}
	// cleanTargets are ordinary requests a "log all requests" profile records.
	cleanTargets = [][2]string{{"GET", "/"}, {"GET", "/index.html"}, {"POST", "/api/v1/orders"}, {"OPTIONS", "/hello"}}
)

// orEmpty maps the App Protect "N/A" placeholder to ASM's empty value.
func orEmpty(s string) string {
	if s == "N/A" {
		return ""
	}
	return s
}

func build(r *rand.Rand, c *catalog.Ctx) string {
	e := event{
		Now: c.Now, HeaderHost: c.Hostname, UnitHost: c.Hostname, MgmtIP: c.Device.MgmtIP,
		Policy:          policies[r.Intn(len(policies))],
		PolicyApplyDate: c.Now.Add(-time.Duration(1+r.Intn(90*24)) * time.Hour).Format("2006-01-02 15:04:05"),
		SupportID:       r.Uint64(),
		ClientIP:        datagen.RandomPublicIPv4(r),
		DateTime:        c.Now,
		Geo:             geos[r.Intn(len(geos))],
		SessionID:       strconv.FormatUint(r.Uint64(), 16),
		SrcPort:         1024 + r.Intn(64512),
		DestIP:          datagen.RandomPrivateIPv4(r),
		VSName:          vsNames[r.Intn(len(vsNames))],
	}
	e.Protocol, e.DestPort = "HTTPS", 443
	if r.Intn(4) == 0 {
		e.Protocol, e.DestPort = "HTTP", 80
	}

	// A third of lines are clean requests; the rest replay a waf scenario.
	if pick := r.Intn(len(waf.Scenarios) + 1); pick < len(waf.Scenarios) {
		s := waf.Scenarios[pick]
		e.Status, e.ResponseCode = s.Status, s.ResponseCode
		e.Method, e.Path, e.Query = s.Method, s.Path, s.Query
		e.Violations, e.SigIDs, e.SigNames = s.Violations, orEmpty(s.SigIDs), orEmpty(s.SigNames)
		e.Severity, e.AttackType, e.Rating = s.Severity, s.AttackTypes, s.Rating
		e.HostHeader = s.HostHeader
		e.Headers = []string{"User-Agent: " + s.UserAgent, "Accept: */*"}
	} else {
		t := cleanTargets[r.Intn(len(cleanTargets))]
		e.Status, e.ResponseCode = "passed", 200
		e.Method, e.Path = t[0], t[1]
		e.Severity = "Informational"
		e.HostHeader = e.DestIP + ":" + strconv.Itoa(e.DestPort)
		e.Headers = []string{"User-Agent: curl/8.4.0", "Accept: */*"}
	}
	return render(e)
}

// render emits the remote-logging line in its real key order.
func render(e event) string {
	target := e.Path
	if e.Query != "" {
		target += "?" + e.Query
	}
	req := e.Method + " " + target + " HTTP/1.1\r\nHost: " + e.HostHeader + "\r\n"
	for _, h := range e.Headers {
		req += h + "\r\n"
	}
	req += "\r\n"

	kv := [][2]string{
		{"unit_hostname", e.UnitHost},
		{"management_ip_address", e.MgmtIP},
		{"management_ip_address_2", ""},
		{"http_class_name", e.Policy},
		{"web_application_name", e.Policy},
		{"policy_name", e.Policy},
		{"policy_apply_date", e.PolicyApplyDate},
		{"violations", e.Violations},
		{"support_id", strconv.FormatUint(e.SupportID, 10)},
		{"request_status", e.Status},
		{"response_code", strconv.Itoa(e.ResponseCode)},
		{"ip_client", e.ClientIP},
		{"route_domain", "0"},
		{"method", e.Method},
		{"protocol", e.Protocol},
		{"query_string", e.Query},
		{"x_forwarded_for_header_value", "N/A"},
		{"sig_ids", e.SigIDs},
		{"sig_names", e.SigNames},
		{"date_time", e.DateTime.Format("2006-01-02 15:04:05")},
		{"severity", e.Severity},
		{"attack_type", e.AttackType},
		{"geo_location", e.Geo},
		{"ip_address_intelligence", "N/A"},
		{"username", "N/A"},
		{"session_id", e.SessionID},
		{"src_port", strconv.Itoa(e.SrcPort)},
		{"dest_port", strconv.Itoa(e.DestPort)},
		{"dest_ip", e.DestIP},
		{"sub_violations", ""},
		{"virus_name", "N/A"},
		{"violation_rating", strconv.Itoa(e.Rating)},
		{"websocket_direction", "N/A"},
		{"websocket_message_type", "N/A"},
		{"device_id", "N/A"},
		{"staged_sig_ids", ""},
		{"staged_sig_names", ""},
		{"threat_campaign_names", ""},
		{"staged_threat_campaign_names", ""},
		{"blocking_exception_reason", "N/A"},
		{"captcha_result", "not_received"},
		{"microservice", ""},
		{"vs_name", e.VSName},
		{"uri", e.Path},
		{"fragment", ""},
		{"request", strings.ReplaceAll(req, "\r\n", `\r\n`)},
		{"response", "Response logging disabled"},
	}
	var b strings.Builder
	b.WriteString(catalog.SyslogHeader(pri, e.Now, e.HeaderHost, "ASM", 0))
	for i, p := range kv {
		if i > 0 {
			b.WriteByte(',')
		}
		// The Splunk key-value format escapes a quote inside a value with a
		// backslash (F5 bug 1054297 states the requirement).
		b.WriteString(p[0] + `="` + strings.ReplaceAll(p[1], `"`, `\"`) + `"`)
	}
	return b.String()
}
