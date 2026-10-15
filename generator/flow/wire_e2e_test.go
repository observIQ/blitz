package flow

import (
	"bytes"
	"encoding/binary"
	"math/rand"
	"testing"
	"time"

	nf "github.com/netsampler/goflow2/v2/decoders/netflow"
	"github.com/observiq/blitz/embed"
	"github.com/observiq/blitz/flow/netflow"
	"github.com/observiq/blitz/flow/wire"
	"github.com/stretchr/testify/require"
)

// TestGeneratedFlowsPostDateExporterBoot runs generateFlow through the v9
// encoder with the exporter boot the flow output uses, and checks every flow's
// switched times sit after boot and before the packet's sysUptime.
func TestGeneratedFlowsPostDateExporterBoot(t *testing.T) {
	now := time.Unix(1700000000, 0)
	enc := netflow.NewV9Encoder(wire.Boot(now), 0)
	r := rand.New(rand.NewSource(3))
	ts := nf.CreateTemplateSystem()
	for i := 0; i < 200; i++ {
		f := generateFlow(r, ScenarioDefault, now, testVictim)
		pkt := enc.Encode(now, []embed.FlowRecord{f})[0]
		var v9 nf.NFv9Packet
		var ipfix nf.IPFIXPacket
		require.NoError(t, nf.DecodeMessageVersion(bytes.NewBuffer(pkt), ts, &v9, &ipfix))
		for _, fs := range v9.FlowSets {
			d, ok := fs.(nf.DataFlowSet)
			if !ok {
				continue
			}
			for _, rec := range d.Records {
				var first, last uint32
				for _, v := range rec.Values {
					b, _ := v.Value.([]byte)
					switch v.Type {
					case 22:
						first = binary.BigEndian.Uint32(b)
					case 21:
						last = binary.BigEndian.Uint32(b)
					}
				}
				require.Positive(t, first, "FIRST_SWITCHED after boot")
				require.LessOrEqual(t, first, last)
				require.LessOrEqual(t, last, v9.SystemUptime)
				require.GreaterOrEqual(t, first, v9.SystemUptime-uint32(activeTimeout.Milliseconds()), "start within the active timeout")
			}
		}
	}
}
