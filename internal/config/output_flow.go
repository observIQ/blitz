package config

import "fmt"

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
// cflowd (v5/v8/v9/IPFIX — Nokia SR OS cflowd; v8 is aggregation-only and
// excluded here, see note); Citrix AppFlow (IPFIX only — Citrix AppFlow is an
// IPFIX application); Redback rFlow (v5/v9 — Redback/Ericsson SmartEdge rFlow).
//
// How the vendor distinction rides each format: IPFIX carries a decoder-readable
// enterprise IE (the vendor's IANA PEN); NetFlow v9 has no PEN mechanism, so the
// signature is a proprietary field type in the template; NetFlow v5 is a fixed
// layout with no vendor mechanism at all, so a v5 export is byte-identical
// across vendors (the pairing is allowed for fidelity, but carries no
// vendor-distinct wire element — this is the real protocol's behavior).
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
	// Vendor is an optional vendor flavor (jflow, netstream, cflow, appflow,
	// rflow); it must be compatible with Protocol. Empty means standard.
	Vendor string `yaml:"vendor,omitempty" mapstructure:"vendor,omitempty"`
	// AgentIP is the exporter address reported by sFlow (optional).
	AgentIP string `yaml:"agentIP,omitempty" mapstructure:"agentIP,omitempty"`
}

// Validate validates the flow output configuration, including the
// vendor/protocol single-flavor compatibility rule.
func (c *FlowOutputConfig) Validate() error {
	if c.Port < 0 || c.Port > 65535 {
		return fmt.Errorf("flow output port must be between 0 and 65535, got %d", c.Port)
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
		if !contains(formats, proto) {
			return fmt.Errorf("flow output vendor %q does not support protocol %q; supported: %v", c.Vendor, proto, formats)
		}
	}
	return nil
}

func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}
