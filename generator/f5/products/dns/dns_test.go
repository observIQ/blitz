package dns

import (
	"math/rand"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/observiq/blitz/generator/f5/catalog"
)

// golden holds real BIG-IP DNS logging-profile lines from F5's DNS lab
// (clouddocs.f5.com/training/community/dns/html/class2/module7/lab3.html),
// read from the local syslog file, so without their <PRI>.
func golden(t *testing.T) []string {
	t.Helper()
	b, err := os.ReadFile("testdata/golden.log")
	require.NoError(t, err)
	return strings.Split(strings.TrimRight(string(b), "\n"), "\n")
}

func TestRenderMatchesGolden(t *testing.T) {
	g := golden(t)
	host, fqdn := "ip-10-1-1-4", "ip-10-1-1-4.us-west-2.compute.internal"
	q := query{ID: 29530, Client: "10.1.10.4", Port: 43881, Name: "www.f5.com", Type: "A", Server: "10.1.10.53"}
	require.Equal(t, "<134>"+g[0], renderQuery(time.Date(2020, 2, 13, 14, 47, 14, 0, time.UTC), host, "tmm", 10647,
		time.Date(2020, 2, 13, 14, 47, 13, 0, time.UTC), fqdn, q))
	require.Equal(t, "<134>"+g[1], renderResponse(time.Date(2020, 2, 13, 14, 47, 14, 0, time.UTC), host, "tmm", 10647,
		time.Date(2020, 2, 13, 14, 47, 14, 0, time.UTC), fqdn, q, "qr,rd,ra", []string{
			"www.f5.com. 30 IN CNAME dwbfwz8xncgmg.cloudfront.net",
			"dwbfwz8xncgmg.cloudfront.net. 60 IN A 99.86.33.52",
			"dwbfwz8xncgmg.cloudfront.net. 60 IN A 99.86.33.5",
			"dwbfwz8xncgmg.cloudfront.net. 60 IN A 99.86.33.9",
			"dwbfwz8xncgmg.cloudfront.net. 60 IN A 99.86.33.53",
		}))
	q2 := query{ID: 32427, Client: "10.1.10.4", Port: 55151, Name: "www.tmz.com", Type: "A"}
	require.Equal(t, "<134>"+g[2], renderResponse(time.Date(2020, 2, 13, 15, 27, 37, 0, time.UTC), host, "tmm", 10647,
		time.Date(2020, 2, 13, 15, 27, 36, 0, time.UTC), fqdn, q2, "qr,rd,ad", []string{"www.tmz.com. 111 IN A 10.1.20.252"}))
}

var (
	queryRe = regexp.MustCompile(`^<134>\w{3} [ \d]\d \d\d:\d\d:\d\d bigip1 info tmm\d?\[(\d+)\]: \d{4}-\d\d-\d\d \d\d:\d\d:\d\d bigip1 qid (\d+) from (\S+)#(\d+): view none: query: (\S+) IN (A|AAAA|CNAME) \+E \((\S+)%0\)$`)
	respRe  = regexp.MustCompile(`^<134>\w{3} [ \d]\d \d\d:\d\d:\d\d bigip1 info tmm\d?\[(\d+)\]: \d{4}-\d\d-\d\d \d\d:\d\d:\d\d bigip1 qid (\d+) to (\S+)#(\d+): \[NOERROR qr,rd,ra\] response: (\S+)\. \d+ IN .+;$`)
)

// TestQueriesPairWithResponses checks each response answers an earlier query
// from the same worker: same qid, client, port, and name.
func TestQueriesPairWithResponses(t *testing.T) {
	dev := catalog.NewDevice(rand.New(rand.NewSource(4)))
	c := &catalog.Ctx{Now: time.Date(2026, 9, 24, 15, 4, 5, 0, time.UTC), Hostname: "bigip1", Device: dev}
	r := rand.New(rand.NewSource(11))
	open := map[string]string{}
	responses := 0
	for i := 0; i < 300; i++ {
		line := build(r, c)
		if m := queryRe.FindStringSubmatch(line); m != nil {
			open[m[2]] = m[3] + "#" + m[4] + " " + m[5]
			continue
		}
		m := respRe.FindStringSubmatch(line)
		require.NotNil(t, m, line)
		want, ok := open[m[2]]
		require.True(t, ok, "response qid %s has no query", m[2])
		require.Equal(t, want, m[3]+"#"+m[4]+" "+m[5])
		delete(open, m[2])
		responses++
	}
	require.Greater(t, responses, 50)
}

func TestRegistered(t *testing.T) {
	_, ok := catalog.Get("dns")
	require.True(t, ok)
}
