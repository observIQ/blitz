package afm

import (
	"encoding/csv"
	"math/rand"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/observiq/blitz/generator/f5/catalog"
)

// documentedOrder is the default ("None") storage format's field list,
// copied from F5's AFM logging guide (Local Logging with the Network
// Firewall, BIG-IP 14.1: "for example:" under Storage Format).
const documentedOrder = `"management_ip_address","bigip_hostname","context_type","context_name","src_ip","dest_ip","src_port","dest_port","vlan","protocol","route_domain","acl_rule_name","action","drop_reason`

// TestRenderRealEvent renders the values of a real AFM network event (BIG-IQ
// lab playback data, f5devcentral/f5-big-iq-lab afmEvent.txt) in the
// documented order, behind tmm's RFC 3164 header.
func TestRenderRealEvent(t *testing.T) {
	e := event{MgmtIP: "10.255.69.68", Host: "jitterbug.dmeast.acopianet.com", ContextType: "Virtual Server",
		ContextName: "/Common/vip1", SrcIP: "192.168.100.81", DestIP: "10.64.6.242", SrcPort: "60396",
		DestPort: "80", VLAN: "/Common/internal", Protocol: "TCP", RouteDomain: "0", Rule: "rule1", Action: "Accept"}
	now := time.Date(2018, 2, 20, 9, 28, 25, 0, time.UTC)
	want := `<134>Feb 20 09:28:25 jitterbug.dmeast.acopianet.com tmm[13301]: "10.255.69.68","jitterbug.dmeast.acopianet.com","Virtual Server","/Common/vip1","192.168.100.81","10.64.6.242","60396","80","/Common/internal","TCP","0","rule1","Accept",""`
	require.Equal(t, want, render(now, 13301, e))
}

func TestFieldOrderMatchesDocs(t *testing.T) {
	require.Equal(t, documentedOrder, `"`+strings.Join(fieldNames, `","`))
}

// TestBuildIsCoherent checks drawn events against the real value vocabulary.
func TestBuildIsCoherent(t *testing.T) {
	dev := catalog.NewDevice(rand.New(rand.NewSource(1)))
	c := &catalog.Ctx{Now: time.Date(2026, 9, 24, 15, 4, 5, 0, time.UTC), Hostname: "bigip1", Device: dev}
	actions := map[string]bool{}
	for seed := int64(0); seed < 300; seed++ {
		line := build(rand.New(rand.NewSource(seed)), c)
		prefix := "<134>Sep 24 15:04:05 bigip1 tmm[" + strconv.Itoa(dev.PID("tmm")) + "]: "
		require.True(t, strings.HasPrefix(line, prefix), line)
		rec, err := csv.NewReader(strings.NewReader(strings.TrimPrefix(line, prefix))).Read()
		require.NoError(t, err)
		require.Len(t, rec, 14)
		f := map[string]string{}
		for i, n := range fieldNames {
			f[n] = rec[i]
		}
		require.Equal(t, dev.MgmtIP, f["management_ip_address"])
		require.Contains(t, []string{"TCP", "UDP", "ICMP"}, f["protocol"])
		if f["protocol"] == "ICMP" {
			require.Empty(t, f["src_port"], "ICMP has no ports")
			require.Empty(t, f["dest_port"], "ICMP has no ports")
		} else {
			require.NotEmpty(t, f["src_port"])
		}
		switch f["context_type"] {
		case "Virtual Server":
			require.True(t, strings.HasPrefix(f["context_name"], "/Common/"), f["context_name"])
		case "Global":
			require.Empty(t, f["context_name"])
		default:
			t.Fatalf("unexpected context_type %q", f["context_type"])
		}
		switch f["action"] {
		case "Accept", "Accept Decisively":
			require.Empty(t, f["drop_reason"])
		case "Drop", "Reject":
			require.NotEmpty(t, f["drop_reason"])
		default:
			t.Fatalf("unexpected action %q", f["action"])
		}
		if f["action"] == "Accept Decisively" {
			require.Equal(t, "Global", f["context_type"], "Accept Decisively exists only in Global and Route Domain contexts")
		}
		if f["drop_reason"] == "Connection Flow Miss" {
			require.Empty(t, f["acl_rule_name"])
		}
		actions[f["action"]] = true
	}
	require.Len(t, actions, 4)
}

func TestRegistered(t *testing.T) {
	_, ok := catalog.Get("afm")
	require.True(t, ok)
}

// fieldNames is the documented default storage format order.
var fieldNames = []string{
	"management_ip_address", "bigip_hostname", "context_type", "context_name", "src_ip", "dest_ip",
	"src_port", "dest_port", "vlan", "protocol", "route_domain", "acl_rule_name", "action", "drop_reason",
}
