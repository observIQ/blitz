package dispatch

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/observiq/blitz/embed"
	"github.com/observiq/blitz/internal/config"
	"github.com/observiq/blitz/internal/datagen"
)

type noopConsumer struct{}

func (noopConsumer) ConsumeLogs(_ context.Context, _ []embed.LogRecord) error { return nil }

type noopMetricConsumer struct{}

func (noopMetricConsumer) ConsumeMetrics(_ context.Context, _ []embed.MetricPoint) error {
	return nil
}

type noopTraceConsumer struct{}

func (noopTraceConsumer) ConsumeTraces(_ context.Context, _ []embed.Span) error { return nil }

func logsOnly() EmbedConsumers {
	return EmbedConsumers{LogConsumer: noopConsumer{}}
}

// capturingMetricConsumer records emitted points so a test can assert on the
// resource attributes the generator attached.
type capturingMetricConsumer struct {
	mu     sync.Mutex
	points []embed.MetricPoint
}

func (c *capturingMetricConsumer) ConsumeMetrics(_ context.Context, batch []embed.MetricPoint) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.points = append(c.points, batch...)
	return nil
}

func (c *capturingMetricConsumer) snapshot() []embed.MetricPoint {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]embed.MetricPoint, len(c.points))
	copy(out, c.points)
	return out
}

// capturingLogConsumer records emitted log records for resource assertions.
type capturingLogConsumer struct {
	mu      sync.Mutex
	records []embed.LogRecord
}

func (c *capturingLogConsumer) ConsumeLogs(_ context.Context, batch []embed.LogRecord) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.records = append(c.records, batch...)
	return nil
}

func (c *capturingLogConsumer) snapshot() []embed.LogRecord {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]embed.LogRecord, len(c.records))
	copy(out, c.records)
	return out
}

// TestForEmbedLogGeneratorWiresEnvironmentIdentity proves the setter path: a log
// generator built through ForEmbed with an environment has its host identity
// applied (via SetHostIdentity), so emitted records carry the simulated host.
func TestForEmbedLogGeneratorWiresEnvironmentIdentity(t *testing.T) {
	env := &datagen.Environment{
		Systems: []*datagen.SystemIdentity{{
			Hostname: "PANTHEON-LOG-01",
			OSInfo:   datagen.OSInfo{Type: datagen.OSLinux, Name: "Ubuntu"},
		}},
	}
	cons := &capturingLogConsumer{}
	cfg := config.Generator{
		Type:  config.GeneratorTypeNginx,
		Nginx: config.NginxGeneratorConfig{Workers: 1, Rate: 20 * time.Millisecond},
	}

	mod, err := ForEmbed(zap.NewNop(), cfg, EmbedConsumers{LogConsumer: cons}, nil, env)
	require.NoError(t, err)
	require.NoError(t, mod.Start(context.Background()))
	require.Eventually(t, func() bool { return len(cons.snapshot()) > 0 }, 2*time.Second, 10*time.Millisecond)
	require.NoError(t, mod.Stop(context.Background()))

	recs := cons.snapshot()
	require.NotEmpty(t, recs)
	assert.Equal(t, "PANTHEON-LOG-01", recs[0].Metadata.Resource["host.name"])
	assert.Equal(t, "nginx", recs[0].Metadata.Resource["telemetry.source"])
}

// TestForEmbedPropagatesConstructorError confirms applyHostIdentity forwards a
// constructor error (here nginx.New rejecting Workers=0) rather than trying to
// apply an identity to a nil module.
func TestForEmbedPropagatesConstructorError(t *testing.T) {
	cfg := config.Generator{
		Type:  config.GeneratorTypeNginx,
		Nginx: config.NginxGeneratorConfig{Workers: 0, Rate: time.Second},
	}
	mod, err := ForEmbed(zap.NewNop(), cfg, EmbedConsumers{LogConsumer: noopConsumer{}}, nil, nil)
	require.Error(t, err)
	require.Nil(t, mod)
}

