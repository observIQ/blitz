// Package flow is the flow output: it consumes FlowRecords, encodes them into
// the configured wire format (NetFlow v5/v9, IPFIX, or sFlow), and pushes the
// packets over UDP — the transport every flow collector expects.
package flow

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/observiq/blitz/embed"
	"github.com/observiq/blitz/flow/ipfix"
	"github.com/observiq/blitz/flow/netflow"
	"github.com/observiq/blitz/flow/sflow"
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
	encode(now time.Time, flows []embed.FlowRecord) [][]byte
}

// Output implements output.Output (metrics-adjacent: flows only) and
// embed.FlowConsumer. Write returns ErrUnsupportedTelemetryType because flows
// are delivered via ConsumeFlows, not the log Write path.
type Output struct {
	logger  *zap.Logger
	conn    *net.UDPConn
	enc     encoder
	metrics *output.Metrics
}

// vendorSpec is how a vendor flavor diverges on the wire, per format. A vendor
// may support several formats (see config.flowVendorFormats); the encoder picks
// the signature matching the configured protocol:
//   - IPFIX  → an enterprise IE carrying the vendor's IANA PEN (pen/ieid).
//   - NetFlow v9 → a proprietary field type in the template (v9Field).
//   - NetFlow v5 → no signature: v5 is a fixed layout with no vendor mechanism,
//     so a v5 export is byte-identical across vendors (real protocol behavior).
//
// pen/v9Field are zero for a format the vendor does not carry a mark in.
type vendorSpec struct {
	pen     uint32 // IPFIX enterprise PEN
	ieid    uint16 // IPFIX enterprise element id
	v9Field uint16 // NetFlow v9 proprietary field type
}

// vendorSpecs holds each vendor's per-format signatures, grounded in the
// vendor's real IANA Private Enterprise Number. AppFlow is IPFIX-only, so it has
// no v9 field; the v5-only path carries no mark for any vendor.
var vendorSpecs = map[string]vendorSpec{
	"jflow":     {pen: 2636, ieid: 137, v9Field: 0x9003}, // Juniper
	"netstream": {pen: 2011, ieid: 140, v9Field: 0x9001}, // Huawei
	"cflowd":    {pen: 6527, ieid: 91, v9Field: 0x9004},  // Nokia/Alcatel-Lucent
	"appflow":   {pen: 5951, ieid: 130},                  // Citrix (IPFIX only)
	"rflow":     {v9Field: 0x9002},                       // Redback (v5/v9; no PEN)
}

// ipfixVendor returns the IPFIX enterprise IE for a vendor, or nil when the
// vendor carries no IPFIX PEN.
func (s vendorSpec) ipfixVendor() *ipfix.Vendor {
	if s.pen == 0 {
		return nil
	}
	return &ipfix.Vendor{PEN: s.pen, IEID: s.ieid}
}

// New dials the collector at host:port over UDP and builds the encoder for
// protocol. vendor (optional) selects a vendor-distinct wire encoding path.
// agentIP is the exporter address reported by sFlow (ignored by the
// NetFlow/IPFIX formats).
func New(logger *zap.Logger, host, port string, protocol Protocol, vendor, agentIP string, tel embed.TelemetrySettings) (*Output, error) {
	if logger == nil {
		return nil, fmt.Errorf("logger cannot be nil")
	}
	addr, err := net.ResolveUDPAddr("udp", net.JoinHostPort(host, port))
	if err != nil {
		return nil, fmt.Errorf("flow output: resolve %s:%s: %w", host, port, err)
	}
	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		return nil, fmt.Errorf("flow output: dial %s: %w", addr, err)
	}
	m, err := output.NewMetrics(tel.MeterProvider)
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("flow output: build metrics: %w", err)
	}

	spec := vendorSpecs[vendor] // zero value (no signature) when vendor == ""
	boot := time.Now()
	var enc encoder
	switch protocol {
	case ProtocolNetFlowV5:
		enc = v5enc{netflow.NewV5Encoder(boot)}
	case ProtocolNetFlowV9:
		enc = v9enc{netflow.NewV9Encoder(boot, 0, spec.v9Field)}
	case ProtocolIPFIX:
		enc = ipfixenc{ipfix.NewEncoder(0, spec.ipfixVendor())}
	case ProtocolSFlow:
		ip := net.ParseIP(agentIP)
		if ip == nil {
			ip = net.IPv4(127, 0, 0, 1)
		}
		enc = sflowenc{e: sflow.NewEncoder(ip, 0), boot: boot}
	default:
		_ = conn.Close()
		return nil, fmt.Errorf("flow output: unknown protocol %q", protocol)
	}

	return &Output{logger: logger, conn: conn, enc: enc, metrics: m}, nil
}

// ConsumeFlows encodes the batch and sends each packet over UDP.
func (o *Output) ConsumeFlows(ctx context.Context, records []embed.FlowRecord) error {
	for _, pkt := range o.enc.encode(time.Now(), records) {
		if _, err := o.conn.Write(pkt); err != nil {
			return fmt.Errorf("flow output: send: %w", err)
		}
	}
	o.metrics.BlitzOutputEntriesReceivedCounter.Add(ctx, int64(len(records)), outputType, "flows")
	return nil
}

// Write reports that logs are unsupported: this output is flows-only.
func (o *Output) Write(_ context.Context, _ output.LogRecord) error {
	return output.ErrUnsupportedTelemetryType
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

// encoder adapters — the wire encoders share a shape but not an interface.
type v5enc struct{ e *netflow.V5Encoder }

func (a v5enc) encode(now time.Time, f []embed.FlowRecord) [][]byte { return a.e.Encode(now, f) }

type v9enc struct{ e *netflow.V9Encoder }

func (a v9enc) encode(now time.Time, f []embed.FlowRecord) [][]byte { return a.e.Encode(now, f) }

type ipfixenc struct{ e *ipfix.Encoder }

func (a ipfixenc) encode(now time.Time, f []embed.FlowRecord) [][]byte { return a.e.Encode(now, f) }

type sflowenc struct {
	e    *sflow.Encoder
	boot time.Time
}

func (a sflowenc) encode(now time.Time, f []embed.FlowRecord) [][]byte {
	return a.e.Encode(now, a.boot, f)
}
