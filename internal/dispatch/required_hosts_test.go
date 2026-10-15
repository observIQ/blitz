package dispatch

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/observiq/blitz/internal/config"
	"github.com/observiq/blitz/internal/datagen"
)

// TestRequiredHostsMeetEveryGenerator pins that an environment built from
// RequiredHosts gives every host generator an identity that meets its
// requirement, on every seed: a WEL dc entry always gets a domain controller,
// and a macOS hostmetrics entry a macOS host, never the generator's fallback.
func TestRequiredHostsMeetEveryGenerator(t *testing.T) {
	gens := []config.Generator{
		{Type: config.GeneratorTypeWel, Wel: config.WelGeneratorConfig{Role: "dc"}},
		{Type: config.GeneratorTypeHostMetrics, HostMetrics: config.HostMetricsGeneratorConfig{OS: "macos"}},
		{Type: config.GeneratorTypeKubernetes},
		{Type: config.GeneratorTypePaloAlto},
	}
	hosts := RequiredHosts(gens)
	require.Len(t, hosts, 3, "a generator that accepts any system needs no host")
	for seed := int64(0); seed < 100; seed++ {
		seeds := datagen.NewSeedConfig()
		seeds.Shared = seed
		env, err := datagen.GenerateEnvironment(seeds, &datagen.EnvironmentOpts{SystemCount: 5, RequiredHosts: hosts})
		require.NoError(t, err)
		dc := hostIdentity(env, gens[0], 0)
		require.NotNil(t, dc, "seed %d: wel dc", seed)
		require.Equal(t, datagen.RoleDC, dc.Role, "seed %d", seed)
		mac := hostIdentity(env, gens[1], 0)
		require.NotNil(t, mac, "seed %d: hostmetrics macos", seed)
		require.Equal(t, datagen.OSMacOS, mac.OSInfo.Type, "seed %d", seed)
		require.NotNil(t, hostIdentity(env, gens[2], 0), "seed %d: kubernetes", seed)
	}
}

// TestBuildEnvironmentHoldsRequiredHosts pins that the loaders' single
// environment builder passes both appliance and host requirements, so a
// configured WEL dc always lands on a domain controller.
func TestBuildEnvironmentHoldsRequiredHosts(t *testing.T) {
	gens := []config.Generator{{Type: config.GeneratorTypeWel, Wel: config.WelGeneratorConfig{Role: "dc"}}, {Type: config.GeneratorTypePaloAlto}}
	for seed := int64(0); seed < 50; seed++ {
		s := seed
		env, err := BuildEnvironment(zap.NewNop(), config.EnvironmentConfig{
			SeedConfig: config.EnvironmentSeedConfig{Shared: &s},
			Counts:     config.EnvironmentCounts{Systems: 3},
		}, gens)
		require.NoError(t, err)
		dc := hostIdentity(env, gens[0], 0)
		require.NotNil(t, dc, "seed %d", seed)
		require.Equal(t, datagen.RoleDC, dc.Role, "seed %d", seed)
		require.NotNil(t, applianceIdentity(env, gens[1], 0), "seed %d", seed)
	}
}

// TestServerSoftwareNeedsApplicationHosts pins that every generator describing
// server software requires a host that runs applications, so it never lands
// on a server whose OS runs none.
func TestServerSoftwareNeedsApplicationHosts(t *testing.T) {
	for _, typ := range []config.GeneratorType{
		config.GeneratorTypeApache, config.GeneratorTypeApacheCombined, config.GeneratorTypeApacheError,
		config.GeneratorTypeNginx, config.GeneratorTypePostgres, config.GeneratorTypeJSON,
		config.GeneratorTypeFIX, config.GeneratorTypeTraces,
	} {
		require.True(t, hostRequirement(config.Generator{Type: typ}).Apps, "%s", typ)
	}
}
