package dispatch

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/observiq/blitz/embed"
	"github.com/observiq/blitz/internal/config"
	"github.com/observiq/blitz/internal/datagen"
)

// TestPaloAltoDescribesAnEnvironmentFirewall pins that the Palo Alto generator
// describes a Palo Alto device from the environment: RequiredAppliances makes
// the environment hold one, and the generator's host.name is its hostname.
func TestPaloAltoDescribesAnEnvironmentFirewall(t *testing.T) {
	cfg := config.Generator{Type: config.GeneratorTypePaloAlto, PaloAlto: config.PaloAltoGeneratorConfig{Workers: 1, Rate: 10 * time.Millisecond}}
	vendors := RequiredAppliances([]config.Generator{cfg})
	require.Equal(t, []datagen.ApplianceVendor{datagen.VendorPaloAlto}, vendors)

	for seed := int64(0); seed < 10; seed++ {
		t.Run(fmt.Sprint(seed), func(t *testing.T) {
			t.Parallel()
			seeds := datagen.NewSeedConfig()
			seeds.Shared = seed
			env, err := datagen.GenerateEnvironment(seeds, &datagen.EnvironmentOpts{SystemCount: 8, RequiredVendors: vendors})
			require.NoError(t, err)
			want := env.ApplianceFor(string(cfg.Type), 0, datagen.VendorPaloAlto)
			require.NotNil(t, want)

			cons := &capturingLogConsumer{}
			mod, err := ForEmbedAt(zap.NewNop(), cfg, 0, EmbedConsumers{LogConsumer: cons}, nil, env, embed.NopTelemetry())
			require.NoError(t, err)
			require.NoError(t, mod.Start(context.Background()))
			require.Eventually(t, func() bool { return len(cons.snapshot()) > 0 }, 5*time.Second, 10*time.Millisecond)
			require.NoError(t, mod.Stop(context.Background()))
			res := cons.snapshot()[0].Metadata.Resource
			require.Equal(t, want.Hostname, res["host.name"], "seed %d", seed)
			require.Equal(t, "Palo Alto Networks", res["device.manufacturer"])
			require.Equal(t, want.Model, res["device.model.name"])
		})
	}
}

// TestSameTypeEntriesGetDistinctHosts pins that ForEmbedAt's ordinal spreads
// two entries of one type across hosts.
func TestSameTypeEntriesGetDistinctHosts(t *testing.T) {
	cfg := config.Generator{Type: config.GeneratorTypeHostMetrics, HostMetrics: config.HostMetricsGeneratorConfig{OS: "linux"}}
	env := &datagen.Environment{Systems: []*datagen.SystemIdentity{
		{Hostname: "a", OSInfo: datagen.OSInfo{Type: datagen.OSLinux}},
		{Hostname: "b", OSInfo: datagen.OSInfo{Type: datagen.OSLinux}},
	}}
	require.NotEqual(t, hostIdentity(env, cfg, 0).Hostname, hostIdentity(env, cfg, 1).Hostname)
}

// TestRequiredAppliancesListsEveryEntry pins one required device per
// appliance entry, so same-type entries each get their own device.
func TestRequiredAppliancesListsEveryEntry(t *testing.T) {
	pa := config.Generator{Type: config.GeneratorTypePaloAlto}
	require.Equal(t, []datagen.ApplianceVendor{datagen.VendorPaloAlto, datagen.VendorPaloAlto},
		RequiredAppliances([]config.Generator{pa, pa}))
}

// resolvePicks resolves every entry's environment device or host in config
// order, the way both loaders do, keyed "type#ordinal".
func resolvePicks(env *datagen.Environment, gens []config.Generator) map[string]string {
	picks := map[string]string{}
	ordinals := map[config.GeneratorType]int{}
	for _, g := range gens {
		o := ordinals[g.Type]
		ordinals[g.Type]++
		key := fmt.Sprintf("%s#%d", g.Type, o)
		switch {
		case g.Type == config.GeneratorTypeFlow:
			picks[key] = flowExporter(env, g, o).Hostname
		case applianceVendor(g.Type) != "":
			picks[key] = applianceIdentity(env, g, o).Hostname
		default:
			picks[key] = hostIdentity(env, g, o).Hostname
		}
	}
	return picks
}

