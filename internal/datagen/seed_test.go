package datagen

import (
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

func TestSeedConfigResolveSeed(t *testing.T) {
	t.Run("returns shared seed when no override", func(t *testing.T) {
		sc := &SeedConfig{Shared: 12345, Systems: -1}
		if got := sc.ResolveSeed(IdentitySystems); got != 12345 {
			t.Errorf("ResolveSeed(IdentitySystems) = %d, want 12345", got)
		}
	})

	t.Run("returns per-type override when set", func(t *testing.T) {
		sc := &SeedConfig{Shared: 12345, Systems: 99999, Users: -1}
		if got := sc.ResolveSeed(IdentitySystems); got != 99999 {
			t.Errorf("ResolveSeed(IdentitySystems) = %d, want 99999", got)
		}
		if got := sc.ResolveSeed(IdentityUsers); got != 12345 {
			t.Errorf("ResolveSeed(IdentityUsers) with override=-1 = %d, want fallback to 12345", got)
		}
	})

	t.Run("override of 0 is a deterministic seed, not a fallback", func(t *testing.T) {
		sc := &SeedConfig{Shared: 12345, Systems: 0}
		if got := sc.ResolveSeed(IdentitySystems); got != 0 {
			t.Errorf("ResolveSeed(IdentitySystems) with override=0 = %d, want 0 (deterministic)", got)
		}
	})

	t.Run("all per-type overrides work", func(t *testing.T) {
		sc := &SeedConfig{
			Shared:       100,
			Systems:      1,
			Users:        2,
			Groups:       3,
			Services:     4,
			Applications: 5,
			Networks:     6,
		}
		expected := map[IdentityType]int64{
			IdentitySystems:      1,
			IdentityUsers:        2,
			IdentityGroups:       3,
			IdentityServices:     4,
			IdentityApplications: 5,
			IdentityNetworks:     6,
		}
		for name, want := range expected {
			if got := sc.ResolveSeed(name); got != want {
				t.Errorf("ResolveSeed(%s) = %d, want %d", name, got, want)
			}
		}
	})

	t.Run("appliance seeds resolve", func(t *testing.T) {
		sc := &SeedConfig{Shared: 100, StorageSystems: 7, NetworkSystems: -1}
		if got := sc.ResolveSeed(IdentityStorageSystems); got != 7 {
			t.Errorf("ResolveSeed(IdentityStorageSystems) = %d, want 7", got)
		}
		if got := sc.ResolveSeed(IdentityNetworkSystems); got != 100 {
			t.Errorf("ResolveSeed(IdentityNetworkSystems) fallback = %d, want 100", got)
		}
	})

	t.Run("unknown type returns shared", func(t *testing.T) {
		sc := &SeedConfig{Shared: 42}
		if got := sc.ResolveSeed(IdentityType("unknown_type")); got != 42 {
			t.Errorf("ResolveSeed(unknown_type) = %d, want 42", got)
		}
	})

	t.Run("zero-value struct yields deterministic seed 0", func(t *testing.T) {
		sc := &SeedConfig{}
		if got := sc.ResolveSeed(IdentitySystems); got != 0 {
			t.Errorf("zero-value SeedConfig ResolveSeed = %d, want 0 (deterministic)", got)
		}
	})
}

func TestSeedConfigInit(t *testing.T) {
	t.Run("randomizes when shared is negative", func(t *testing.T) {
		sc := &SeedConfig{Shared: -1}
		logger := zaptest.NewLogger(t)
		sc.Init(logger)
		if sc.Shared < 0 {
			t.Errorf("Init() should set Shared to a non-negative random value, got %d", sc.Shared)
		}
	})

	t.Run("preserves explicit shared seed of 0 as deterministic", func(t *testing.T) {
		sc := &SeedConfig{Shared: 0}
		logger := zaptest.NewLogger(t)
		sc.Init(logger)
		if sc.Shared != 0 {
			t.Errorf("Init() changed deterministic Shared=0 to %d", sc.Shared)
		}
	})

	t.Run("preserves explicit positive shared seed", func(t *testing.T) {
		sc := &SeedConfig{Shared: 42}
		logger := zaptest.NewLogger(t)
		sc.Init(logger)
		if sc.Shared != 42 {
			t.Errorf("Init() changed Shared from 42 to %d", sc.Shared)
		}
	})

	t.Run("randomizes for any negative shared value", func(t *testing.T) {
		sc := &SeedConfig{Shared: -42}
		logger := zaptest.NewLogger(t)
		sc.Init(logger)
		if sc.Shared < 0 {
			t.Errorf("Init() should randomize when Shared=-42, got %d", sc.Shared)
		}
	})

	t.Run("nop logger does not panic", func(t *testing.T) {
		sc := &SeedConfig{Shared: 1}
		sc.Init(zap.NewNop())
	})
}

func TestNewSeedConfig(t *testing.T) {
	sc := NewSeedConfig()
	if sc.Shared != -1 {
		t.Errorf("NewSeedConfig().Shared = %d, want -1", sc.Shared)
	}
	if sc.Systems != -1 || sc.Users != -1 || sc.Groups != -1 ||
		sc.Services != -1 || sc.Applications != -1 || sc.Networks != -1 {
		t.Errorf("NewSeedConfig() did not initialize all per-type fields to -1: %+v", sc)
	}
}
