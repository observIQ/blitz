package flow

import (
	"math/rand"
	"net"
	"time"

	"github.com/observiq/blitz/embed"
	"github.com/observiq/blitz/flow/wire"
	"github.com/observiq/blitz/internal/datagen"
)

// Scenario names a canned traffic shape. Each preset controls the per-flow
// packet count and packet size distribution and the destination spread, so
// the emitted flows resemble a recognizable environment. It does not set the
// flow rate (that is the generator's workers and rate) or pair flows into
// request/response directions: each record is one unidirectional flow.
type Scenario string

const (
	// ScenarioDefault is a broad mix of flow sizes.
	ScenarioDefault Scenario = "default"
	// ScenarioWANEdge is long, large flows at a 1500-byte MTU.
	ScenarioWANEdge Scenario = "wan-edge"
	// ScenarioDatacenter is short flows of up to 10 packets, jumbo frames allowed.
	ScenarioDatacenter Scenario = "datacenter"
	// ScenarioDDOSTarget is tiny flood flows from many sources onto one destination.
	ScenarioDDOSTarget Scenario = "ddos-target"
)

// ValidScenario reports whether s is a known scenario.
func ValidScenario(s Scenario) bool {
	switch s {
	case ScenarioDefault, ScenarioWANEdge, ScenarioDatacenter, ScenarioDDOSTarget, "":
		return true
	default:
		return false
	}
}

// shape holds a scenario's tunable ranges. Ranges are int64 so the
// rand.Int63n span math needs no unsigned conversion; the final counts are
// widened to the FlowRecord's uint64 fields.
type shape struct {
	minPackets, maxPackets int64
	// maxPktSize caps the mean IP packet size in bytes (the MTU the scenario
	// runs at); the floor is the protocol's minimum packet, see minPktSize.
	maxPktSize int64
	fixedDst   bool
}

func shapeFor(s Scenario) shape {
	switch s {
	case ScenarioWANEdge:
		return shape{minPackets: 100, maxPackets: 10_000, maxPktSize: 1_500}
	case ScenarioDatacenter:
		return shape{minPackets: 1, maxPackets: 10, maxPktSize: 9_000} // jumbo frames
	case ScenarioDDOSTarget:
		return shape{minPackets: 1, maxPackets: 3, maxPktSize: 60, fixedDst: true} // small-packet flood
	default:
		return shape{minPackets: 1, maxPackets: 1_000, maxPktSize: 1_500}
	}
}

// IP protocol numbers.
const (
	protoICMP       = wire.ProtoICMP
	protoTCP  uint8 = 6
	protoUDP  uint8 = 17
)

// icmpMessages are the ICMP type*256+code values a flow carries in DstPort:
// echo request, echo reply, net/host/port unreachable, and time exceeded.
var icmpMessages = []uint16{8 << 8, 0, 3 << 8, 3<<8 | 1, 3<<8 | 3, 11 << 8}

// icmpEchoRequest is a ping, the ddos-target ICMP flood.
const icmpEchoRequest uint16 = 8 << 8

// TCP flag combinations, as the cumulative OR over a flow's packets.
const (
	flagsSYN uint8 = 0x02 // a SYN flood: handshakes that never complete
)

// minPktSize is the smallest IP packet the protocol can carry: a 20-byte IPv4
// header plus a 20-byte TCP or 8-byte UDP/ICMP header.
func minPktSize(proto uint8) int64 {
	if proto == protoTCP {
		return 40
	}
	return 28
}

// tcpFlags is the flow's cumulative TCP flags; only TCP carries them.
func tcpFlags(s Scenario, proto uint8, packets uint64) uint8 {
	switch {
	case proto != protoTCP:
		return 0
	case s == ScenarioDDOSTarget:
		return flagsSYN
	}
	// A connection's packets add SYN, ACK, PSH, FIN in that order, so the
	// cumulative flags are the first min(packets, 4) of them: a 1-packet
	// flow is a bare SYN, 4+ packets a full connection.
	var f uint8
	for i, flag := range connectionFlags {
		if uint64(i) >= packets {
			break
		}
		f |= flag
	}
	return f
}

