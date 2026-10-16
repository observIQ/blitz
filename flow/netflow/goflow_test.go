package netflow

import (
	"bytes"
	"testing"

	nf "github.com/netsampler/goflow2/v2/decoders/netflow"
	protoproducer "github.com/netsampler/goflow2/v2/producer/proto"
	"github.com/stretchr/testify/require"
)

// v9Collector decodes a stream of v9 packets the way the collector's
// netflowreceiver does: one template system and one sampling-rate system for
// the whole stream, then goflow2's proto producer per packet.
type v9Collector struct {
	t        *testing.T
	ts       nf.NetFlowTemplateSystem
	sampling protoproducer.SamplingRateSystem
}

func newV9Collector(t *testing.T) *v9Collector {
	return &v9Collector{t: t, ts: nf.CreateTemplateSystem(), sampling: protoproducer.CreateSamplingSystem()}
}

// decode returns the packet and the flow messages goflow2 produced from it.
func (c *v9Collector) decode(pkt []byte) (nf.NFv9Packet, []*protoproducer.ProtoProducerMessage) {
	c.t.Helper()
	var v9 nf.NFv9Packet
	var ipfix nf.IPFIXPacket
	require.NoError(c.t, nf.DecodeMessageVersion(bytes.NewBuffer(pkt), c.ts, &v9, &ipfix))
	msgs, err := protoproducer.ProcessMessageNetFlowV9Config(&v9, c.sampling, nil)
	require.NoError(c.t, err)
	out := make([]*protoproducer.ProtoProducerMessage, 0, len(msgs))
	for _, m := range msgs {
		out = append(out, m.(*protoproducer.ProtoProducerMessage))
	}
	return v9, out
}
