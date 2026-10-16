package flow

import (
	"math/rand"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func meanBytes(s Scenario, seed int64, n int) float64 {
	r := rand.New(rand.NewSource(seed))
	var sum uint64
	for i := 0; i < n; i++ {
		sum += generateFlow(r, s, time.Unix(1700000000, 0), testVictim).Bytes
	}
	return float64(sum) / float64(n)
}

func TestWANEdgeFlowsLargerThanDatacenter(t *testing.T) {
	require.Greater(t, meanBytes(ScenarioWANEdge, 1, 500), meanBytes(ScenarioDatacenter, 1, 500))
}

// TestDDOSTargetSharesDestination checks every ddos-target flow lands on the
// one victim host: its address and its AS number.
func TestDDOSTargetSharesDestination(t *testing.T) {
	r := rand.New(rand.NewSource(1))
	first := generateFlow(r, ScenarioDDOSTarget, time.Unix(1700000000, 0), testVictim)
	for i := 0; i < 100; i++ {
		f := generateFlow(r, ScenarioDDOSTarget, time.Unix(1700000000, 0), testVictim)
		require.True(t, f.DstIP.Equal(first.DstIP))
		require.Equal(t, first.DstAS, f.DstAS)
	}
}

func TestDeterministicFromSeed(t *testing.T) {
	a := generateFlow(rand.New(rand.NewSource(42)), ScenarioDefault, time.Unix(1700000000, 0), testVictim)
	b := generateFlow(rand.New(rand.NewSource(42)), ScenarioDefault, time.Unix(1700000000, 0), testVictim)
	require.Equal(t, a, b)
}

// TestFlowShapesArePhysical checks every generated flow is a real traffic
// shape: a whole number of packets, each between the protocol's minimum IP
// packet and the scenario's maximum, and TCP flags only on TCP.
func TestFlowShapesArePhysical(t *testing.T) {
	maxSize := map[Scenario]uint64{
		ScenarioDefault: 1500, ScenarioWANEdge: 1500, ScenarioDatacenter: 9000, ScenarioDDOSTarget: 60,
	}
	for s, hi := range maxSize {
		for seed := int64(0); seed < 50; seed++ {
			r := rand.New(rand.NewSource(seed))
			for i := 0; i < 200; i++ {
				f := generateFlow(r, s, time.Unix(1700000000, 0), testVictim)
				require.GreaterOrEqual(t, f.Packets, uint64(1), "scenario %s", s)
				require.Zero(t, f.Bytes%f.Packets, "scenario %s: bytes %d not a whole number of %d packets", s, f.Bytes, f.Packets)
				size := f.Bytes / f.Packets
				lo := uint64(28) // IPv4 + UDP/ICMP header
				if f.Protocol == 6 {
					lo = 40 // IPv4 + TCP header
				}
				require.GreaterOrEqual(t, size, lo, "scenario %s proto %d", s, f.Protocol)
				require.LessOrEqual(t, size, hi, "scenario %s proto %d", s, f.Protocol)

				switch {
				case f.Protocol != 6:
					require.Zero(t, f.TCPFlags, "scenario %s proto %d carries TCP flags", s, f.Protocol)
				case s == ScenarioDDOSTarget:
					require.Equal(t, uint8(0x02), f.TCPFlags, "ddos TCP flows are SYN-only")
				default:
					require.NotZero(t, f.TCPFlags&0x02, "scenario %s: a TCP flow opens with SYN", s) // exact set: TestTCPFlagsFitPacketCount
				}
			}
		}
	}
}

// TestICMPFlowsCarryTypeCode checks ICMP flows follow the NetFlow convention:
// no source port, and the destination port carries ICMP type*256+code drawn
// from real messages (a ping flood for ddos-target).
// TestProtocolMixFavorsTCP pins the traffic mix: mostly TCP, some UDP, and a
// little ICMP, as on a real network.
func TestProtocolMixFavorsTCP(t *testing.T) {
	for _, s := range []Scenario{ScenarioDefault, ScenarioWANEdge, ScenarioDatacenter, ScenarioDDOSTarget} {
		r := rand.New(rand.NewSource(3))
		n := map[uint8]int{}
		for i := 0; i < 4000; i++ {
			n[generateFlow(r, s, time.Unix(1700000000, 0), testVictim).Protocol]++
		}
		require.Greater(t, n[6], 4000*7/10, "scenario %s: TCP share", s)
		require.Greater(t, n[17], 4000/20, "scenario %s: UDP share", s)
		require.Less(t, n[1], 4000/10, "scenario %s: ICMP share", s)
	}
}

// TestSYNOnlyFlowsAreHeaderSized pins that a TCP flow whose packets never
// carried ACK (a bare handshake or a SYN flood) is made of header-only
// packets, at most 60 bytes each.
func TestSYNOnlyFlowsAreHeaderSized(t *testing.T) {
	for _, s := range []Scenario{ScenarioDefault, ScenarioWANEdge, ScenarioDatacenter, ScenarioDDOSTarget} {
		r := rand.New(rand.NewSource(11))
		seen := 0
		for i := 0; i < 4000; i++ {
			f := generateFlow(r, s, time.Unix(1700000000, 0), testVictim)
			if f.Protocol != 6 || f.TCPFlags&0x10 != 0 {
				continue
			}
			seen++
			require.LessOrEqual(t, f.Bytes/f.Packets, uint64(60), "scenario %s: SYN-only flow mean size", s)
		}
		if s != ScenarioWANEdge { // wan-edge flows run 100+ packets, so always reach ACK
			require.Positive(t, seen, "scenario %s produced no SYN-only flows", s)
		}
	}
}

func TestICMPFlowsCarryTypeCode(t *testing.T) {
	allowed := map[uint16]bool{8 << 8: true, 0: true, 3 << 8: true, 3<<8 | 1: true, 3<<8 | 3: true, 11 << 8: true}
	for _, s := range []Scenario{ScenarioDefault, ScenarioWANEdge, ScenarioDatacenter, ScenarioDDOSTarget} {
		r := rand.New(rand.NewSource(7))
		seen := 0
		for i := 0; i < 2000; i++ {
			f := generateFlow(r, s, time.Unix(1700000000, 0), testVictim)
			if f.Protocol != 1 {
				continue
			}
			seen++
			require.Zero(t, f.SrcPort, "scenario %s: ICMP has no source port", s)
			if s == ScenarioDDOSTarget {
				require.Equal(t, uint16(8<<8), f.DstPort, "ddos ICMP is an echo-request flood")
			} else {
				require.True(t, allowed[f.DstPort], "scenario %s: unexpected ICMP type/code 0x%04x", s, f.DstPort)
			}
		}
		require.Positive(t, seen, "scenario %s produced no ICMP flows", s)
	}
}

// TestTCPFlagsFitPacketCount pins that a flow's cumulative TCP flags are
// ones its packets could carry: each packet adds at most one of SYN, ACK,
// PSH, FIN in connection order, so a 2-packet flow cannot have PSH or FIN.
func TestTCPFlagsFitPacketCount(t *testing.T) {
	order := []uint8{0x02, 0x10, 0x08, 0x01} // SYN, ACK, PSH, FIN
	for _, s := range []Scenario{ScenarioDefault, ScenarioWANEdge, ScenarioDatacenter} {
		r := rand.New(rand.NewSource(1))
		for i := 0; i < 5000; i++ {
			f := generateFlow(r, s, time.Unix(1700000000, 0), testVictim)
			if f.Protocol != 6 {
				continue
			}
			var want uint8
			for k := 0; k < len(order) && uint64(k) < f.Packets; k++ {
				want |= order[k]
			}
			require.Equal(t, want, f.TCPFlags, "%s flow with %d packets", s, f.Packets)
		}
	}
}

// TestFlowDurationFitsPacketCount checks a flow's duration is one its real
// packets can span. Packets are sampled 1 in SamplingRate, so a record of n
// samples stands for n x SamplingRate packets on the wire, at least a
// millisecond apart (the NetFlow v5 timestamp resolution), up to the active
// timeout. The scaled throughput then stays under one 10 Gbps link.
func TestFlowDurationFitsPacketCount(t *testing.T) {
	for _, s := range []Scenario{ScenarioDefault, ScenarioWANEdge, ScenarioDatacenter, ScenarioDDOSTarget} {
		r := rand.New(rand.NewSource(1))
		for i := 0; i < 2000; i++ {
			f := generateFlow(r, s, time.Unix(1700000000, 0), testVictim)
			dur := f.EndTime.Sub(f.StartTime)
			onWire := f.Packets * uint64(f.SamplingRate)
			require.GreaterOrEqual(t, dur, min(time.Duration(onWire-1)*time.Millisecond, activeTimeout), "scenario %s: %d packets in %s", s, onWire, dur)
			require.LessOrEqual(t, dur, activeTimeout, "scenario %s", s)
			bps := float64(f.Bytes*uint64(f.SamplingRate)*8) / dur.Seconds()
			require.LessOrEqual(t, bps, 10e9, "scenario %s: %d bytes in %s", s, f.Bytes, dur)
		}
	}
}

// TestFlowPortsLookLikeServices checks TCP/UDP flows run from an ephemeral
// client port (IANA dynamic range, RFC 6335) to a service port, so traffic
// concentrates on real services (443 over TCP, 53 over UDP) instead of
// looking like a port scan.
func TestFlowPortsLookLikeServices(t *testing.T) {
	r := rand.New(rand.NewSource(1))
	count := map[uint8]map[uint16]int{6: {}, 17: {}}
	for i := 0; i < 5000; i++ {
		f := generateFlow(r, ScenarioDefault, time.Unix(1700000000, 0), testVictim)
		if f.Protocol != 6 && f.Protocol != 17 {
			continue
		}
		require.GreaterOrEqual(t, f.SrcPort, uint16(49152), "client port is ephemeral")
		count[f.Protocol][f.DstPort]++
	}
	top := func(m map[uint16]int) uint16 {
		var best uint16
		for p, n := range m {
			if n > m[best] {
				best = p
			}
		}
		return best
	}
	require.Equal(t, uint16(443), top(count[6]), "HTTPS leads TCP")
	require.Equal(t, uint16(53), top(count[17]), "DNS leads UDP")
	require.Less(t, len(count[6]), 50, "TCP flows hit a service set, not every port")
}

// testVictim is the ddos-target victim the scenario tests pass in.
var testVictim = host{ip: net.IPv4(93, 184, 216, 34), as: 15133}

// TestFlowEndpointsFitTheScenario checks where each scenario's flows run.
// Datacenter flows are east-west between private (RFC 1918) hosts. Default
// and wan-edge flows cross the edge, one private host and one public, in
// either direction. ddos-target flows run public to public. A private host
// has no AS (0); a public one has a public 16-bit AS number: never AS_TRANS
// 23456 (RFC 6793), the documentation block 64496-64511 (RFC 5398), or
// private use 64512-65534 (RFC 6996).
func TestFlowEndpointsFitTheScenario(t *testing.T) {
	publicAS := func(as uint32) bool { return as >= 1 && as <= 64495 && as != 23456 }
	for s, wantPrivate := range map[Scenario]int{ScenarioDatacenter: 2, ScenarioDefault: 1, ScenarioWANEdge: 1, ScenarioDDOSTarget: 0} {
		r := rand.New(rand.NewSource(1))
		privateSrc := 0
		for i := 0; i < 20000; i++ {
			f := generateFlow(r, s, time.Unix(1700000000, 0), testVictim)
			private := 0
			for _, end := range []struct {
				ip net.IP
				as uint32
			}{{f.SrcIP, f.SrcAS}, {f.DstIP, f.DstAS}} {
				if end.ip.IsPrivate() {
					private++
					require.Zero(t, end.as, "scenario %s: private %s has AS %d", s, end.ip, end.as)
				} else {
					require.True(t, publicAS(end.as), "scenario %s: public %s has AS %d", s, end.ip, end.as)
				}
			}
			require.Equal(t, wantPrivate, private, "scenario %s: %s -> %s", s, f.SrcIP, f.DstIP)
			if f.SrcIP.IsPrivate() {
				privateSrc++
			}
		}
		if wantPrivate == 1 {
			require.InDelta(t, 10000, privateSrc, 500, "scenario %s: edge flows run both ways", s)
		}
	}
}

// TestFlowsSampleOneIn1024 pins the generator's packet sampling rate, which
// collectors use to scale packet and byte counts: every scenario's records
// are sampled 1 in 1024, as the docs state.
func TestFlowsSampleOneIn1024(t *testing.T) {
	r := rand.New(rand.NewSource(1))
	for _, s := range []Scenario{ScenarioDefault, ScenarioWANEdge, ScenarioDatacenter, ScenarioDDOSTarget} {
		for range 50 {
			require.Equal(t, uint32(1024), uint32(generateFlow(r, s, time.Unix(1700000000, 0), testVictim).SamplingRate), s)
		}
	}
}

// TestPublicASNSkipsASTrans pins the draw's mapping at its edges: draws below
// AS_TRANS keep their value, AS_TRANS (23456) and above shift up by one, so
// the range 1..64494 covers 1..64495 without 23456.
func TestPublicASNSkipsASTrans(t *testing.T) {
	for draw, want := range map[uint32]uint32{1: 1, 23455: 23455, 23456: 23457, 64494: 64495} {
		require.Equal(t, want, asnFromDraw(draw), "draw %d", draw)
	}
}

// TestFlowsRouteThroughTheirEgressInterface pins router-shaped forwarding: a
// flow leaves on another interface than it arrived on, and its next hop is
// the one neighbor on that egress interface.
func TestFlowsRouteThroughTheirEgressInterface(t *testing.T) {
	r := rand.New(rand.NewSource(1)) // #nosec G404 -- test
	hops := map[uint32]string{}
	for range 5000 {
		f := generateFlow(r, ScenarioDefault, time.Unix(1700000000, 0), testVictim)
		require.NotEqual(t, f.InputIface, f.OutputIface)
		if hop, ok := hops[f.OutputIface]; ok {
			require.Equal(t, hop, f.NextHop.String(), "interface %d", f.OutputIface)
		}
		hops[f.OutputIface] = f.NextHop.String()
	}
}
