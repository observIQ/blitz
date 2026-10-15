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
	// Prod Windows releases, keyed by whether the host is a domain controller.
	prodWindows := map[bool]map[OSInfo]struct{}{false: {}, true: {}}
	for _, s := range env.Systems {
		if s.Tier == "" {
			t.Fatal("system has no deployment tier assigned")
		}
		counts[s.Tier]++
		if s.Tier == TierProd && s.OSInfo.Type == OSWindows {
			prodWindows[s.Role == RoleDC][s.OSInfo] = struct{}{}
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
	// Prod Windows hosts are pinned to a single OS release (uniform fleet),
	// one for domain controllers (Windows Server) and one for the rest.
	for dc, releases := range prodWindows {
		if len(releases) != 1 {
			t.Errorf("prod Windows hosts (domain controllers: %v) should share one release, got %d distinct", dc, len(releases))
		}
	}
}
