package datagen

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestRequiredRoutersAreAdded pins that the environment holds at least as many
// routing devices as required, appended after the drawn fleet so the drawn
// devices never move.
func TestRequiredRoutersAreAdded(t *testing.T) {
	for seed := int64(0); seed < 50; seed++ {
		seeds := NewSeedConfig()
		seeds.Shared = seed
		plain, err := GenerateEnvironment(seeds, &EnvironmentOpts{NetworkSystemCount: 1})
		require.NoError(t, err)
		env, err := GenerateEnvironment(seeds, &EnvironmentOpts{NetworkSystemCount: 1, RequiredRouters: 3})
		require.NoError(t, err)
		routers := 0
		for _, n := range env.NetworkSystems {
			if n.L3Routing != nil {
				routers++
			}
		}
		require.GreaterOrEqual(t, routers, 3, "seed %d", seed)
		require.Equal(t, plain.NetworkSystems[0].Serial, env.NetworkSystems[0].Serial, "seed %d: drawn device moved", seed)
	}
}

// TestFlowExporterFor pins the flow exporter selection: a routing device, of
// the vendor when one is named, distinct per ordinal, and nil when the
// environment has none.
func TestFlowExporterFor(t *testing.T) {
	env, err := GenerateEnvironment(NewSeedConfig(), &EnvironmentOpts{RequiredRouters: 2, RequiredVendors: []ApplianceVendor{VendorJuniper}})
	require.NoError(t, err)
	a, b := env.FlowExporterFor("flow", 0, ""), env.FlowExporterFor("flow", 1, "")
	require.NotNil(t, a)
	require.NotNil(t, b)
	require.NotNil(t, a.L3Routing)
	require.NotSame(t, a, b, "ordinals take separate devices")
	j := env.FlowExporterFor("flow", 0, VendorJuniper)
	require.NotNil(t, j)
	require.Equal(t, VendorJuniper, j.Vendor)
	require.NotNil(t, j.L3Routing)
	require.Nil(t, (&Environment{}).FlowExporterFor("flow", 0, ""))
}

// TestParseExporterVendor pins the config spellings of the flow exporter vendors.
func TestParseExporterVendor(t *testing.T) {
	for in, want := range map[string]ApplianceVendor{
		"": "", "cisco": VendorCisco, "arista": VendorArista, "juniper": VendorJuniper,
		"f5": VendorF5, "palo-alto": VendorPaloAlto, "fortinet": VendorFortinet,
	} {
		got, err := ParseExporterVendor(in)
		require.NoError(t, err, in)
		require.Equal(t, want, got, in)
	}
	_, err := ParseExporterVendor("hpe")
	require.Error(t, err)
}
