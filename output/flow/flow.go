// Package flow is the flow output: it consumes FlowRecords, encodes them into
// the configured wire format (NetFlow v5/v9, IPFIX, or sFlow), and pushes the
// packets over UDP — the transport every flow collector expects.
package flow

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/observiq/blitz/embed"
	"github.com/observiq/blitz/flow/ipfix"
	"github.com/observiq/blitz/flow/netflow"
	"github.com/observiq/blitz/flow/sflow"
	"github.com/observiq/blitz/flow/wire"
	"github.com/observiq/blitz/output"
	"github.com/observiq/blitz/telemetry"
	"go.uber.org/zap"
)

// Protocol is a flow wire format.
type Protocol string

const (
	// ProtocolNetFlowV5 is fixed-layout NetFlow v5.
	ProtocolNetFlowV5 Protocol = "netflow-v5"
	// ProtocolNetFlowV9 is template-based NetFlow v9.
	ProtocolNetFlowV9 Protocol = "netflow-v9"
	// ProtocolIPFIX is IPFIX (RFC 7011).
	ProtocolIPFIX Protocol = "ipfix"
	// ProtocolSFlow is sFlow v5.
	ProtocolSFlow Protocol = "sflow"
)

// encoder is the format-agnostic surface the output drives. Each wire encoder
// packs a batch of records into one or more UDP payloads.
type encoder interface {
	encode(now time.Time, flows []embed.FlowRecord) ([][]byte, error)
	// resendTemplates makes the next packet carry the templates again
	// (a no-op for a format without them).
	resendTemplates()
}

// streamer hands each packet to send as it is encoded, for a format (sFlow)
// whose output grows with a record's packet count. An Output has an encoder
// or a streamer.
type streamer interface {
	encodeTo(ctx context.Context, now time.Time, flows []embed.FlowRecord, send func([]byte) error) error
}

// Output implements embed.FlowConsumer: flows are its only signal.
type Output struct {
	logger  *zap.Logger
	conn    net.Conn
	metrics *output.Metrics

	// mu spans encode and send, so packets leave in the order they were
	// encoded: a template packet always precedes the data that needs it, even
	// with several generator workers calling ConsumeFlows at once.
	mu     sync.Mutex
	enc    encoder
	stream streamer
}

// ipfixVendors maps a vendor flavor to the enterprise IE that marks its IPFIX
// export. Only vendors with a published, decoder-known IE (Wireshark
// epan/dissectors/packet-netflow.c) carry one: Juniper jFlow (PEN 2636) and
// Citrix AppFlow (NetScaler, PEN 5951). The other flavors (netstream, cflowd,
// rflow) have no such IE, so their export is wire-standard, as is every
// flavor on NetFlow v5 and v9, which have no vendor mechanism.
var ipfixVendors = map[string]*ipfix.Vendor{
	"jflow":   ipfix.JuniperCommonProperties,
	"appflow": ipfix.CitrixConnectionID,
}

// New dials the collector at host:port over UDP and builds the encoder for
// protocol (empty means ProtocolNetFlowV9, the config default). vendor
// (optional) adds the vendor's IPFIX enterprise IE where one exists.
// agentIP is the exporter address reported by sFlow (ignored by the
// NetFlow/IPFIX formats); empty reports each record's ExporterIP.
func New(logger *zap.Logger, host, port string, protocol Protocol, vendor, agentIP string, tel embed.TelemetrySettings) (*Output, error) {
	if logger == nil {
		return nil, fmt.Errorf("logger cannot be nil")
	}
	if agentIP != "" && net.ParseIP(agentIP) == nil {
		return nil, fmt.Errorf("flow output: agentIP must be an IPv4 or IPv6 address, got %q", agentIP)
	}
	conn, err := net.Dial("udp", net.JoinHostPort(host, port))
	if err != nil {
		return nil, fmt.Errorf("flow output: dial %s:%s: %w", host, port, err)
	}
	m, err := output.NewMetrics(tel.MeterProvider)
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("flow output: build metrics: %w", err)
	}

	boot := wire.Boot(time.Now())
	var enc encoder
	var stream streamer
	if protocol == "" {
		protocol = ProtocolNetFlowV9
	}
	switch protocol {
	case ProtocolNetFlowV5:
		enc = v5enc{netflow.NewV5Encoder(boot)}
	case ProtocolNetFlowV9:
		enc = v9enc{netflow.NewV9Encoder(boot, 0)}
	case ProtocolIPFIX:
		enc = ipfixenc{ipfix.NewEncoder(0, ipfixVendors[vendor])}
	case ProtocolSFlow:
		// An unset agentIP (nil) reports each record's exporter.
		stream = sflowenc{sflow.NewEncoder(net.ParseIP(agentIP), 0, boot)}
	default:
		_ = conn.Close()
		return nil, fmt.Errorf("flow output: unknown protocol %q", protocol)
	}

	return &Output{logger: logger, conn: conn, enc: enc, stream: stream, metrics: m}, nil
}

// ConsumeFlows encodes the batch and sends each packet over UDP.
func (o *Output) ConsumeFlows(ctx context.Context, records []embed.FlowRecord) error {
	o.metrics.BlitzOutputEntriesReceivedCounter.Add(ctx, int64(len(records)), outputType, "flows")
	o.mu.Lock()
	defer o.mu.Unlock()
	send := func(pkt []byte) error {
		if _, err := o.conn.Write(pkt); err != nil {
			o.metrics.BlitzOutputSendErrorsCounter.Add(ctx, 1, outputType, "flows")
			return fmt.Errorf("flow output: send: %w", err)
		}
		return nil
	}
	if o.stream != nil {
		if err := o.stream.encodeTo(ctx, time.Now(), records, send); err != nil {
			return err
		}
	} else {
		pkts, err := o.enc.encode(time.Now(), records)
		if err != nil {
			return fmt.Errorf("flow output: %w", err)
		}
		for _, pkt := range pkts {
			if err := send(pkt); err != nil {
				// The failed packet may have carried the templates; without
				// them the collector drops every data packet that follows
				// until the next scheduled resend.
				o.enc.resendTemplates()
				return err
			}
		}
	}
	return nil
}

// SupportedTelemetry reports that only flows are consumed.
func (o *Output) SupportedTelemetry() []telemetry.Type {
	return []telemetry.Type{telemetry.Flows}
}

// Stop closes the UDP socket.
func (o *Output) Stop(_ context.Context) error {
	return o.conn.Close()
}

const outputType = "flow"

// encoder adapters: the wire encoders share a shape but not an interface.
type v5enc struct{ e *netflow.V5Encoder }

func (a v5enc) encode(now time.Time, f []embed.FlowRecord) ([][]byte, error) {
	return a.e.Encode(now, f)
}

func (v5enc) resendTemplates() {}

type v9enc struct{ e *netflow.V9Encoder }

func (a v9enc) encode(now time.Time, f []embed.FlowRecord) ([][]byte, error) {
	return a.e.Encode(now, f), nil
}

func (a v9enc) resendTemplates() { a.e.ResendTemplates() }

type ipfixenc struct{ e *ipfix.Encoder }

func (a ipfixenc) encode(now time.Time, f []embed.FlowRecord) ([][]byte, error) {
	return a.e.Encode(now, f), nil
}

func (a ipfixenc) resendTemplates() { a.e.ResendTemplates() }

type sflowenc struct{ e *sflow.Encoder }

func (a sflowenc) encodeTo(ctx context.Context, now time.Time, f []embed.FlowRecord, send func([]byte) error) error {
	return a.e.EncodeTo(ctx, now, f, send)
}
