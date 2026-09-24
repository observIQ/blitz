package catalog

import (
	"fmt"
	"time"
)

// bsdStamp is the BSD-syslog (RFC 3164) timestamp layout: month, space-
// padded day, time. F5 devices emit this on the syslog wire.
const bsdStamp = "Jan _2 15:04:05"

// SyslogHeader builds an RFC 3164 header: "<pri>timestamp host tag[pid]:".
// A pid <= 0 omits the "[pid]" segment (used by daemons like mcpd that
// log without one in some records).
func SyslogHeader(pri int, now time.Time, host, tag string, pid int) string {
	if pid <= 0 {
		return fmt.Sprintf("<%d>%s %s %s:", pri, now.Format(bsdStamp), host, tag)
	}
	return fmt.Sprintf("<%d>%s %s %s[%d]:", pri, now.Format(bsdStamp), host, tag, pid)
}
