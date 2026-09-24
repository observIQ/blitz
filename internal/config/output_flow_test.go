package config

import "testing"

func TestFlowOutputVendorFormatMatrix(t *testing.T) {
	tests := []struct {
		name     string
		protocol string
		vendor   string
		wantErr  bool
	}{
		{"no vendor ipfix", "ipfix", "", false},
		{"appflow ipfix ok", "ipfix", "appflow", false},
		{"appflow v9 rejected", "netflow-v9", "appflow", true},
		{"appflow v5 rejected", "netflow-v5", "appflow", true},
		{"jflow v5 ok", "netflow-v5", "jflow", false},
		{"jflow v9 ok", "netflow-v9", "jflow", false},
		{"jflow ipfix ok", "ipfix", "jflow", false},
		{"netstream v5 ok", "netflow-v5", "netstream", false},
		{"netstream ipfix ok", "ipfix", "netstream", false},
		{"rflow v9 ok", "netflow-v9", "rflow", false},
		{"rflow ipfix rejected", "ipfix", "rflow", true},
		{"cflowd v9 ok", "netflow-v9", "cflowd", false},
		{"unknown vendor rejected", "ipfix", "bogus", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := FlowOutputConfig{Host: "127.0.0.1", Port: 2055, Protocol: tt.protocol, Vendor: tt.vendor}
			err := c.Validate()
			if tt.wantErr && err == nil {
				t.Fatalf("expected error for protocol=%q vendor=%q, got nil", tt.protocol, tt.vendor)
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error for protocol=%q vendor=%q: %v", tt.protocol, tt.vendor, err)
			}
		})
	}
}
