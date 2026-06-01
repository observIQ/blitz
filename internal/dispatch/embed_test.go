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

func TestForEmbedWelReturnsProducerModule(t *testing.T) {
	cfg := config.Generator{
		Type: config.GeneratorTypeWel,
		Wel: config.WelGeneratorConfig{
			Workers: 1,
			Rate:    50 * time.Millisecond,
			Role:    "member",
		},
	}
	mod, err := ForEmbed(zap.NewNop(), cfg, noopConsumer{}, nil)
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
	mod, err := ForEmbed(zap.NewNop(), cfg, noopConsumer{}, nil)
	require.NoError(t, err)
	require.NotNil(t, mod)
}

func TestForEmbedWinevtRejectionMentionsWel(t *testing.T) {
	cfg := config.Generator{Type: config.GeneratorTypeWinevt}
	_, err := ForEmbed(zap.NewNop(), cfg, noopConsumer{}, nil)
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
	mod, err := ForEmbed(zap.NewNop(), cfg, noopConsumer{}, nil)
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
	_, err := ForEmbed(zap.NewNop(), cfg, noopConsumer{}, nil)
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
	_, err := ForEmbed(zap.NewNop(), cfg, noopConsumer{}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown asset category")
}

func TestForEmbedRejectsNilLogger(t *testing.T) {
	_, err := ForEmbed(nil, config.Generator{Type: config.GeneratorTypeFIX}, noopConsumer{}, nil)
	require.Error(t, err)
}

func TestForEmbedRejectsNilConsumer(t *testing.T) {
	_, err := ForEmbed(zap.NewNop(), config.Generator{Type: config.GeneratorTypeFIX}, nil, nil)
	require.Error(t, err)
}
