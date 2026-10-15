// Package apm registers the BIG-IP APM (Access Policy Manager) product:
// access-session lifecycle logs.
//
// Every message ID and text is from real APM logs. Each session walks the
// real sequence under one session ID: user agent seen, session created
// (01490500), username, policy branch, policy result, client info (allowed
// sessions only), session deleted (01490520), and closing statistics
// (01490521). tmm logs the traffic-side messages and apmd the policy-side
// ones, each under its stable PID, at local1.notice (K7342: APM uses local1).
//
// Sources (validated 2026-09-29):
//
//	https://community.f5.com/t5/technical-forum/outlook-anywhere-2016-apm-rpc-issues/td-p/169738
//	https://community.f5.com/discussions/technicalforum/remote-desktop-web-access-and-remote-desktop-gateway-sso-through-apm/69459
//	(real APM session lines; testdata/golden.log)
//	splunk/splunk-connect-for-syslog tests/test_f5_bigip.py (01490248 client-info line)
//	https://my.f5.com/manage/s/article/K7342 (syslog facilities)
package apm

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/observiq/blitz/generator/f5/catalog"
	"github.com/observiq/blitz/internal/datagen"
)

func init() { catalog.Register(catalog.Product{Name: "apm", Build: build}) }

// pri is local1.notice.
var pri = catalog.PRI(catalog.Local1, 5)

// session is one access session in flight.
type session struct {
	prefix   string // "/Common/<profile>:Common:<sid>"
	step     int
	allow    bool
	user, ip string
	geo      string
	vip      string
	listener string
	ua       string
}

var (
	profiles = []string{"exchange2016", "ap_vpn", "portal_access"}
	users    = []string{"jsmith", "adoe", "svc_app", "contractor1"}
	geos     = []string{"ST=Ile-de-France/CC=FR/C=EU", "ST=Washington/CC=US/C=NA", "ST=California/CC=US/C=NA"}
	// agents are request User-Agent values as APM logs them: URL-encoded.
	agents = []string{"MSRPC", "Microsoft%20Office%2f16.0%20(Windows%20NT%2010.0%3b%20Microsoft%20Outlook%2016.0.4266%3b%20Pro)", "Mozilla%2f5.0%20(Windows%20NT%2010.0%3b%20Win64%3b%20x64)"}
	items  = []string{"SSO Credential Mapping", "NTLM Auth Result", "Logon Page"}
)

// maxOpen caps a worker's concurrent sessions.
const maxOpen = 4

func build(r *rand.Rand, c *catalog.Ctx) string {
	open := c.State("apm", func() any { return &[]*session{} }).(*[]*session)
	if len(*open) == 0 || (len(*open) < maxOpen && r.Intn(3) == 0) {
		*open = append(*open, newSession(r))
	}
	i := r.Intn(len(*open))
	s := (*open)[i]
	tag, pid, code, msg := s.next(r, c)
	if s.done() {
		*open = append((*open)[:i], (*open)[i+1:]...)
	}
	return render(c.Now, c.Hostname, tag, pid, code, s.prefix, msg)
}

func newSession(r *rand.Rand) *session {
	return &session{
		prefix:   fmt.Sprintf("/Common/%s:Common:%08x", profiles[r.Intn(len(profiles))], r.Uint32()),
		allow:    r.Intn(4) != 0,
		user:     users[r.Intn(len(users))],
		ip:       datagen.RandomPublicIPv4(r),
		geo:      geos[r.Intn(len(geos))],
		vip:      datagen.RandomPrivateIPv4(r),
		listener: "/Common/vs_apm_https",
		ua:       agents[r.Intn(len(agents))],
	}
}

// steps returns how many lines this session logs in all.
func (s *session) steps() int {
	if s.allow {
		return 8
	}
	return 7
}

func (s *session) done() bool { return s.step >= s.steps() }

// next returns the session's next line: process tag, PID, message ID, and
// text, then advances it.
func (s *session) next(r *rand.Rand, c *catalog.Ctx) (string, int, string, string) {
	tmm := func(code, msg string) (string, int, string, string) {
		return catalog.TmmTag(r), c.Device.PID("tmm"), code, msg
	}
	apmd := func(code, msg string) (string, int, string, string) {
		return "apmd", c.Device.PID("apmd"), code, msg
	}
	step := s.step
	s.step++
	if !s.allow && step >= 5 {
		step++ // denied sessions skip the client-info line
	}
	ending, result := "Deny", "Logon_Deny"
	if s.allow {
		ending, result = "Allow", "LTM+APM_Mode"
	}
	switch step {
	case 0:
		return tmm("01490506", "Received User-Agent header: "+s.ua+".")
	case 1:
		return tmm("01490500", fmt.Sprintf("New session from client IP %s (%s) at VIP %s Listener %s (Reputation=Unknown)", s.ip, s.geo, s.vip, s.listener))
	case 2:
		return apmd("01490010", fmt.Sprintf("Username '%s'", s.user))
	case 3:
		return apmd("01490005", fmt.Sprintf("Following rule 'fallback' from item '%s' to ending '%s'", items[r.Intn(len(items))], ending))
	case 4:
		return apmd("01490102", "Access policy result: "+result)
	case 5:
		return apmd("01490248", "Received client info - Hostname:  Type: IE Version: 8 Platform: Win7 CPU: WOW64 UI Mode: Full Javascript Support: 1 ActiveX Support: 1 Plugin Support: 0")
	case 6:
		return tmm("01490520", "Session deleted due to admin initiated termination.")
	default:
		return tmm("01490521", fmt.Sprintf("Session statistics - bytes in: %d, bytes out: %d", r.Intn(200000), r.Intn(2000000)))
	}
}

// render renders one APM line: "<code>:5: <prefix>: <message>".
func render(now time.Time, host, tag string, pid int, code, prefix, msg string) string {
	return fmt.Sprintf("%s %s:5: %s: %s", catalog.BigIPHeader(pri, now, host, tag, pid), code, prefix, msg)
}
