package config_test

import (
	"context"
	"sync"
	"testing"

	"github.com/observiq/blitz/config"
	"github.com/observiq/blitz/embed"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
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
    workers: 1
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