// TestFlowExportersAreDistinct pins that flow entries each export from their
// own device on every seed, even when an entry that takes any routing device
// is mixed with entries pinned to a vendor.
func TestFlowExportersAreDistinct(t *testing.T) {
	gens := []config.Generator{
		{Type: config.GeneratorTypeFlow},
		{Type: config.GeneratorTypeFlow, Flow: config.FlowGeneratorConfig{ExporterVendor: "cisco"}},
		{Type: config.GeneratorTypeFlow},
		{Type: config.GeneratorTypeFlow, Flow: config.FlowGeneratorConfig{ExporterVendor: "cisco"}},
	}
	for seed := int64(0); seed < 200; seed++ {
		env, err := BuildEnvironment(zap.NewNop(), config.EnvironmentConfig{SeedConfig: config.EnvironmentSeedConfig{Shared: &seed}}, gens)
		require.NoError(t, err)
		seen := map[string]string{}
		for entry, host := range resolvePicks(env, gens) {
			require.NotContains(t, seen, host, "seed %d: %s and %s share %s", seed, seen[host], entry, host)
			seen[host] = entry
		}
		require.Equal(t, datagen.VendorCisco, flowExporter(env, gens[1], 1).Vendor, "seed %d", seed)
		require.Equal(t, datagen.VendorCisco, flowExporter(env, gens[3], 3).Vendor, "seed %d", seed)
	}
}

// TestHostEntriesAreDistinct pins that same-type host entries with different
// requirements each describe their own host on every seed: an entry that
// accepts any Windows role never takes the host a domain-controller entry
// needs.
func TestHostEntriesAreDistinct(t *testing.T) {
	gens := []config.Generator{
		{Type: config.GeneratorTypeWel},
		{Type: config.GeneratorTypeWel, Wel: config.WelGeneratorConfig{Role: "dc"}},
	}
	for seed := int64(0); seed < 500; seed++ {
		// A one-system fleet leaves mostly the hosts the entries require.
		env, err := BuildEnvironment(zap.NewNop(), config.EnvironmentConfig{SeedConfig: config.EnvironmentSeedConfig{Shared: &seed}, Counts: config.EnvironmentCounts{Systems: 1}}, gens)
		require.NoError(t, err)
		picks := resolvePicks(env, gens)
		require.NotEqual(t, picks["wel#0"], picks["wel#1"], "seed %d", seed)
		require.Equal(t, datagen.RoleDC, hostIdentity(env, gens[1], 1).Role, "seed %d", seed)
	}
}

// TestPicksAreRepeatable pins the seed contract for attachments: the same
// seed and config resolve every entry to the same device or host.
func TestPicksAreRepeatable(t *testing.T) {
	seed := int64(42)
	envCfg := config.EnvironmentConfig{SeedConfig: config.EnvironmentSeedConfig{Shared: &seed}}
	gens := []config.Generator{
		{Type: config.GeneratorTypeFlow},
		{Type: config.GeneratorTypeFlow, Flow: config.FlowGeneratorConfig{ExporterVendor: "palo-alto"}},
		{Type: config.GeneratorTypePaloAlto},
		{Type: config.GeneratorTypeJSON},
		{Type: config.GeneratorTypeWel, Wel: config.WelGeneratorConfig{Role: "dc"}},
	}
	build := func() map[string]string {
		env, err := BuildEnvironment(zap.NewNop(), envCfg, gens)
		require.NoError(t, err)
		return resolvePicks(env, gens)
	}
	first := build()
	for range 10 {
		require.Equal(t, first, build())
	}
}

// TestF5DescribesAnEnvironmentLoadBalancer pins F5 wiring: the environment
// holds an F5 device, BIG-IP lines name it, and NGINX Plus lines name a
// server host from the environment.
func TestF5DescribesAnEnvironmentLoadBalancer(t *testing.T) {
	cfg := config.Generator{Type: config.GeneratorTypeF5, F5: config.F5GeneratorConfig{Workers: 1, Rate: 5 * time.Millisecond, Seed: 1}}
	vendors := RequiredAppliances([]config.Generator{cfg})
	require.Equal(t, []datagen.ApplianceVendor{datagen.VendorF5}, vendors)

	seeds := datagen.NewSeedConfig()
	seeds.Shared = 3
	env, err := datagen.GenerateEnvironment(seeds, &datagen.EnvironmentOpts{SystemCount: 8, RequiredVendors: vendors})
	require.NoError(t, err)
	lb := env.ApplianceFor(string(cfg.Type), 0, datagen.VendorF5)
	web := hostIdentity(env, cfg, 0)
	require.NotNil(t, lb)
	require.NotNil(t, web)

	for product, want := range map[string]string{"asm": lb.Hostname, "nginx-plus": web.Hostname} {
		c := cfg
		c.F5.EnabledProducts = []string{product}
		cons := &capturingLogConsumer{}
		mod, err := ForEmbedAt(zap.NewNop(), c, 0, EmbedConsumers{LogConsumer: cons}, nil, env, embed.NopTelemetry())
		require.NoError(t, err)
		require.NoError(t, mod.Start(context.Background()))
		require.Eventually(t, func() bool { return len(cons.snapshot()) > 0 }, 5*time.Second, 5*time.Millisecond)
		require.NoError(t, mod.Stop(context.Background()))
		require.Equal(t, want, cons.snapshot()[0].Metadata.Resource["host.name"], product)
	}
}
