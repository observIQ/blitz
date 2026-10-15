package config

import (
	"fmt"
	"net"
	"slices"
)

// Default flow output configuration values.
const (
	// DefaultFlowOutputHost is the collector host.
	DefaultFlowOutputHost = "127.0.0.1"
	// DefaultFlowOutputPort is the common NetFlow/IPFIX collector port.
	DefaultFlowOutputPort = 2055
	// DefaultFlowProtocol is the wire format used when none is set.
	DefaultFlowProtocol = "netflow-v9"
)

// validFlowProtocols is the set of accepted wire formats.
var validFlowProtocols = map[string]bool{
	"": true, "netflow-v5": true, "netflow-v9": true, "ipfix": true, "sflow": true,
}

// flowVendorFormats maps a vendor flavor to every wire format that vendor's
// exporter actually supports in the real world. A vendor is mutually exclusive
// with the others by construction (one field), and the configured protocol must
// be one the vendor supports.
//
// Sources (per row): Juniper jFlow (v5/v9/IPFIX — Juniper "Flow Monitoring"
// docs); Huawei NetStream (v5/v9/IPFIX — Huawei NetStream config guide); Nokia
// cflowd (v5/v8/v9/IPFIX — Nokia SR OS cflowd; v8 is aggregation-only, and
// blitz encodes no v8); Citrix AppFlow (IPFIX only — Citrix AppFlow is an
// IPFIX application); Redback rFlow (v5/v9 — Redback/Ericsson SmartEdge rFlow).
//
// On the wire, only IPFIX can mark a vendor, with an enterprise IE under the
// vendor's IANA PEN, and only jflow (Juniper) and appflow (Citrix) have a
// published IE to use (see output/flow). NetFlow v5 and v9 have no vendor
// mechanism, so those exports, and every netstream/cflowd/rflow export, are
// wire-standard; the pairing is still validated for realism.
var flowVendorFormats = map[string][]string{
	"jflow":     {"netflow-v5", "netflow-v9", "ipfix"}, // Juniper, PEN 2636
	"netstream": {"netflow-v5", "netflow-v9", "ipfix"}, // Huawei, PEN 2011
	"cflowd":    {"netflow-v5", "netflow-v9", "ipfix"}, // Nokia/Alcatel-Lucent, PEN 6527
	"appflow":   {"ipfix"},                             // Citrix, PEN 5951
	"rflow":     {"netflow-v5", "netflow-v9"},          // Redback/Ericsson
}

// FlowOutputConfig contains configuration for the flow output.
type FlowOutputConfig struct {
	// Host is the collector host.
	Host string `yaml:"host,omitempty" mapstructure:"host,omitempty"`
	// Port is the collector UDP port.
	Port int `yaml:"port,omitempty" mapstructure:"port,omitempty"`
	// Protocol is the wire format: netflow-v5, netflow-v9, ipfix, sflow.
	Protocol string `yaml:"protocol,omitempty" mapstructure:"protocol,omitempty"`
	// Vendor is an optional vendor flavor (jflow, netstream, cflowd, appflow,
	// rflow); it must be compatible with Protocol. Empty means standard.
	Vendor string `yaml:"vendor,omitempty" mapstructure:"vendor,omitempty"`
	// AgentIP is the exporter address reported by sFlow (optional).
	AgentIP string `yaml:"agentIP,omitempty" mapstructure:"agentIP,omitempty"`
}

// Validate validates the flow output configuration, including the
// vendor/protocol single-flavor compatibility rule.
func (c *FlowOutputConfig) Validate() error {
	if err := ValidateHost(c.Host); err != nil {
		return fmt.Errorf("flow output %w", err)
	}
	if err := ValidatePort(c.Port); err != nil {
		return fmt.Errorf("flow output %w", err)
	}
	if c.AgentIP != "" && net.ParseIP(c.AgentIP) == nil {
		return fmt.Errorf("flow output agentIP must be an IPv4 or IPv6 address, got %q", c.AgentIP)
	}
	if !validFlowProtocols[c.Protocol] {
		return fmt.Errorf("flow output protocol must be one of: netflow-v5, netflow-v9, ipfix, sflow; got %q", c.Protocol)
	}
	if c.Vendor != "" {
		formats, ok := flowVendorFormats[c.Vendor]
		if !ok {
			return fmt.Errorf("flow output vendor must be one of: appflow, jflow, cflowd, netstream, rflow; got %q", c.Vendor)
		}
		proto := c.Protocol
		if proto == "" {
			proto = DefaultFlowProtocol
		}
		if !slices.Contains(formats, proto) {
			return fmt.Errorf("flow output vendor %q does not support protocol %q; supported: %v", c.Vendor, proto, formats)
		}
	}
	return nil
}
