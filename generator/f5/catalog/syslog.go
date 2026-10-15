package catalog

import (
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"time"
)

// bsdStamp is the BSD-syslog (RFC 3164) timestamp layout: month, space-
// padded day, time. F5 devices emit this on the syslog wire.
const bsdStamp = "Jan _2 15:04:05"

// Syslog facilities F5 products log under. BIG-IP routes each module to its
// own local facility (K7342: local0 BIG-IP/tmm, local1 APM); NGINX defaults
// to local7 (nginx.org syslog docs); F5OS sends audit.log under authpriv
// (K000135458).
const (
	AuthPriv = 10
	Local0   = 16
	Local1   = 17
	Local7   = 23
)

// levelWords are the syslog-ng severity names BIG-IP writes after the host,
// indexed by severity (as in "SV5-F5-5600-2 warning tmm1[23068]:").
var levelWords = [8]string{"emerg", "alert", "crit", "err", "warning", "notice", "info", "debug"}

// PRI returns the syslog priority value for a facility and severity.
func PRI(facility, severity int) int { return facility*8 + severity }

// SyslogHeader builds an RFC 3164 header: "<pri>timestamp host tag[pid]:".
// A pid <= 0 omits the "[pid]" segment, as ASM and NGINX log without one.
func SyslogHeader(pri int, now time.Time, host, tag string, pid int) string {
	if pid <= 0 {
		return fmt.Sprintf("<%d>%s %s %s:", pri, now.Format(bsdStamp), host, tag)
	}
	return fmt.Sprintf("<%d>%s %s %s[%d]:", pri, now.Format(bsdStamp), host, tag, pid)
}

// BigIPHeader builds the header BIG-IP's syslog-ng emits for its daemons
// (tmm, mcpd, apmd, tmsh): the RFC 3164 header with the severity word, taken
// from the PRI, between the host and the tag.
func BigIPHeader(pri int, now time.Time, host, tag string, pid int) string {
	return fmt.Sprintf("<%d>%s %s %s %s[%d]:", pri, now.Format(bsdStamp), host, levelWords[pri%8], tag, pid)
}

// TmmTag picks the tmm process tag. tmm runs one thread per core under a
// single PID, and each thread logs as tmm, tmm1, tmm2, and so on.
func TmmTag(r *rand.Rand) string {
	if n := r.Intn(4); n > 0 {
		return "tmm" + strconv.Itoa(n)
	}
	return "tmm"
}

// SeverityFromPRI maps the leading "<pri>" of a line to the severity name of
// its syslog level. A line without a parsable PRI reads as INFO.
func SeverityFromPRI(line string) string {
	end := strings.IndexByte(line, '>')
	if !strings.HasPrefix(line, "<") || end < 2 {
		return "INFO"
	}
	pri, err := strconv.Atoi(line[1:end])
	if err != nil || pri < 0 {
		return "INFO"
	}
	return [8]string{"EMERGENCY", "ALERT", "CRITICAL", "ERROR", "WARN", "NOTICE", "INFO", "DEBUG"}[pri%8]
}