// TestHostIdentityResolvesFromEnvironment covers the component-keyed identity
// resolution: a nil environment yields nil (process-hostname fallback), and a
// populated environment returns a deterministic SystemForKey selection.
func TestHostIdentityResolvesFromEnvironment(t *testing.T) {
	assert.Nil(t, hostIdentity(nil, config.GeneratorTypeHostMetrics))

	env := &datagen.Environment{
		Systems: []*datagen.SystemIdentity{
			{Hostname: "PANTHEON-01", OSInfo: datagen.OSInfo{Type: datagen.OSLinux}},
		},
	}
	got := hostIdentity(env, config.GeneratorTypeHostMetrics)
	require.NotNil(t, got)
	assert.Equal(t, "PANTHEON-01", got.Hostname)
}

// TestForEmbedHostMetricsWiresEnvironmentIdentity proves the full wiring: a
// hostmetrics module built through ForEmbed with an environment emits points
// carrying the resolved simulated host's identity attributes.
func TestForEmbedHostMetricsWiresEnvironmentIdentity(t *testing.T) {
	env := &datagen.Environment{
		Systems: []*datagen.SystemIdentity{{
			Hostname: "PANTHEON-01",
			HostID:   "id-1",
			Arch:     datagen.ArchAMD64,
			Tier:     datagen.TierProd,
			OSInfo:   datagen.OSInfo{Type: datagen.OSLinux, Name: "Ubuntu", Version: "22.04.5"},
		}},
	}
	cons := &capturingMetricConsumer{}
	cfg := config.Generator{
		Type: config.GeneratorTypeHostMetrics,
		HostMetrics: config.HostMetricsGeneratorConfig{
			Workers:  1,
			Rate:     20 * time.Millisecond,
			Scrapers: []string{"cpu"},
		},
	}

	mod, err := ForEmbed(zap.NewNop(), cfg, EmbedConsumers{MetricConsumer: cons}, nil, env)
	require.NoError(t, err)
	require.NoError(t, mod.Start(context.Background()))
	require.Eventually(t, func() bool { return len(cons.snapshot()) > 0 }, 2*time.Second, 10*time.Millisecond)
	require.NoError(t, mod.Stop(context.Background()))

	pts := cons.snapshot()
	require.NotEmpty(t, pts)
	res := pts[0].Metadata.Resource
	assert.Equal(t, "PANTHEON-01", res["host.name"])
	assert.Equal(t, "linux", res["os.type"])
	assert.Equal(t, "production", res["deployment.environment.name"])
}

func TestForEmbedWelReturnsProducerModule(t *testing.T) {
	cfg := config.Generator{
		Type: config.GeneratorTypeWel,
		Wel: config.WelGeneratorConfig{
			Workers: 1,
			Rate:    50 * time.Millisecond,
			Role:    "member",
		},
	}
	mod, err := ForEmbed(zap.NewNop(), cfg, logsOnly(), nil, nil)
	require.NoError(t, err)
	require.NotNil(t, mod)
	assert.Equal(t, "wel", mod.Name())
}

func TestForEmbedWelDefaultsEmptyRole(t *testing.T) {
	cfg := config.Generator{
		Type: config.GeneratorTypeWel,
		Wel: config.WelGeneratorConfig{
			Workers: 1,
			Rate:    50 * time.Millisecond,
		},
	}
	mod, err := ForEmbed(zap.NewNop(), cfg, logsOnly(), nil, nil)
	require.NoError(t, err)
	require.NotNil(t, mod)
}

func TestForEmbedWinevtRejectionMentionsWel(t *testing.T) {
	cfg := config.Generator{Type: config.GeneratorTypeWinevt}
	_, err := ForEmbed(zap.NewNop(), cfg, logsOnly(), nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "DEPRECATED")
	assert.Contains(t, err.Error(), "`wel` generator")
	assert.NotContains(t, err.Error(), "when it lands")
}

