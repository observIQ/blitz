package config_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/observiq/blitz/config"
	"github.com/observiq/blitz/embed"
	internalconfig "github.com/observiq/blitz/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

// mockConsumer captures records for assertions.
type mockConsumer struct {
	mu      sync.Mutex
	batches int
}

func (m *mockConsumer) ConsumeLogs(_ context.Context, records []embed.LogRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.batches++
	_ = records
	return nil
}

func TestLoad_ValidYAML(t *testing.T) {
	yaml := []byte(`
generator:
  type: apache-common
  apache-common:
    workers: 1
    rate: 1s
output:
  type: nop
logging:
  type: stdout
metrics:
  port: 19000
`)
	cfg, err := config.Load(yaml, config.LoadOpts{})
	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Equal(t, "apache-common", string(cfg.Generator.Type))
	assert.Equal(t, 1, cfg.Generator.Apache.Workers)
}

func TestLoad_InvalidYAML(t *testing.T) {
	_, err := config.Load([]byte("not: valid: yaml: ::"), config.LoadOpts{})
	require.Error(t, err)
}

func TestLoad_FailsValidation(t *testing.T) {
	// Invalid generator type triggers Validate failure.
	yaml := []byte(`
generator:
  type: this-generator-does-not-exist
output:
  type: nop
logging:
  type: stdout
metrics:
  port: 19000
`)
	_, err := config.Load(yaml, config.LoadOpts{})
	require.Error(t, err)
}

func TestLoad_EnvOverrideAppliesAfterYAML(t *testing.T) {
	// YAML sets workers=1, EnvOverrides bumps it to 4. The host pretends
	// to have collected BLITZ_GENERATOR_APACHE-COMMON_WORKERS=4 from its
	// own env-loading layer and translated it to the YAML path.
	yaml := []byte(`
generator:
  type: apache-common
  apache-common:
    workers: 1
    rate: 1s
output:
  type: nop
logging:
  type: stdout
metrics:
  port: 19000
`)
	cfg, err := config.Load(yaml, config.LoadOpts{
		EnvOverrides: map[string]string{
			"generator.apache-common.workers": "4",
		},
	})
	require.NoError(t, err)
	assert.Equal(t, 4, cfg.Generator.Apache.Workers, "EnvOverrides should overlay onto YAML values")
}

