package flow

import (
	"math/rand"
	"net"
	"time"

	"github.com/observiq/blitz/embed"
	"github.com/observiq/blitz/flow/wire"
	"github.com/observiq/blitz/internal/datagen"
)

// Scenario names a canned traffic shape. Each preset controls the byte/packet
// size distribution and source/destination spread so the emitted flows
// resemble a recognizable environment.
type Scenario string

const (
	// ScenarioDefault is a broad mix of flow sizes.
	ScenarioDefault Scenario = "default"
	// ScenarioWANEdge is few, large, bidirectional flows (low rate).
	ScenarioWANEdge Scenario = "wan-edge"
	// ScenarioDatacenter is many small flows (high rate).
	ScenarioDatacenter Scenario = "datacenter"
	// ScenarioDDOSTarget skews many sources onto a single destination.
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
	minBytes, maxBytes     int64
	minPackets, maxPackets int64
	fixedDst               bool
}

func shapeFor(s Scenario) shape {
	switch s {
	case ScenarioWANEdge:
		return shape{minBytes: 100_000, maxBytes: 10_000_000, minPackets: 100, maxPackets: 10_000}
	case ScenarioDatacenter:
		return shape{minBytes: 40, maxBytes: 1_500, minPackets: 1, maxPackets: 10}
	case ScenarioDDOSTarget:
		return shape{minBytes: 40, maxBytes: 200, minPackets: 1, maxPackets: 3, fixedDst: true}
	default:
		return shape{minBytes: 100, maxBytes: 1_000_000, minPackets: 1, maxPackets: 1_000}
	}
}

// ddosDst is the fixed victim for the ddos-target scenario.
var ddosDst = net.IPv4(203, 0, 113, 7)

// generateFlow builds one FlowRecord shaped by the scenario, using r for all
// randomness so a fixed seed yields a deterministic stream.
func generateFlow(r *rand.Rand, s Scenario, now time.Time) embed.FlowRecord {
	sh := shapeFor(s)
	proto := []uint8{6, 17, 1}[r.Intn(3)] // TCP, UDP, ICMP

	dst := net.ParseIP(datagen.RandomPublicIPv4(r))
	dstPort := wire.U16(r.Intn(65535) + 1)
	if sh.fixedDst {
		dst = ddosDst
		dstPort = 443
	}

	bytes := wire.U64(sh.minBytes + r.Int63n(sh.maxBytes-sh.minBytes+1))
	pkts := wire.U64(sh.minPackets + r.Int63n(sh.maxPackets-sh.minPackets+1))
	dur := time.Duration(r.Intn(10_000)) * time.Millisecond

	return embed.FlowRecord{
		SrcIP:        net.ParseIP(datagen.RandomPublicIPv4(r)),
		DstIP:        dst,
		SrcPort:      wire.U16(r.Intn(65535) + 1),
		DstPort:      dstPort,
		Protocol:     proto,
		TOS:          0,
		TCPFlags:     wire.U8(r.Intn(256)),
		NextHop:      net.ParseIP(datagen.RandomPrivateIPv4(r)),
		InputIface:   wire.U32(r.Intn(48) + 1),
		OutputIface:  wire.U32(r.Intn(48) + 1),
		Packets:      pkts,
		Bytes:        bytes,
		StartTime:    now.Add(-dur),
		EndTime:      now,
		SrcAS:        wire.U32(r.Intn(65000) + 1),
		DstAS:        wire.U32(r.Intn(65000) + 1),
		SrcMask:      wire.U8(r.Intn(9) + 24),
		DstMask:      wire.U8(r.Intn(9) + 24),
		SamplingRate: 1024,
		Metadata:     embed.FlowRecordMetadata{Timestamp: now},
	}
}
