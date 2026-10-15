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
