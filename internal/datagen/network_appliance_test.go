package datagen

import (
	"math/rand"
	"reflect"
	"testing"
)

func TestGenerateNetworkSystem_AllModels(t *testing.T) {
	r := rand.New(rand.NewSource(42))
	for _, spec := range networkModels {
		n := generateNetworkSystem(r, spec)

		if err := n.Validate(); err != nil {
			t.Errorf("%s: Validate() = %v, want nil", spec.model, err)
		}
		if n.Vendor != spec.vendor {
			t.Errorf("%s: vendor = %q, want %q", spec.model, n.Vendor, spec.vendor)
		}
		if n.Model != spec.model {
			t.Errorf("model = %q, want %q", n.Model, spec.model)
		}
		if n.OS == nil || n.OS.Vendor != spec.vendor || n.OS.Family != spec.osFamily {
			t.Errorf("%s: OS = %v, want vendor %q family %q", spec.model, n.OS, spec.vendor, spec.osFamily)
		}
		// Facet presence must match the spec mask exactly.
		checks := []struct {
			name    string
			present bool
			want    bool
		}{
			{"L2", n.L2Switching != nil, spec.facets.has(facetL2)},
			{"L3", n.L3Routing != nil, spec.facets.has(facetL3)},
			{"firewall", n.Firewall != nil, spec.facets.has(facetFirewall)},
			{"loadbalancing", n.LoadBalancing != nil, spec.facets.has(facetLB)},
			{"wireless", n.Wireless != nil, spec.facets.has(facetWireless)},
		}
		for _, c := range checks {
			if c.present != c.want {
				t.Errorf("%s: %s facet present = %v, want %v", spec.model, c.name, c.present, c.want)
			}
		}
		if len(n.Interfaces) < spec.minPorts || len(n.Interfaces) > spec.maxPorts {
			t.Errorf("%s: %d interfaces, want [%d,%d]", spec.model, len(n.Interfaces), spec.minPorts, spec.maxPorts)
		}
		if n.ManagementInterface == nil {
			t.Errorf("%s: nil management interface", spec.model)
		}
	}
}

func TestNetworkSystemIdentity_Validate(t *testing.T) {
	good := generateNetworkSystem(rand.New(rand.NewSource(1)), networkModels[0]) // Catalyst 9300: L2+L3

	mutate := func(fn func(n *NetworkSystemIdentity)) *NetworkSystemIdentity {
		cp := *good
		fn(&cp)
		return &cp
	}

	tests := []struct {
		name    string
		n       *NetworkSystemIdentity
		wantErr bool
	}{
		{"valid", good, false},
		{"empty vendor", mutate(func(n *NetworkSystemIdentity) { n.Vendor = "" }), true},
		{"empty model", mutate(func(n *NetworkSystemIdentity) { n.Model = "" }), true},
		{"empty serial", mutate(func(n *NetworkSystemIdentity) { n.Serial = "" }), true},
		{"nil OS", mutate(func(n *NetworkSystemIdentity) { n.OS = nil }), true},
		{"invalid OS", mutate(func(n *NetworkSystemIdentity) { n.OS = &ApplianceOS{} }), true},
		{"vendor/OS mismatch", mutate(func(n *NetworkSystemIdentity) {
			n.OS = &ApplianceOS{Vendor: VendorFortinet, Family: FamilyFortiOS, Version: "7.4.3"}
		}), true},
		{"no facets", mutate(func(n *NetworkSystemIdentity) {
			n.L2Switching, n.L3Routing, n.Firewall, n.LoadBalancing, n.Wireless = nil, nil, nil, nil, nil
		}), true},
		{"no interfaces", mutate(func(n *NetworkSystemIdentity) { n.Interfaces = nil }), true},
		{"nil management interface", mutate(func(n *NetworkSystemIdentity) { n.ManagementInterface = nil }), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.n.Validate(); (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNetworkSerial_UnknownVendor(t *testing.T) {
	// An unmapped vendor falls back to a generic uppercase-alphanumeric serial.
	got := networkSerial(rand.New(rand.NewSource(3)), ApplianceVendor("acme"))
	if len(got) != 12 {
		t.Errorf("fallback serial %q has length %d, want 12", got, len(got))
	}
}

func TestRandomNetworkSystemIdentity_Deterministic(t *testing.T) {
	a := RandomNetworkSystemIdentity(rand.New(rand.NewSource(99)))
	b := RandomNetworkSystemIdentity(rand.New(rand.NewSource(99)))
	if !reflect.DeepEqual(a, b) {
		t.Error("same seed produced different network systems")
	}
	if err := a.Validate(); err != nil {
		t.Errorf("RandomNetworkSystemIdentity produced invalid system: %v", err)
	}
}
