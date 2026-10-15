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
		config.GeneratorTypeFIX, config.GeneratorTypeTraces, config.GeneratorTypeF5,
	} {
		require.True(t, hostRequirement(config.Generator{Type: typ}).Apps, "%s", typ)
	}
}

// TestF5NeedsItsApplianceAndServer pins that the F5 generator, which describes
// both the F5 device (BIG-IP) and an NGINX server, gets both on every seed.
func TestF5NeedsItsApplianceAndServer(t *testing.T) {
	gens := []config.Generator{{Type: config.GeneratorTypeF5}}
	for seed := int64(0); seed < 50; seed++ {
		s := seed
		env, err := BuildEnvironment(zap.NewNop(), config.EnvironmentConfig{
			SeedConfig: config.EnvironmentSeedConfig{Shared: &s},
			Counts:     config.EnvironmentCounts{Systems: 1},
		}, gens)
		require.NoError(t, err)
		require.NotNil(t, applianceIdentity(env, gens[0], 0), "seed %d: F5 device", seed)
		srv := hostIdentity(env, gens[0], 0)
		require.NotNil(t, srv, "seed %d: NGINX server", seed)
		require.Equal(t, datagen.RoleServer, srv.Role, "seed %d", seed)
	}
}

// TestF5NginxHostIsLinux pins that the F5 generator's NGINX Plus host is a
// Linux server: NGINX Plus supports only Linux and FreeBSD (NGINX technical
// specs), so it never lands on Windows.
func TestF5NginxHostIsLinux(t *testing.T) {
	gens := []config.Generator{{Type: config.GeneratorTypeF5}}
	for seed := int64(0); seed < 100; seed++ {
		s := seed
		env, err := BuildEnvironment(zap.NewNop(), config.EnvironmentConfig{
			SeedConfig: config.EnvironmentSeedConfig{Shared: &s},
		}, gens)
		require.NoError(t, err)
		h := hostIdentity(env, gens[0], 0)
		require.NotNil(t, h, "seed %d", seed)
		require.Equal(t, datagen.OSLinux, h.OSInfo.Type, "seed %d: %s", seed, h.Hostname)
	}
}

// TestRequiredHostsCoverExtendedOSes pins that a hostmetrics entry for any OS
// ParseOSType accepts, the hypervisor hosts and Unix families included, gets
// an environment host of that OS on every seed, with a real release of it.
func TestRequiredHostsCoverExtendedOSes(t *testing.T) {
	for _, name := range []string{"esxi", "xen-dom0", "nutanix-ahv", "openstack-kvm", "aix", "solaris", "freebsd", "openbsd"} {
		want, err := datagen.ParseOSType(name)
		require.NoError(t, err)
		gens := []config.Generator{{Type: config.GeneratorTypeHostMetrics, HostMetrics: config.HostMetricsGeneratorConfig{OS: name}}}
		for seed := int64(0); seed < 50; seed++ {
			seeds := datagen.NewSeedConfig()
			seeds.Shared = seed
			env, err := datagen.GenerateEnvironment(seeds, &datagen.EnvironmentOpts{SystemCount: 5, RequiredHosts: RequiredHosts(gens)})
			require.NoError(t, err)
			host := hostIdentity(env, gens[0], 0)
			require.NotNil(t, host, "%s seed %d", name, seed)
			require.Equal(t, want, host.OSInfo.Type, "%s seed %d", name, seed)
			require.NotEmpty(t, host.OSInfo.Name, "%s seed %d: a real release", name, seed)
		}
	}
}

// TestFlowGetsAnEnvironmentRouter pins that a flow generator describes a
// routing device from the environment on every seed: any vendor by default,
// the configured vendor when one is set.
func TestFlowGetsAnEnvironmentRouter(t *testing.T) {
	for _, tc := range []struct {
		vendor string
		want   datagen.ApplianceVendor
	}{{"", ""}, {"juniper", datagen.VendorJuniper}, {"fortinet", datagen.VendorFortinet}} {
		gens := []config.Generator{{Type: config.GeneratorTypeFlow, Flow: config.FlowGeneratorConfig{ExporterVendor: tc.vendor}}}
		for seed := int64(0); seed < 50; seed++ {
			s := seed
			env, err := BuildEnvironment(zap.NewNop(), config.EnvironmentConfig{
				SeedConfig: config.EnvironmentSeedConfig{Shared: &s},
				Counts:     config.EnvironmentCounts{Systems: 1},
			}, gens)
			require.NoError(t, err)
			dev := flowExporter(env, gens[0], 0)
			require.NotNil(t, dev, "%q seed %d", tc.vendor, seed)
			require.NotNil(t, dev.L3Routing, "%q seed %d", tc.vendor, seed)
			if tc.want != "" {
				require.Equal(t, tc.want, dev.Vendor, "seed %d", seed)
			}
		}
	}
}