func TestForEmbedFIXReturnsProducerModule(t *testing.T) {
	cfg := config.Generator{
		Type: config.GeneratorTypeFIX,
		FIX: config.FIXGeneratorConfig{
			Workers: 1,
			Rate:    50 * time.Millisecond,
			Version: "4.4",
		},
	}
	mod, err := ForEmbed(zap.NewNop(), cfg, logsOnly(), nil, nil)
	require.NoError(t, err)
	require.NotNil(t, mod)
	assert.Equal(t, "fix", mod.Name())
}

func TestForEmbedFIXRejectsUnknownVersion(t *testing.T) {
	cfg := config.Generator{
		Type: config.GeneratorTypeFIX,
		FIX: config.FIXGeneratorConfig{
			Workers: 1,
			Rate:    time.Second,
			Version: "4.3",
		},
	}
	_, err := ForEmbed(zap.NewNop(), cfg, logsOnly(), nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown version")
}

func TestForEmbedFIXRejectsUnknownCategory(t *testing.T) {
	cfg := config.Generator{
		Type: config.GeneratorTypeFIX,
		FIX: config.FIXGeneratorConfig{
			Workers:           1,
			Rate:              time.Second,
			EnabledCategories: []string{"crypto"},
		},
	}
	_, err := ForEmbed(zap.NewNop(), cfg, logsOnly(), nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown asset category")
}

func TestForEmbedRejectsNilLogger(t *testing.T) {
	_, err := ForEmbed(nil, config.Generator{Type: config.GeneratorTypeFIX}, logsOnly(), nil, nil)
	require.Error(t, err)
}

func TestForEmbedRejectsMissingLogConsumerForLogType(t *testing.T) {
	_, err := ForEmbed(zap.NewNop(), config.Generator{Type: config.GeneratorTypeFIX}, EmbedConsumers{}, nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "LogConsumer")
}

// PIPE-1023 additions — hostmetrics dispatch.
func TestForEmbedHostMetricsReturnsProducerModule(t *testing.T) {
	cfg := config.Generator{
		Type: config.GeneratorTypeHostMetrics,
		HostMetrics: config.HostMetricsGeneratorConfig{
			Workers:  1,
			Rate:     50 * time.Millisecond,
			OS:       "linux",
			Hostname: "test-host",
		},
	}
	mod, err := ForEmbed(zap.NewNop(), cfg, EmbedConsumers{MetricConsumer: noopMetricConsumer{}}, nil, nil)
	require.NoError(t, err)
	require.NotNil(t, mod)
	assert.Equal(t, "hostmetrics", mod.Name())
}

func TestForEmbedHostMetricsRejectsMissingMetricConsumer(t *testing.T) {
	cfg := config.Generator{
		Type: config.GeneratorTypeHostMetrics,
		HostMetrics: config.HostMetricsGeneratorConfig{
			Workers: 1,
			Rate:    time.Second,
			OS:      "linux",
		},
	}
	_, err := ForEmbed(zap.NewNop(), cfg, EmbedConsumers{}, nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "MetricConsumer")
}

// PIPE-1024 additions — traces dispatch.
func TestForEmbedTracesReturnsProducerModule(t *testing.T) {
	cfg := config.Generator{
		Type: config.GeneratorTypeTraces,
		Traces: config.TracesGeneratorConfig{
			Workers: 1,
			Rate:    50 * time.Millisecond,
		},
	}
	mod, err := ForEmbed(zap.NewNop(), cfg, EmbedConsumers{TraceConsumer: noopTraceConsumer{}}, nil, nil)
	require.NoError(t, err)
	require.NotNil(t, mod)
	assert.Equal(t, "traces", mod.Name())
}

func TestForEmbedTracesRejectsMissingTraceConsumer(t *testing.T) {
	cfg := config.Generator{
		Type: config.GeneratorTypeTraces,
		Traces: config.TracesGeneratorConfig{
			Workers: 1,
			Rate:    time.Second,
		},
	}
	_, err := ForEmbed(zap.NewNop(), cfg, EmbedConsumers{}, nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "TraceConsumer")
}
