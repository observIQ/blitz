package dispatch

import (
	"context"
	"encoding/json"
	"regexp"
	"testing"
	"testing/fstest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/observiq/blitz/embed"
	"github.com/observiq/blitz/generator/resource"
	"github.com/observiq/blitz/internal/config"
	"github.com/observiq/blitz/internal/datagen"
)

var welComputer = regexp.MustCompile(`<Computer>([^<]*)</Computer>`)

// jsonHost extracts the top-level "host" field of a JSON log line.
func jsonHost(msg string) string {
	var m map[string]any
	if json.Unmarshal([]byte(msg), &m) != nil {
		return ""
	}
	h, _ := m["host"].(string)
	return h
}

// TestLogGeneratorsResolveOneHostname pins the single-hostname contract for
// every log generator: a generator resolves one hostname at construction and
// uses it everywhere. An explicit hostname setting wins and replaces only the
// identity's hostname; otherwise the environment identity's hostname applies;
// without either, the generator's own fallback (or the process hostname for
// generators with no hostname knob). A generator that also writes a hostname
// inside the message (inMessage non-nil) must write the same value there as in
// host.name.
func TestLogGeneratorsResolveOneHostname(t *testing.T) {
	const rate = 10 * time.Millisecond
	lib := fstest.MapFS{"sample.log": {Data: []byte("line one\nline two\n")}}

	cases := []struct {
		name string
		cfg  config.Generator
		// fallback is the hostname expected with neither an identity nor an
		// explicit hostname; empty means the process hostname.
		fallback  string
		inMessage func(msg string) string
		// setHostname sets the generator's explicit hostname knob; nil when
		// the generator has none.
		setHostname func(g *config.Generator, host string)
	}{
		{name: "json", cfg: config.Generator{Type: config.GeneratorTypeJSON, JSON: config.JSONGeneratorConfig{Workers: 1, Rate: rate}}, inMessage: jsonHost},
		{name: "json-pii", cfg: config.Generator{Type: config.GeneratorTypeJSON, JSON: config.JSONGeneratorConfig{Workers: 1, Rate: rate, Type: "pii"}}, inMessage: jsonHost},
		// palo-alto describes a network appliance, not a host: see appliance_test.go.
		{name: "apache-common", cfg: config.Generator{Type: config.GeneratorTypeApache, Apache: config.ApacheGeneratorConfig{Workers: 1, Rate: rate}}},
		{name: "apache-combined", cfg: config.Generator{Type: config.GeneratorTypeApacheCombined, ApacheCombined: config.ApacheCombinedGeneratorConfig{Workers: 1, Rate: rate}}},
		{name: "apache-error", cfg: config.Generator{Type: config.GeneratorTypeApacheError, ApacheError: config.ApacheErrorGeneratorConfig{Workers: 1, Rate: rate}}},
		{name: "nginx", cfg: config.Generator{Type: config.GeneratorTypeNginx, Nginx: config.NginxGeneratorConfig{Workers: 1, Rate: rate}}},
		{name: "postgres", cfg: config.Generator{Type: config.GeneratorTypePostgres, Postgres: config.PostgresGeneratorConfig{Workers: 1, Rate: rate}}},
		{name: "kubernetes", cfg: config.Generator{Type: config.GeneratorTypeKubernetes, Kubernetes: config.KubernetesGeneratorConfig{Workers: 1, Rate: rate}}},
		{name: "filegen", cfg: config.Generator{Type: config.GeneratorTypeFile, Filegen: config.FileGeneratorConfig{Workers: 1, Rate: rate, Source: "sample.log"}}},
		{name: "okta", cfg: config.Generator{Type: config.GeneratorTypeOkta, Okta: config.OktaGeneratorConfig{Workers: 1, Rate: rate}}},
		{name: "fix", cfg: config.Generator{Type: config.GeneratorTypeFIX, FIX: config.FIXGeneratorConfig{Workers: 1, Rate: rate}}},
		{
			name:     "wel",
			cfg:      config.Generator{Type: config.GeneratorTypeWel, Wel: config.WelGeneratorConfig{Workers: 1, Rate: rate}},
			fallback: "BLITZ-WEL",
			inMessage: func(msg string) string {
				m := welComputer.FindStringSubmatch(msg)
				if m == nil {
					return ""
				}
				return m[1]
			},
			setHostname: func(g *config.Generator, host string) { g.Wel.Computer = host },
		},
	}

	// One host per OS, sharing a name, so every generator (OS-limited or not)
	// resolves a host whose hostname the assertions can name.
	identityEnv := func() *datagen.Environment {
		return &datagen.Environment{Systems: []*datagen.SystemIdentity{
			{Hostname: "PANTHEON-ID-01", Role: datagen.RoleServer, OSInfo: datagen.OSInfo{Type: datagen.OSWindows}},
			{Hostname: "PANTHEON-ID-01", Role: datagen.RoleServer, OSInfo: datagen.OSInfo{Type: datagen.OSLinux}},
		}}
	}
	check := func(t *testing.T, rec embed.LogRecord, want string, inMessage func(string) string) {
		t.Helper()
		assert.Equal(t, want, rec.Metadata.Resource["host.name"])
		if inMessage != nil {
			assert.Equal(t, want, inMessage(rec.Message))
		}
	}

	for _, tc := range cases {
		t.Run(tc.name+"/identity", func(t *testing.T) {
			t.Parallel()
			check(t, firstRecord(t, tc.cfg, lib, identityEnv()), "PANTHEON-ID-01", tc.inMessage)
		})
		t.Run(tc.name+"/no-identity", func(t *testing.T) {
			t.Parallel()
			want := tc.fallback
			if want == "" {
				want = resource.Hostname()
			}
			check(t, firstRecord(t, tc.cfg, lib, nil), want, tc.inMessage)
		})
		if tc.setHostname == nil {
			continue
		}
		t.Run(tc.name+"/explicit-over-identity", func(t *testing.T) {
			t.Parallel()
			cfg := tc.cfg
			tc.setHostname(&cfg, "EXPLICIT-01")
			env := identityEnv()
			rec := firstRecord(t, cfg, lib, env)
			check(t, rec, "EXPLICIT-01", tc.inMessage)
			assert.Equal(t, "windows", rec.Metadata.Resource["os.type"], "explicit hostname replaces only the hostname")
			assert.Equal(t, "PANTHEON-ID-01", env.Systems[0].Hostname, "shared identity must not be mutated")
		})
		t.Run(tc.name+"/explicit-no-identity", func(t *testing.T) {
			t.Parallel()
			cfg := tc.cfg
			tc.setHostname(&cfg, "EXPLICIT-01")
			check(t, firstRecord(t, cfg, lib, nil), "EXPLICIT-01", tc.inMessage)
		})
	}
}

func firstRecord(t *testing.T, cfg config.Generator, lib fstest.MapFS, env *datagen.Environment) embed.LogRecord {
	t.Helper()
	cons := &capturingLogConsumer{}
	mod, err := ForEmbed(zap.NewNop(), cfg, EmbedConsumers{LogConsumer: cons}, lib, env, embed.NopTelemetry())
	require.NoError(t, err)
	require.NoError(t, mod.Start(context.Background()))
	require.Eventually(t, func() bool { return len(cons.snapshot()) > 0 }, 5*time.Second, 10*time.Millisecond)
	require.NoError(t, mod.Stop(context.Background()))
	return cons.snapshot()[0]
}
