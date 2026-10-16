// Package afm registers the BIG-IP AFM (Advanced Firewall Manager) product:
// network firewall events.
//
// Lines use the default ("None") storage format: fourteen double-quoted,
// comma-separated values in F5's documented order, sent by tmm over a
// remote high-speed log destination at local0.info. Values use the spellings
// real events carry: "Virtual Server" and "Global" contexts (a Global event
// has an empty context_name), uppercase protocols, "Accept", "Accept
// Decisively" (Global context only), "Drop", "Reject", and no ports for ICMP.
//
// Sources (validated 2026-09-29):
//
//	https://techdocs.f5.com/kb/en-us/products/big-ip-afm/manuals/product/big-ip-network-firewall-policies-and-implementations-14-1-0/20.html
//	(default storage format field order)
//	f5devcentral/f5-big-iq-lab lab/traffic-scripts/playback/AFMPlayback/dataAFM/*.txt
//	(real AFM events: context, protocol, action, and drop_reason spellings)
//	https://cdn.f5.com/product/bugtracker/ID439094.html ("Accept Decisively" as logged)
//	elastic/integrations packages/f5_bigip/_dev/deploy/docker/sample_logs/log.log (drop_reason "Policy")
//	Juniper JSA F5 BIG-IP AFM DSM sample (tmm as the sending process)
package afm

import (
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"github.com/observiq/blitz/generator/f5/catalog"
	"github.com/observiq/blitz/internal/datagen"
)

func init() { catalog.Register(catalog.Product{Name: "afm", Build: build}) }

// pri is local0.info.
var pri = catalog.PRI(catalog.Local0, 6)

// event is one firewall event's values, in the documented default storage
// format order.
type event struct {
	MgmtIP, Host, ContextType, ContextName, SrcIP, DestIP, SrcPort, DestPort string
	VLAN, Protocol, RouteDomain, Rule, Action, DropReason                    string
}

// outcome is a rule decision and the context it can happen in.
type outcome struct {
	action, dropReason, context string
	rule                        string // empty when no rule matched
}

var (
	outcomes = []outcome{
		{"Accept", "", "Virtual Server", "allow_web"},
		{"Accept Decisively", "", "Global", "allow_trusted_sources"},
		{"Drop", "Policy", "Global", "deny_blocklisted_ips"},
		{"Reject", "Policy", "Virtual Server", "block_geo"},
		{"Drop", "Connection Flow Miss", "Virtual Server", ""},
	}
	vss       = []string{"/Common/vip1", "/Common/VS1", "/Common/vs_app"}
	protocols = []string{"TCP", "TCP", "UDP", "ICMP"}
)

func build(r *rand.Rand, c *catalog.Ctx) string {
	o := outcomes[r.Intn(len(outcomes))]
	e := event{
		MgmtIP: c.Device.MgmtIP, Host: c.FullName(), ContextType: o.context,
		SrcIP: datagen.RandomPublicIPv4(r), DestIP: datagen.RandomPrivateIPv4(r),
		VLAN: "/Common/external", Protocol: protocols[r.Intn(len(protocols))], RouteDomain: "0",
		Rule: o.rule, Action: o.action, DropReason: o.dropReason,
	}
	if o.context == "Virtual Server" {
		e.ContextName = vss[r.Intn(len(vss))]
	}
	if o.dropReason == "Connection Flow Miss" {
		e.Protocol = "TCP" // a flow miss is a TCP segment without a connection
	}
	if e.Protocol != "ICMP" {
		e.SrcPort = strconv.Itoa(1024 + r.Intn(64512))
		e.DestPort = strconv.Itoa([]int{80, 443, 22, 53, 3389}[r.Intn(5)])
	}
	return render(c.Now, c.Device.PID("tmm"), e)
}

// render emits the event as quoted CSV behind tmm's header.
func render(now time.Time, pid int, e event) string {
	vals := []string{e.MgmtIP, e.Host, e.ContextType, e.ContextName, e.SrcIP, e.DestIP, e.SrcPort,
		e.DestPort, e.VLAN, e.Protocol, e.RouteDomain, e.Rule, e.Action, e.DropReason}
	return fmt.Sprintf(`%s "%s"`, catalog.SyslogHeader(pri, now, e.Host, "tmm", pid), strings.Join(vals, `","`))
}