func TestLoadModules_NilConsumer(t *testing.T) {
	yaml := []byte(`
generator:
  type: apache-common
  apache-common:
    workers: 1
    rate: 1s
output:
  type: nop
logging:
  type: stdout
metrics:
  port: 19000
`)
	_, err := config.LoadModules(yaml, config.EmbedOpts{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "LogConsumer")
}

func TestLoadModules_SingleProducer(t *testing.T) {
	yaml := []byte(`
generator:
  type: apache-common
  apache-common:
    workers: 1
    rate: 1s
output:
  type: nop
logging:
  type: stdout
metrics:
  port: 19000
`)
	consumer := &mockConsumer{}
	mods, err := config.LoadModules(yaml, config.EmbedOpts{
		Logger:      zap.NewNop(),
		LogConsumer: consumer,
	})
	require.NoError(t, err)
	require.Len(t, mods, 1)
	assert.Equal(t, "apache", mods[0].Name())
}

func TestLoadModules_MultiGenerator(t *testing.T) {
	yaml := []byte(`
generators:
  - type: apache-common
    apache-common: {workers: 1, rate: 1s}
  - type: nginx
    nginx: {workers: 1, rate: 1s}
  - type: postgres
    postgres: {workers: 1, rate: 1s}
output:
  type: nop
logging:
  type: stdout
metrics:
  port: 19000
`)
	consumer := &mockConsumer{}
	mods, err := config.LoadModules(yaml, config.EmbedOpts{LogConsumer: consumer})
	require.NoError(t, err)
	require.Len(t, mods, 3)
	names := []string{mods[0].Name(), mods[1].Name(), mods[2].Name()}
	assert.ElementsMatch(t, []string{"apache", "nginx", "postgres"}, names)
}

func TestLoadModules_HostMetricsRequiresMetricConsumer(t *testing.T) {
	// hostmetrics is a metric Producer; LoadModules must require
	// EmbedOpts.MetricConsumer rather than accepting a log-only opts.
	yaml := []byte(`
generator:
  type: hostmetrics
  hostmetrics:
    rate: 1s
    os: linux
output:
  type: nop
logging:
  type: stdout
metrics:
  port: 19000
`)
	_, err := config.LoadModules(yaml, config.EmbedOpts{LogConsumer: &mockConsumer{}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "MetricConsumer")
}

// nopMetricConsumer accepts and discards metric points.
type nopMetricConsumer struct{}

func (nopMetricConsumer) ConsumeMetrics(context.Context, []embed.MetricPoint) error { return nil }

// TestLoadModules_WarnsOnRemovedSettings pins that the embed path warns about
// a removed setting, as the CLI does, rather than ignoring it silently.
func TestLoadModules_WarnsOnRemovedSettings(t *testing.T) {
	yaml := []byte(`
generator:
  type: hostmetrics
  hostmetrics:
    workers: 4
    rate: 1s
    os: linux
output:
  type: nop
logging:
  type: stdout
metrics:
  port: 19000
`)
	core, logs := observer.New(zap.WarnLevel)
	_, err := config.LoadModules(yaml, config.EmbedOpts{Logger: zap.New(core), MetricConsumer: nopMetricConsumer{}})
	require.NoError(t, err)
	assert.Equal(t, 1, logs.FilterMessage(internalconfig.HostMetricsWorkersRemoved).Len())
}

func TestLoadModules_TracesRequiresTraceConsumer(t *testing.T) {
	// traces is a trace Producer; LoadModules must require
	// EmbedOpts.TraceConsumer rather than accepting a log-only opts.
	yaml := []byte(`
generator:
  type: traces
  traces:
    workers: 1
    rate: 1s
output:
  type: nop
logging:
  type: stdout
metrics:
  port: 19000
`)
	_, err := config.LoadModules(yaml, config.EmbedOpts{LogConsumer: &mockConsumer{}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "TraceConsumer")
}

func TestLoadModules_RejectsWinevt(t *testing.T) {
	yaml := []byte(`
generator:
  type: winevt
  winevt:
    workers: 1
    rate: 1s
output:
  type: nop
logging:
  type: stdout
metrics:
  port: 19000
`)
	_, err := config.LoadModules(yaml, config.EmbedOpts{LogConsumer: &mockConsumer{}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "winevt")
	assert.Contains(t, err.Error(), "DEPRECATED")
	assert.Contains(t, err.Error(), "`wel` generator")
}

func TestLoadModules_PartialResultNotReturnedOnError(t *testing.T) {
	// First generator is valid (apache), second is not (winevt). The
	// returned slice must be nil — never the partial first-generator
	// result that would silently lose the second.
	yaml := []byte(`
generators:
  - type: apache-common
    apache-common: {workers: 1, rate: 1s}
  - type: winevt
    winevt: {workers: 1, rate: 1s}
output:
  type: nop
logging:
  type: stdout
metrics:
  port: 19000
`)
	mods, err := config.LoadModules(yaml, config.EmbedOpts{LogConsumer: &mockConsumer{}})
	require.Error(t, err)
	assert.Nil(t, mods, "must not return a partial module slice on error")
}

// TestLoadAcceptsUnquotedRemoteWriteVersion pins that an unquoted YAML
// version (2.0, which YAML reads as a number and viper hands over as "2") is
// accepted, as the docs' quoted form is.
func TestLoadAcceptsUnquotedRemoteWriteVersion(t *testing.T) {
	for _, v := range []string{"1.0", "2.0"} {
		_, err := config.Load([]byte("output:\n  type: prometheus-remote-write\n  prometheus-remote-write:\n    endpoint: http://127.0.0.1:9090/api/v1/write\n    version: "+v+"\nlogging:\n  type: stdout\nmetrics:\n  port: 19000\n"), config.LoadOpts{})
		require.NoError(t, err, v)
	}
}

// TestLoad_ListEntrySeedsFollowTheContract pins the seed contract for
// generators: list entries: an omitted seed is random (-1), an explicit 0 is
// deterministic, like the singular form.
func TestLoad_ListEntrySeedsFollowTheContract(t *testing.T) {
	cfg, err := config.Load([]byte(`
generators:
  - type: hostmetrics
    hostmetrics: {workers: 1, rate: 1s, os: linux}
  - type: traces
    traces: {workers: 1, rate: 1s}
  - type: hostmetrics
    hostmetrics: {workers: 1, rate: 1s, os: linux, seed: 0}
output:
  type: nop
logging:
  type: stdout
metrics:
  port: 19000
`), config.LoadOpts{})
	require.NoError(t, err)
	require.Len(t, cfg.Generators, 3)
	assert.Equal(t, int64(-1), cfg.Generators[0].HostMetrics.Seed, "omitted hostmetrics seed is random")
	assert.Equal(t, int64(-1), cfg.Generators[1].Traces.Seed, "omitted traces seed is random")
	assert.Equal(t, int64(0), cfg.Generators[2].HostMetrics.Seed, "explicit 0 is deterministic")
}

// TestLoad_ExpandedRandomSeedResolvesOnce pins that Load resolves a random
// seed on an OS-expanded hostmetrics entry once, so its hosts replay.
func TestLoad_ExpandedRandomSeedResolvesOnce(t *testing.T) {
	cfg, err := config.Load([]byte(`
generators:
  - type: hostmetrics
    hostmetrics: {workers: 1, rate: 1s, os: "linux,windows", seed: -1}
output:
  type: nop
logging:
  type: stdout
metrics:
  port: 19000
`), config.LoadOpts{})
	require.NoError(t, err)
	gens := cfg.EffectiveGenerators()
	require.Len(t, gens, 2)
	assert.GreaterOrEqual(t, gens[0].HostMetrics.Seed, int64(0), "resolved at load")
	assert.Equal(t, gens, cfg.EffectiveGenerators())
}

// Load applies the same declared defaults as the CLI, so a singular stanza
// that omits a key gets that key's default, exactly as a list entry does.
func TestLoad_SingularGeneratorGetsDefaults(t *testing.T) {
	cfg, err := config.Load([]byte("generator:\n  type: json\n"), config.LoadOpts{})
	require.NoError(t, err)
	assert.Equal(t, 1, cfg.Generator.JSON.Workers, "unset workers gets the type default")
}

func TestLoad_SingularOutputGetsDefaults(t *testing.T) {
	cfg, err := config.Load([]byte(`
generator:
  type: json
  json:
    workers: 1
    rate: 1s
output:
  type: tcp
  tcp:
    host: localhost
    port: 5000
`), config.LoadOpts{})
	require.NoError(t, err)
	assert.Equal(t, 1, cfg.Output.TCP.Workers, "unset workers gets the type default")
}

func TestLoad_RejectsOutputAndOutputs(t *testing.T) {
	_, err := config.Load([]byte(`
generator:
  type: json
output:
  type: nop
outputs:
  - type: nop
`), config.LoadOpts{})
	require.Error(t, err, "embedded Load must reject singular and plural forms together, like the CLI")
	assert.Contains(t, err.Error(), "set either output or outputs, not both")
}

// TestLoad_RejectsSingularOverrideWithOutputs pins that an embedded override
// of the singular form counts as setting it, as a CLI env var or flag does.
func TestLoad_RejectsSingularOverrideWithOutputs(t *testing.T) {
	_, err := config.Load([]byte("generator:\n  type: json\noutputs:\n  - type: nop\n"),
		config.LoadOpts{EnvOverrides: map[string]string{"output.type": "tcp", "output.tcp.host": "h"}})
	require.ErrorContains(t, err, "set either output or outputs, not both")
}

func TestLoad_RejectsEmptyOutputsList(t *testing.T) {
	_, err := config.Load([]byte("generator:\n  type: json\noutputs: []\n"), config.LoadOpts{})
	require.ErrorContains(t, err, "outputs must not be empty")
}

func TestLoad_RejectsSingularOverrideEqualToDefaultWithOutputs(t *testing.T) {
	// An override set to the default value is still an explicit singular form.
	_, err := config.Load([]byte("generator:\n  type: json\noutputs:\n  - type: stdout\n"),
		config.LoadOpts{EnvOverrides: map[string]string{"output.type": "nop"}})
	require.ErrorContains(t, err, "set either output or outputs, not both")
}

// TestLoad_OutputsEntryMigratesDeprecatedKey pins that a list entry's legacy
// otlpGrpc: sub-key is honored, as it is on the singular form, rather than
// dropped in favor of the type defaults.
func TestLoad_OutputsEntryMigratesDeprecatedKey(t *testing.T) {
	cfg, err := config.Load([]byte(`generator:
  type: json
outputs:
  - type: otlp-grpc
    otlpGrpc:
      host: collector.example
      port: 5317
`), config.LoadOpts{})
	require.NoError(t, err)
	require.Len(t, cfg.Outputs, 1)
	assert.Equal(t, "collector.example", cfg.Outputs[0].OTLPGrpc.Host)
	assert.Equal(t, 5317, cfg.Outputs[0].OTLPGrpc.Port)
}

// TestLoad_RejectsSingularOverrideWithGenerators pins that an embedded
// override of the singular generator form counts as setting it.
func TestLoad_RejectsSingularOverrideWithGenerators(t *testing.T) {
	_, err := config.Load([]byte("generators:\n  - type: json\noutput:\n  type: nop\n"),
		config.LoadOpts{EnvOverrides: map[string]string{"generator.type": "nginx"}})
	require.ErrorContains(t, err, "set either generator or generators, not both")
}

func TestLoad_RejectsGeneratorAndGenerators(t *testing.T) {
	_, err := config.Load([]byte(`
generator:
  type: json
generators:
  - type: json
`), config.LoadOpts{})
	require.ErrorContains(t, err, "set either generator or generators, not both",
		"embedded Load must reject singular and plural forms together, like the CLI")
}

func TestLoad_RejectsSingularOverrideEqualToDefaultWithGenerators(t *testing.T) {
	// An override set to the default value is still an explicit singular form.
	_, err := config.Load([]byte("generators:\n  - type: json\noutput:\n  type: nop\n"),
		config.LoadOpts{EnvOverrides: map[string]string{"generator.type": "nop"}})
	require.ErrorContains(t, err, "set either generator or generators, not both")
}

// TestLoad_GeneratorsEntryMigratesDeprecatedKey is the generators: mirror of
// TestLoad_OutputsEntryMigratesDeprecatedKey, for the legacy paloAlto: key.
func TestLoad_GeneratorsEntryMigratesDeprecatedKey(t *testing.T) {
	cfg, err := config.Load([]byte(`generators:
  - type: palo-alto
    paloAlto:
      workers: 7
output:
  type: nop
`), config.LoadOpts{})
	require.NoError(t, err)
	require.Len(t, cfg.Generators, 1)
	assert.Equal(t, 7, cfg.Generators[0].PaloAlto.Workers)
}

type flowSink struct{}

func (flowSink) ConsumeFlows(context.Context, []embed.FlowRecord) error { return nil }

func TestLoadModules_FlowUsesFlowConsumer(t *testing.T) {
	yaml := []byte(`
generator:
  type: flow
  flow:
    workers: 1
    rate: 1s
output:
  type: nop
logging:
  type: stdout
metrics:
  port: 19000
`)
	mods, err := config.LoadModules(yaml, config.EmbedOpts{FlowConsumer: flowSink{}})
	require.NoError(t, err)
	require.Len(t, mods, 1)
}

// TestLoadModules_FlowRequiresFlowConsumer pins the documented contract: a
// flow generator with no EmbedOpts.FlowConsumer is rejected at load.
func TestLoadModules_FlowRequiresFlowConsumer(t *testing.T) {
	_, err := config.LoadModules([]byte(`
generator:
  type: flow
  flow:
    workers: 1
    rate: 1s
output:
  type: nop
`), config.EmbedOpts{})
	require.ErrorContains(t, err, "FlowConsumer")
}

type recordingConsumer struct {
	mu   sync.Mutex
	recs []embed.LogRecord
}

func (r *recordingConsumer) ConsumeLogs(_ context.Context, recs []embed.LogRecord) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.recs = append(r.recs, recs...)
	return nil
}

func (r *recordingConsumer) first() (embed.LogRecord, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.recs) == 0 {
		return embed.LogRecord{}, false
	}
	return r.recs[0], true
}

// TestLoadModules_PaloAltoGetsEnvironmentFirewall pins the embedded path end
// to end: the environment is built to hold a Palo Alto device, and the
// generator describes it.
func TestLoadModules_PaloAltoGetsEnvironmentFirewall(t *testing.T) {
	yaml := []byte(`
generator:
  type: palo-alto
  palo-alto:
    workers: 1
    rate: 10ms
output:
  type: nop
logging:
  type: stdout
metrics:
  port: 19000
`)
	cons := &recordingConsumer{}
	mods, err := config.LoadModules(yaml, config.EmbedOpts{LogConsumer: cons})
	require.NoError(t, err)
	require.Len(t, mods, 1)
	require.NoError(t, mods[0].Start(context.Background()))
	require.Eventually(t, func() bool { _, ok := cons.first(); return ok }, 5*time.Second, 10*time.Millisecond)
	require.NoError(t, mods[0].Stop(context.Background()))
	rec, _ := cons.first()
	assert.Regexp(t, `^pa-fw-\d{2}$`, rec.Metadata.Resource["host.name"])
}
