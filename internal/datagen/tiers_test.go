package datagen

import (
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestGenerateSystems_Tiers(t *testing.T) {
	seeds := NewSeedConfig()
	seeds.Shared = 42
	seeds.Init(zap.NewNop())

	env, err := GenerateEnvironment(seeds, &EnvironmentOpts{
		SystemCount: 300,
		Now:         time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("GenerateEnvironment: %v", err)
	}

	counts := map[DeploymentTier]int{}
	prodWindows := map[OSInfo]struct{}{}
	for _, s := range env.Systems {
		if s.Tier == "" {
			t.Fatal("system has no deployment tier assigned")
		}
		counts[s.Tier]++
		if s.Tier == TierProd && s.OSInfo.Type == OSWindows {
			prodWindows[s.OSInfo] = struct{}{}
		}
	}

	// All four tiers appear in a fleet of 300.
	for _, tr := range DeploymentTiers {
		if counts[tr] == 0 {
			t.Errorf("tier %q never assigned", tr)
		}
	}
	// Prod is the plurality (55% vs 15%).
	if counts[TierProd] <= counts[TierStaging] {
		t.Errorf("prod (%d) should dominate staging (%d)", counts[TierProd], counts[TierStaging])
	}
	// Prod Windows hosts are pinned to a single OS release (uniform fleet).
	if len(prodWindows) > 1 {
		t.Errorf("prod Windows hosts should share one release, got %d distinct", len(prodWindows))
	}
	if len(prodWindows) == 0 {
		t.Error("expected at least one prod Windows host in a fleet of 300")
	}
}
