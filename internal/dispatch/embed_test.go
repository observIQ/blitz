package dispatch

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/observiq/blitz/embed"
	"github.com/observiq/blitz/internal/config"
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

func TestForEmbedWelReturnsProducerModule(t *testing.T) {
	cfg := config.Generator{
		Type: config.GeneratorTypeWel,
		Wel: config.WelGeneratorConfig{
			Workers: 1,
			Rate:    50 * time.Millisecond,
			Role:    "member",
		},
	}
	mod, err := ForEmbed(zap.NewNop(), cfg, logsOnly(), nil)
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
	mod, err := ForEmbed(zap.NewNop(), cfg, logsOnly(), nil)
	require.NoError(t, err)
	require.NotNil(t, mod)
}

func TestForEmbedWinevtRejectionMentionsWel(t *testing.T) {
	cfg := config.Generator{Type: config.GeneratorTypeWinevt}
	_, err := ForEmbed(zap.NewNop(), cfg, logsOnly(), nil)
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
	mod, err := ForEmbed(zap.NewNop(), cfg, logsOnly(), nil)
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
	_, err := ForEmbed(zap.NewNop(), cfg, logsOnly(), nil)
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
	_, err := ForEmbed(zap.NewNop(), cfg, logsOnly(), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown asset category")
}

func TestForEmbedRejectsNilLogger(t *testing.T) {
	_, err := ForEmbed(nil, config.Generator{Type: config.GeneratorTypeFIX}, logsOnly(), nil)
	require.Error(t, err)
}

func TestForEmbedRejectsMissingLogConsumerForLogType(t *testing.T) {
	_, err := ForEmbed(zap.NewNop(), config.Generator{Type: config.GeneratorTypeFIX}, EmbedConsumers{}, nil)
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
	mod, err := ForEmbed(zap.NewNop(), cfg, EmbedConsumers{MetricConsumer: noopMetricConsumer{}}, nil)
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
	_, err := ForEmbed(zap.NewNop(), cfg, EmbedConsumers{}, nil)
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
	mod, err := ForEmbed(zap.NewNop(), cfg, EmbedConsumers{TraceConsumer: noopTraceConsumer{}}, nil)
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
	_, err := ForEmbed(zap.NewNop(), cfg, EmbedConsumers{}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "TraceConsumer")
}
