// Package dns registers the BIG-IP DNS (formerly GTM) product: query and
// response lines from a DNS logging profile (K65762138).
//
// Each query line is later answered by a response line with the same qid,
// client, and name, as a real resolver pair logs:
//
//	<date> <host> qid <id> from <client>#<port>: view none: query: <name> IN <type> +E (<listener>%0)
//	<date> <host> qid <id> to <client>#<port>: [NOERROR qr,rd,ra] response: <rr>; <rr>;
//
// Sources (validated 2026-09-29):
//
//	https://clouddocs.f5.com/training/community/dns/html/class2/module7/lab3.html
//	(F5 DNS lab, real query and response lines; testdata/golden.log)
//	https://my.f5.com/manage/s/article/K65762138 (Configuring BIG-IP DNS to log DNS queries and responses)
//	https://community.f5.com/discussions/technicalforum/dns-logging-profile---response-ok-but-query-are-empty/264538
//	(further query/response pairs in the same shape)
package dns

import (
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/observiq/blitz/generator/f5/catalog"
	"github.com/observiq/blitz/internal/datagen"
)

func init() { catalog.Register(catalog.Product{Name: "dns", Build: build}) }

// pri is local0.info, where tmm writes DNS logging-profile output.
var pri = catalog.PRI(catalog.Local0, 6)

// query is one client DNS question.
type query struct {
	ID     int
	Client string
	Port   int
	Name   string
	Type   string
	// Server is the listener address the query arrived on.
	Server string
}

// state is a worker's DNS listener and its not-yet-answered queries.
type state struct {
	listener string
	pending  []query
}

var names = []string{"www.example.com", "api.example.com", "app.corp.example.net", "mail.example.org"}

func build(r *rand.Rand, c *catalog.Ctx) string {
	st := c.State("dns", func() any { return &state{listener: datagen.RandomPrivateIPv4(r)} }).(*state)
	tag, pid := catalog.TmmTag(r), c.Device.PID("tmm")

	// Answer an open query about half the time, or always once a few are open.
	if n := len(st.pending); n > 0 && (n >= 4 || r.Intn(2) == 0) {
		i := r.Intn(n)
		q := st.pending[i]
		st.pending = append(st.pending[:i], st.pending[i+1:]...)
		return renderResponse(c.Now, c.Hostname, tag, pid, c.Now, c.FullName(), q, "qr,rd,ra", answers(r, q))
	}
	q := query{
		ID: r.Intn(65536), Client: datagen.RandomPrivateIPv4(r), Port: 1024 + r.Intn(64512),
		Name: names[r.Intn(len(names))], Type: []string{"A", "AAAA"}[r.Intn(2)], Server: st.listener,
	}
	st.pending = append(st.pending, q)
	return renderQuery(c.Now, c.Hostname, tag, pid, c.Now, c.FullName(), q)
}

// answers draws the answer records for q: sometimes a CNAME to a CDN name,
// then one or more address records.
func answers(r *rand.Rand, q query) []string {
	owner := q.Name
	var rrs []string
	if r.Intn(3) == 0 {
		cdn := fmt.Sprintf("d%x.cloudfront.net", r.Uint32())
		rrs = append(rrs, fmt.Sprintf("%s. %d IN CNAME %s", owner, 30+r.Intn(270), cdn))
		owner = cdn
	}
	ttl := 30 + r.Intn(270)
	for i := 1 + r.Intn(3); i > 0; i-- {
		addr := datagen.RandomPublicIPv4(r)
		if q.Type == "AAAA" {
			addr = datagen.RandomPublicIPv6(r)
		}
		rrs = append(rrs, fmt.Sprintf("%s. %d IN %s %s", owner, ttl, q.Type, addr))
	}
	return rrs
}

// renderQuery renders a query line. hdr is the syslog header time and at the
// profile's own timestamp. They are separate because real captures can differ
// by a second (the golden lines pin that); the generator passes the same time.
func renderQuery(hdr time.Time, host, tag string, pid int, at time.Time, logHost string, q query) string {
	return fmt.Sprintf("%s %s %s qid %d from %s#%d: view none: query: %s IN %s +E (%s%%0)",
		catalog.BigIPHeader(pri, hdr, host, tag, pid), at.Format(time.DateTime), logHost,
		q.ID, q.Client, q.Port, q.Name, q.Type, q.Server)
}

// renderResponse renders the response line answering q; each record is
// followed by "; " and the last by ";".
func renderResponse(hdr time.Time, host, tag string, pid int, at time.Time, logHost string, q query, flags string, answers []string) string {
	return fmt.Sprintf("%s %s %s qid %d to %s#%d: [NOERROR %s] response: %s;",
		catalog.BigIPHeader(pri, hdr, host, tag, pid), at.Format(time.DateTime), logHost,
		q.ID, q.Client, q.Port, flags, strings.Join(answers, "; "))
}