// flagACK is the TCP ACK flag, and maxSYNSize a bare SYN: IPv4 and TCP
// headers with the full 40 bytes of TCP options.
const (
	flagACK    = 0x10
	maxSYNSize = 60
)

// connectionFlags are the flags a TCP connection's first packets carry, in
// order: SYN, ACK, PSH, FIN.
var connectionFlags = [...]uint8{0x02, 0x10, 0x08, 0x01}

// ephemeralPortMin starts the IANA dynamic (ephemeral) port range, 49152-65535
// (RFC 6335), that clients connect from.
const ephemeralPortMin = 49152

// weightedPort is a service port and its relative share of flows.
type weightedPort struct {
	port   uint16
	weight int
}

// tcpServices and udpServices are the services flows connect to, weighted
// toward the common ones: web (443, 80, 8080, 8443), SSH, mail (25, 993),
// RDP, SMB, and databases and caches over TCP; DNS, QUIC (443), NTP, SNMP,
// syslog, and IKE/IPsec NAT-T over UDP.
var (
	tcpServices = []weightedPort{
		{443, 40}, {80, 15}, {8080, 5}, {8443, 5}, {22, 8}, {445, 5}, {25, 4},
		{3389, 4}, {3306, 4}, {5432, 4}, {993, 3}, {6379, 3},
	}
	udpServices = []weightedPort{{53, 50}, {443, 20}, {123, 10}, {161, 5}, {514, 5}, {500, 5}, {4500, 5}}
)

// servicePort draws a destination port for proto from its service table. ICMP
// has no port; the caller encodes its type and code instead.
func servicePort(r *rand.Rand, proto uint8) uint16 {
	table := tcpServices
	if proto == protoUDP {
		table = udpServices
	}
	total := 0
	for _, p := range table {
		total += p.weight
	}
	n := r.Intn(total)
	for _, p := range table {
		if n < p.weight {
			return p.port
		}
		n -= p.weight
	}
	// Safety net: n < total, so the loop always returns.
	return table[len(table)-1].port
}

// samplingRate is the generator's 1-in-N packet sampling rate.
const samplingRate = 1024

// activeTimeout is the exporter's active flow timeout: a long-lived flow is
// exported (and its counters restarted) at least this often, so no record
// spans longer.
const activeTimeout = 60 * time.Second

// publicASN draws a public 16-bit AS number: 1-64495, skipping AS_TRANS
// 23456 (RFC 6793). Above 64495 sit documentation (RFC 5398) and private
// (RFC 6996) numbers.
func publicASN(r *rand.Rand) uint32 {
	return asnFromDraw(uint32(r.Intn(64494) + 1)) // #nosec G115 G404 -- 1..64494
}

// asnFromDraw maps a draw in 1..64494 onto the public ASNs, stepping over
// AS_TRANS.
func asnFromDraw(as uint32) uint32 {
	if as >= 23456 {
		as++
	}
	return as
}

// host is one end of a flow: its address and its AS number.
type host struct {
	ip net.IP
	as uint32
}

// endpoint draws one end of a flow: a private (RFC 1918) host, which has no
// AS number (0), or a public host with its public AS number.
func endpoint(r *rand.Rand, private bool) host {
	if private {
		return host{ip: net.ParseIP(datagen.RandomPrivateIPv4(r))}
	}
	return host{ip: net.ParseIP(datagen.RandomPublicIPv4(r)), as: publicASN(r)}
}

// ifaces is how many interfaces the simulated router has.
const ifaces = 48

// nextHop is the one neighbor on egress interface out: each interface is a
// point-to-point /30 in 10.255.0.0/16, and the neighbor holds its .2.
func nextHop(out int) net.IP {
	return net.IPv4(10, 255, byte(out), 2) // #nosec G115 -- out is 1..48
}

