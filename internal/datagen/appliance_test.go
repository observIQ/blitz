package datagen

import (
	"math/rand"
	"testing"
)

func TestApplianceOS_String(t *testing.T) {
	tests := []struct {
		os   ApplianceOS
		want string
	}{
		{ApplianceOS{VendorHPE, FamilyNimbleOS, "6.1.2.0"}, "NimbleOS 6.1.2.0"},
		{ApplianceOS{VendorHPE, FamilyStoreOnceOS, "4.3.13"}, "HPE StoreOnce 4.3.13"},
		{ApplianceOS{VendorF5, FamilyBIGIP, "17.1.0.3"}, "BIG-IP 17.1.0.3"},
		{ApplianceOS{VendorCisco, FamilyIOSXE, "17.12.3"}, "Cisco IOS XE Software, Version 17.12.3"},
		{ApplianceOS{VendorCisco, FamilyNXOS, "10.3(4a)"}, "Cisco Nexus Operating System (NX-OS) Software, Version 10.3(4a)"},
		{ApplianceOS{VendorJuniper, FamilyJunos, "23.4R1"}, "Junos: 23.4R1"},
		{ApplianceOS{VendorFortinet, FamilyFortiOS, "7.4.3"}, "FortiOS v7.4.3"},
		{ApplianceOS{VendorPaloAlto, FamilyPANOS, "11.1.3"}, "PAN-OS 11.1.3"},
	}
	for _, tt := range tests {
		if got := tt.os.String(); got != tt.want {
			t.Errorf("String() = %q, want %q", got, tt.want)
		}
	}
}

func TestApplianceOS_Validate(t *testing.T) {
	tests := []struct {
		name    string
		os      ApplianceOS
		wantErr bool
	}{
		{"valid", ApplianceOS{VendorHPE, FamilyNimbleOS, "6.1.2.0"}, false},
		{"valid three-segment", ApplianceOS{VendorCisco, FamilyIOSXE, "17.12.3"}, false},
		{"valid vendor-suffixed", ApplianceOS{VendorJuniper, FamilyJunos, "23.4R1"}, false},
		{"missing vendor", ApplianceOS{"", FamilyNimbleOS, "6.1.2.0"}, true},
		{"missing family", ApplianceOS{VendorHPE, "", "6.1.2.0"}, true},
		{"missing version", ApplianceOS{VendorHPE, FamilyNimbleOS, ""}, true},
		{"non-numeric version", ApplianceOS{VendorHPE, FamilyNimbleOS, "latest"}, true},
		{"single-segment version", ApplianceOS{VendorHPE, FamilyNimbleOS, "6"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.os.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// knownApplianceFamilies is every family GenerateApplianceOS must support.
var knownApplianceFamilies = []ApplianceOSFamily{
	FamilyNimbleOS, Family3PAROS, FamilyAlletraOS, FamilyStoreOnceOS,
	FamilyBIGIP, FamilyIOSXE, FamilyNXOS, FamilyEOS, FamilyJunos, FamilyPANOS, FamilyFortiOS,
}

func TestGenerateApplianceOS_AllFamiliesValid(t *testing.T) {
	r := rand.New(rand.NewSource(42))
	for _, f := range knownApplianceFamilies {
		os := GenerateApplianceOS(r, f)
		if err := os.Validate(); err != nil {
			t.Errorf("GenerateApplianceOS(%q) produced invalid OS %v: %v", f, os, err)
		}
		if os.Family != f {
			t.Errorf("GenerateApplianceOS(%q) family = %q, want %q", f, os.Family, f)
		}
		if os.Vendor == "" {
			t.Errorf("GenerateApplianceOS(%q) has empty vendor", f)
		}
	}
}

func TestGenerateApplianceOS_Deterministic(t *testing.T) {
	a := GenerateApplianceOS(rand.New(rand.NewSource(7)), FamilyNimbleOS)
	b := GenerateApplianceOS(rand.New(rand.NewSource(7)), FamilyNimbleOS)
	if a != b {
		t.Errorf("same seed produced different results: %v vs %v", a, b)
	}
}

func TestGenerateApplianceOS_VendorCoherence(t *testing.T) {
	// A family always resolves to its one true vendor; NimbleOS is HPE and can
	// never drift to F5's tmos.
	os := GenerateApplianceOS(rand.New(rand.NewSource(1)), FamilyNimbleOS)
	if os.Vendor != VendorHPE {
		t.Errorf("NimbleOS vendor = %q, want %q", os.Vendor, VendorHPE)
	}
	if os.Family == FamilyBIGIP {
		t.Error("NimbleOS family drifted to BIG-IP")
	}
}
