package dispatch

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/observiq/blitz/internal/config"
	"github.com/observiq/blitz/internal/datagen"
)

// TestOSLimitedGeneratorsGetMatchingOS pins that an OS-limited generator is
// only ever handed a host of its OS, across many environment seeds. A nil
// identity (no matching host in a tiny environment) is allowed: the generator
// then falls back to its own hostname.
func TestOSLimitedGeneratorsGetMatchingOS(t *testing.T) {
	cases := []struct {
		name string
		cfg  config.Generator
		want datagen.OSType
	}{
		{name: "wel", cfg: config.Generator{Type: config.GeneratorTypeWel}, want: datagen.OSWindows},
		{name: "kubernetes", cfg: config.Generator{Type: config.GeneratorTypeKubernetes}, want: datagen.OSLinux},
		{name: "hostmetrics-windows", cfg: config.Generator{Type: config.GeneratorTypeHostMetrics, HostMetrics: config.HostMetricsGeneratorConfig{OS: "windows"}}, want: datagen.OSWindows},
		{name: "hostmetrics-linux", cfg: config.Generator{Type: config.GeneratorTypeHostMetrics, HostMetrics: config.HostMetricsGeneratorConfig{OS: "linux"}}, want: datagen.OSLinux},
		// An unset os is the documented linux default, not "any OS".
		{name: "hostmetrics-unset", cfg: config.Generator{Type: config.GeneratorTypeHostMetrics}, want: datagen.OSLinux},
		// Validate accepts any spelling ParseOSType accepts, so selection must too.
		{name: "hostmetrics-Windows", cfg: config.Generator{Type: config.GeneratorTypeHostMetrics, HostMetrics: config.HostMetricsGeneratorConfig{OS: "Windows"}}, want: datagen.OSWindows},
		{name: "hostmetrics-spaced", cfg: config.Generator{Type: config.GeneratorTypeHostMetrics, HostMetrics: config.HostMetricsGeneratorConfig{OS: " windows "}}, want: datagen.OSWindows},
	}

	envs := testEnvironments(t)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			matched := 0
			for seed, env := range envs {
				id := hostIdentity(env, tc.cfg, 0)
				if id == nil {
					continue
				}
				require.Equal(t, tc.want, id.OSInfo.Type, "seed %d handed %s a %s host", seed, tc.name, id.OSInfo.Type)
				matched++
			}
			require.Positive(t, matched, "no seed produced a matching host")
		})
	}
}

// TestHostKindLimitedGeneratorsGetMatchingRole pins that a generator is only
// ever handed a host of a role that could emit its telemetry: server software
// never lands on a router or workstation, and WEL follows its configured role.
func TestHostKindLimitedGeneratorsGetMatchingRole(t *testing.T) {
	servers := []datagen.SystemRole{datagen.RoleServer}
	cases := []struct {
		name string
		cfg  config.Generator
		want []datagen.SystemRole
	}{
		{name: "apache-common", cfg: config.Generator{Type: config.GeneratorTypeApache}, want: servers},
		{name: "apache-combined", cfg: config.Generator{Type: config.GeneratorTypeApacheCombined}, want: servers},
		{name: "apache-error", cfg: config.Generator{Type: config.GeneratorTypeApacheError}, want: servers},
		{name: "nginx", cfg: config.Generator{Type: config.GeneratorTypeNginx}, want: servers},
		{name: "postgres", cfg: config.Generator{Type: config.GeneratorTypePostgres}, want: servers},
		{name: "kubernetes", cfg: config.Generator{Type: config.GeneratorTypeKubernetes}, want: servers},
		{name: "json", cfg: config.Generator{Type: config.GeneratorTypeJSON}, want: servers},
		{name: "fix", cfg: config.Generator{Type: config.GeneratorTypeFIX}, want: servers},
		{name: "traces", cfg: config.Generator{Type: config.GeneratorTypeTraces}, want: servers},
		{name: "wel-unset", cfg: config.Generator{Type: config.GeneratorTypeWel}, want: []datagen.SystemRole{datagen.RoleDC, datagen.RoleServer, datagen.RoleWorkstation}},
		{name: "wel-dc", cfg: config.Generator{Type: config.GeneratorTypeWel, Wel: config.WelGeneratorConfig{Role: "dc"}}, want: []datagen.SystemRole{datagen.RoleDC}},
		{name: "wel-member", cfg: config.Generator{Type: config.GeneratorTypeWel, Wel: config.WelGeneratorConfig{Role: "member"}}, want: servers},
		{name: "wel-workstation", cfg: config.Generator{Type: config.GeneratorTypeWel, Wel: config.WelGeneratorConfig{Role: "workstation"}}, want: []datagen.SystemRole{datagen.RoleWorkstation}},
	}

	envs := testEnvironments(t)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			matched := 0
			for seed, env := range envs {
				id := hostIdentity(env, tc.cfg, 0)
				if id == nil {
					continue
				}
				require.Contains(t, tc.want, id.Role, "seed %d handed %s a %s host", seed, tc.name, id.Role)
				matched++
			}
			require.Positive(t, matched, "no seed produced a matching host")
		})
	}
}

// testEnvironments builds the environments for seeds 0-199 once, for every
// case to share. A case may reuse an earlier case's pick when that host meets
// its requirement, and re-draws when it does not, so every host a case gets
// still meets its requirement, which is all these tests assert.
func testEnvironments(t *testing.T) []*datagen.Environment {
	t.Helper()
	envs := make([]*datagen.Environment, 200)
	for seed := range envs {
		envs[seed] = testEnvironment(t, int64(seed))
	}
	return envs
}

func testEnvironment(t *testing.T, seed int64) *datagen.Environment {
	t.Helper()
	seeds := datagen.NewSeedConfig()
	seeds.Shared = seed
	env, err := datagen.GenerateEnvironment(seeds, &datagen.EnvironmentOpts{SystemCount: 8})
	require.NoError(t, err)
	return env
}