// drawProto draws a flow's protocol in a typical network mix: mostly TCP,
// some UDP, and a little ICMP.
func drawProto(r *rand.Rand) uint8 {
	switch n := r.Intn(20); {
	case n < 16:
		return protoTCP
	case n < 19:
		return protoUDP
	default:
		return protoICMP
	}
}

// generateFlow builds one FlowRecord shaped by the scenario, using r for all
// randomness so a fixed seed yields a deterministic stream. victim is the
// ddos-target scenario's one destination.
func generateFlow(r *rand.Rand, s Scenario, now time.Time, victim host) embed.FlowRecord {
	sh := shapeFor(s)
	proto := drawProto(r)

	// Every generated flow is IPv4: the scenarios have no IP-family option.
	// The encoders already handle IPv6 (v9/IPFIX template 257, sFlow
	// sampled_ipv6), and datagen.RandomPublicIPv6 can supply addresses.
	// Datacenter flows run east-west between private hosts, and ddos-target
	// flows public to public. The rest cross the edge, one private host and
	// one public, in either direction.
	var srcPrivate, dstPrivate bool
	switch s {
	case ScenarioDatacenter:
		srcPrivate, dstPrivate = true, true
	case ScenarioDDOSTarget:
	default:
		srcPrivate = r.Intn(2) == 0
		dstPrivate = !srcPrivate
	}
	src := endpoint(r, srcPrivate)
	dst := endpoint(r, dstPrivate)
	srcPort := wire.U16(ephemeralPortMin + r.Intn(65536-ephemeralPortMin))
	dstPort := servicePort(r, proto)
	if sh.fixedDst {
		dst = victim
		dstPort = 443
	}
	if proto == protoICMP {
		// ICMP has no ports: SrcPort 0, DstPort type*256+code (NetFlow v5).
		srcPort = 0
		dstPort = icmpMessages[r.Intn(len(icmpMessages))]
		if sh.fixedDst {
			dstPort = icmpEchoRequest
		}
	}

	// Draw packets, then a mean packet size the protocol and scenario allow,
	// so bytes is always a physical total (never under a header per packet
	// or over the MTU).
	pkts := sh.minPackets + r.Int63n(sh.maxPackets-sh.minPackets+1)
	flags := tcpFlags(s, proto, wire.U64(pkts))
	minSize, maxSize := minPktSize(proto), sh.maxPktSize
	if proto == protoTCP && flags&flagACK == 0 {
		// Packets that never carried ACK are handshake SYNs, which carry no
		// data: headers and TCP options only.
		maxSize = min(maxSize, maxSYNSize)
	}
	size := minSize + r.Int63n(maxSize-minSize+1)
	bytes := wire.U64(pkts * size)
	// pkts are samples, each standing for samplingRate packets on the wire.
	// Those arrive 1-10ms apart, so a flow lasts as long as they span, up to
	// the active timeout.
	gap := time.Duration(1+r.Intn(10)) * time.Millisecond
	dur := min(time.Duration(pkts*samplingRate-1)*gap, activeTimeout)

	// A router forwards out another of its 48 interfaces than the flow came in on.
	in := r.Intn(ifaces) + 1
	out := (in+r.Intn(ifaces-1))%ifaces + 1

	return embed.FlowRecord{
		SrcIP:        src.ip,
		DstIP:        dst.ip,
		SrcPort:      srcPort,
		DstPort:      dstPort,
		Protocol:     proto,
		TOS:          0,
		TCPFlags:     flags,
		NextHop:      nextHop(out),
		InputIface:   wire.U32(in),
		OutputIface:  wire.U32(out),
		Packets:      wire.U64(pkts),
		Bytes:        bytes,
		StartTime:    now.Add(-dur),
		EndTime:      now,
		SrcAS:        src.as,
		DstAS:        dst.as,
		SrcMask:      wire.U8(r.Intn(9) + 24),
		DstMask:      wire.U8(r.Intn(9) + 24),
		SamplingRate: samplingRate,
		Metadata:     embed.FlowRecordMetadata{Timestamp: now},
	}
}
