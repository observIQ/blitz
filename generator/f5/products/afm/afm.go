// Package afm registers the BIG-IP AFM (Advanced Firewall Manager)
// product: L3/L4 network firewall events.
//
// Byte-exact to F5's documented DEFAULT network-firewall log format (the
// "None" storage-format type): a comma-separated, double-quoted, positional
// value list in the documented field order below.
//
// Reference (validated 2026-09-25):
//
//	https://techdocs.f5.com/kb/en-us/products/big-ip-afm/manuals/product/network-firewall-policies-implementations-11-6-0/13.html
//	(Local Logging with the Network Firewall — default log format field order.)
package afm

import (
	"fmt"
	"math/rand"
	"strings"

	"github.com/observiq/blitz/generator/f5/catalog"
	"github.com/observiq/blitz/internal/datagen"
)

func init() { catalog.Register(catalog.Product{Name: "afm", Build: build}) }

// defaultFields is the documented AFM default ("None") log format field order.
// The wire format is positional quoted CSV in exactly this order.
var defaultFields = []string{
	"management_ip_address",
	"bigip_hostname",
	"context_type",
	"context_name",
	"src_ip",
	"dest_ip",
	"src_port",
	"dest_port",
	"vlan",
	"protocol",
	"route_domain",
	"acl_rule_name",
	"action",
	"drop_reason",
}

// DefaultFields returns the documented default field order (for tests).
func DefaultFields() []string { return append([]string(nil), defaultFields...) }

var (
	actions     = []string{"Drop", "Reject", "Accept", "Accept-Decisively"}
	protocols   = []string{"tcp", "udp", "icmp"}
	dropReasons = []string{"Policy", "Blacklisted address", "No route to host", "Port denied"}
	rules       = []string{"deny_inbound_rfc1918", "allow_web", "block_geo_cn", "default_deny"}
	ctxTypes    = []string{"Virtual Server", "Route Domain", "Global", "Self IP"}
)

func build(r *rand.Rand, c *catalog.Ctx) string {
	action := actions[r.Intn(len(actions))]
	dropReason := ""
	if action == "Drop" || action == "Reject" {
		dropReason = dropReasons[r.Intn(len(dropReasons))]
	}
	// Positional values in defaultFields order.
	vals := map[string]string{
		"management_ip_address": datagen.RandomPrivateIPv4(r),
		"bigip_hostname":        c.Hostname,
		"context_type":          ctxTypes[r.Intn(len(ctxTypes))],
		"context_name":          "/Common/vs_app",
		"src_ip":                datagen.RandomPublicIPv4(r),
		"dest_ip":               datagen.RandomPrivateIPv4(r),
		"src_port":              fmt.Sprintf("%d", 1024+r.Intn(64000)),
		"dest_port":             fmt.Sprintf("%d", []int{80, 443, 22, 53, 3389}[r.Intn(5)]),
		"vlan":                  "/Common/external",
		"protocol":              protocols[r.Intn(len(protocols))],
		"route_domain":          "0",
		"acl_rule_name":         rules[r.Intn(len(rules))],
		"action":                action,
		"drop_reason":           dropReason,
	}

	header := catalog.SyslogHeader(134, c.Now, c.Hostname, "tmm", 4000+r.Intn(2000))
	parts := make([]string, len(defaultFields))
	for i, name := range defaultFields {
		parts[i] = `"` + vals[name] + `"`
	}
	return header + " " + strings.Join(parts, ",")
}
