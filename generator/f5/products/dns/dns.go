// Package dns registers the BIG-IP DNS (formerly GTM) product: GSLB /
// DNS query-response logs from the DNS data plane.
//
// BIG-IP DNS logging is driven by an operator-defined DNS logging profile,
// so there is no fixed positional wire spec. This emits a realistic instance
// of a default DNS/GTM query-resolution line. Config-dependent.
//
// Reference (validated 2026-09-25):
//
//	https://techdocs.f5.com/en-us/bigip-17-5-0/external-monitoring-of-big-ip-systems-implementations.html
package dns

import (
	"fmt"
	"math/rand"

	"github.com/observiq/blitz/generator/f5/catalog"
	"github.com/observiq/blitz/internal/datagen"
)

func init() { catalog.Register(catalog.Product{Name: "dns", Build: build}) }

var (
	qnames  = []string{"www.example.com", "api.example.com", "app.corp.example.net", "mail.example.org"}
	qtypes  = []string{"A", "AAAA", "CNAME", "MX", "SRV"}
	wideips = []string{"/Common/www_gslb", "/Common/api_gslb", "/Common/app_gslb"}
	pools   = []string{"/Common/pool_us_east", "/Common/pool_eu_west", "/Common/pool_ap_south"}
	results = []string{"RESOLVED", "NOERROR", "NXDOMAIN", "SERVFAIL"}
)

func build(r *rand.Rand, c *catalog.Ctx) string {
	header := catalog.SyslogHeader(134, c.Now, c.Hostname, "tmm", 4000+r.Intn(2000))
	qname := qnames[r.Intn(len(qnames))]
	qtype := qtypes[r.Intn(len(qtypes))]
	wideip := wideips[r.Intn(len(wideips))]
	pool := pools[r.Intn(len(pools))]
	member := datagen.RandomPublicIPv4(r)
	result := results[r.Intn(len(results))]
	// GTM/DNS query-resolution line.
	return fmt.Sprintf(`%s client %s#%d: query [%s %s] wideip %s -> pool %s member %s result %s rtt %dms`,
		header, datagen.RandomPublicIPv4(r), 1024+r.Intn(64000), qname, qtype, wideip, pool, member, result, 1+r.Intn(200))
}
