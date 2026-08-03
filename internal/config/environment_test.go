package config

import (
	"testing"

	"go.uber.org/zap"
)

func i64(v int64) *int64 { return &v }

func TestEnvironmentConfig_Build_Counts(t *testing.T) {
	cfg := EnvironmentConfig{
		SeedConfig: EnvironmentSeedConfig{Shared: i64(42)},
		Counts:     EnvironmentCounts{Systems: 3, StorageSystems: 2, NetworkSystems: 4},
	}
	env, err := cfg.Build(zap.NewNop())
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if env == nil {
		t.Fatal("Build returned nil environment")
	}
	if len(env.Systems) != 3 {
		t.Errorf("systems = %d, want 3", len(env.Systems))
	}
	if len(env.StorageSystems) != 2 {
		t.Errorf("storage systems = %d, want 2", len(env.StorageSystems))
	}
	if len(env.NetworkSystems) != 4 {
		t.Errorf("network systems = %d, want 4", len(env.NetworkSystems))
	}
}

func TestEnvironmentConfig_Build_DeterministicIncludingSeedZero(t *testing.T) {
	// Every per-type seed set (covers all hydration branches), and shared:0
	// must be deterministic rather than randomized.
	cfg := EnvironmentConfig{
		SeedConfig: EnvironmentSeedConfig{
			Shared: i64(0), Systems: i64(1), Users: i64(2), Groups: i64(3),
			Services: i64(4), Applications: i64(5), Networks: i64(6),
			Domains: i64(7), StorageSystems: i64(8), NetworkSystems: i64(9),
		},
		Counts: EnvironmentCounts{Systems: 2, StorageSystems: 1, NetworkSystems: 1},
	}
	a, err := cfg.Build(zap.NewNop())
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	b, err := cfg.Build(zap.NewNop())
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if a.Systems[0].Hostname != b.Systems[0].Hostname {
		t.Error("shared:0 with fixed per-type seeds should be deterministic")
	}
	if a.StorageSystems[0].Serial != b.StorageSystems[0].Serial {
		t.Error("storage systems should be deterministic")
	}
}

func TestEnvironmentConfig_Build_NilLoggerNoPanic(t *testing.T) {
	env, err := EnvironmentConfig{Counts: EnvironmentCounts{Systems: 1}}.Build(nil)
	if err != nil {
		t.Fatalf("Build(nil logger): %v", err)
	}
	if env == nil {
		t.Fatal("Build(nil logger) returned nil environment")
	}
}

func TestEnvironmentConfig_Validate(t *testing.T) {
	if err := (EnvironmentConfig{Counts: EnvironmentCounts{Systems: 5, Users: 10}}).Validate(); err != nil {
		t.Errorf("valid config should pass: %v", err)
	}
	if err := (EnvironmentConfig{Counts: EnvironmentCounts{Systems: -1}}).Validate(); err == nil {
		t.Error("negative count should fail validation")
	}
	if err := (EnvironmentConfig{Counts: EnvironmentCounts{StorageSystems: -3}}).Validate(); err == nil {
		t.Error("negative storage-systems count should fail validation")
	}
}
