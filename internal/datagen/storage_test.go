package datagen

import (
	"math/rand"
	"reflect"
	"testing"
)

func TestGenerateStorageSystem_AllHPEModels(t *testing.T) {
	r := rand.New(rand.NewSource(42))
	for _, spec := range hpeStorageModels {
		s := generateStorageSystem(r, spec)

		if err := s.Validate(); err != nil {
			t.Errorf("%s: Validate() = %v, want nil", spec.model, err)
		}
		if s.Vendor != StorageVendorHPE {
			t.Errorf("%s: vendor = %q, want hpe", spec.model, s.Vendor)
		}
		if s.Model != spec.model {
			t.Errorf("model = %q, want %q", s.Model, spec.model)
		}
		if s.OS == nil || s.OS.Vendor != VendorHPE || s.OS.Family != spec.osFamily {
			t.Errorf("%s: OS = %v, want vendor hpe family %q", spec.model, s.OS, spec.osFamily)
		}
		if !storageWWNRE.MatchString(s.WWN) {
			t.Errorf("%s: WWN %q is malformed", spec.model, s.WWN)
		}
		if !storageNAARE.MatchString(s.NAA) {
			t.Errorf("%s: NAA %q is malformed", spec.model, s.NAA)
		}
		if !storageIQNRE.MatchString(s.IQN) {
			t.Errorf("%s: IQN %q is malformed", spec.model, s.IQN)
		}
		if len(s.WWPN) == 0 {
			t.Errorf("%s: no WWPNs", spec.model)
		}
		for _, p := range s.WWPN {
			if !storageWWNRE.MatchString(p) {
				t.Errorf("%s: WWPN %q is malformed", spec.model, p)
			}
		}
		// Capacity coherence.
		c := s.Capacity
		if !(c.RawCapacityTB >= c.UsableCapacityTB && c.UsableCapacityTB > 0) {
			t.Errorf("%s: raw %.1f must be >= usable %.1f > 0", spec.model, c.RawCapacityTB, c.UsableCapacityTB)
		}
		if c.EffectiveCapacityTB < c.UsableCapacityTB {
			t.Errorf("%s: effective %.1f < usable %.1f", spec.model, c.EffectiveCapacityTB, c.UsableCapacityTB)
		}
		if c.DataReductionRatio < 1 {
			t.Errorf("%s: data reduction ratio %.2f < 1", spec.model, c.DataReductionRatio)
		}
		if len(s.Controllers) < 2 {
			t.Errorf("%s: %d controllers, want >= 2", spec.model, len(s.Controllers))
		}
		if len(s.Drives) == 0 {
			t.Errorf("%s: no drives", spec.model)
		}
		if s.ManagementInterface == nil {
			t.Errorf("%s: nil management interface", spec.model)
		}
	}
}

func TestStorageSystemIdentity_Validate(t *testing.T) {
	good := generateStorageSystem(rand.New(rand.NewSource(1)), hpeStorageModels[0])

	// mutate returns a copy of good with fn applied, for negative cases.
	mutate := func(fn func(s *StorageSystemIdentity)) *StorageSystemIdentity {
		cp := *good
		fn(&cp)
		return &cp
	}

	tests := []struct {
		name    string
		s       *StorageSystemIdentity
		wantErr bool
	}{
		{"valid", good, false},
		{"OS vendor incoherent with family", mutate(func(s *StorageSystemIdentity) {
			s.OS = &ApplianceOS{Vendor: VendorFortinet, Family: FamilyNimbleOS, Version: "6.1.2.0"}
		}), true},
		{"empty vendor", mutate(func(s *StorageSystemIdentity) { s.Vendor = "" }), true},
		{"nil OS", mutate(func(s *StorageSystemIdentity) { s.OS = nil }), true},
		{"bad WWN", mutate(func(s *StorageSystemIdentity) { s.WWN = "zz:zz" }), true},
		{"bad NAA", mutate(func(s *StorageSystemIdentity) { s.NAA = "naa.5deadbeef" }), true},
		{"bad IQN", mutate(func(s *StorageSystemIdentity) { s.IQN = "not-an-iqn" }), true},
		{"usable exceeds raw", mutate(func(s *StorageSystemIdentity) { s.Capacity.UsableCapacityTB = s.Capacity.RawCapacityTB + 1 }), true},
		{"effective below usable", mutate(func(s *StorageSystemIdentity) { s.Capacity.EffectiveCapacityTB = s.Capacity.UsableCapacityTB - 1 }), true},
		{"implausible reduction", mutate(func(s *StorageSystemIdentity) { s.Capacity.EffectiveCapacityTB = s.Capacity.UsableCapacityTB * 100 }), true},
		{"empty model", mutate(func(s *StorageSystemIdentity) { s.Model = "" }), true},
		{"empty serial", mutate(func(s *StorageSystemIdentity) { s.Serial = "" }), true},
		{"invalid OS", mutate(func(s *StorageSystemIdentity) { s.OS = &ApplianceOS{} }), true},
		{"malformed WWPN", mutate(func(s *StorageSystemIdentity) { s.WWPN = []string{"zz:zz"} }), true},
		{"non-positive capacity", mutate(func(s *StorageSystemIdentity) { s.Capacity.RawCapacityTB = 0 }), true},
		{"reduction below one", mutate(func(s *StorageSystemIdentity) { s.Capacity.DataReductionRatio = 0.5 }), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.s.Validate(); (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestIQNFor_UnknownVendor(t *testing.T) {
	// An unmapped vendor falls back to the com.example naming authority and
	// still produces a well-formed IQN.
	got := iqnFor(StorageVendor("acme"), "SN-123")
	if !storageIQNRE.MatchString(got) {
		t.Errorf("iqnFor unknown vendor produced malformed IQN %q", got)
	}
	if want := "iqn.2007-11.com.example:sn-123"; got != want {
		t.Errorf("iqnFor unknown vendor = %q, want %q", got, want)
	}
}

func TestRandomStorageSystemIdentity_Deterministic(t *testing.T) {
	a := RandomStorageSystemIdentity(rand.New(rand.NewSource(99)))
	b := RandomStorageSystemIdentity(rand.New(rand.NewSource(99)))
	if !reflect.DeepEqual(a, b) {
		t.Error("same seed produced different storage systems")
	}
	if err := a.Validate(); err != nil {
		t.Errorf("RandomStorageSystemIdentity produced invalid system: %v", err)
	}
}
