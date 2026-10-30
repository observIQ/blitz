package dispatch

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/observiq/blitz/embed"
	"github.com/observiq/blitz/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// TestForEmbedFilegenWiresSeed pins that generator.filegen.seed reaches the
// generator: two modules built from the same seeded config pick the same lines.
func TestForEmbedFilegenWiresSeed(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lines.log")
	var data []byte
	for i := range 40 {
		data = append(data, []byte("line-"+strconv.Itoa(i)+"\n")...)
	}
	require.NoError(t, os.WriteFile(path, data, 0o600))
	cfg := config.Generator{
		Type:    config.GeneratorTypeFile,
		Filegen: config.FileGeneratorConfig{Workers: 1, Rate: time.Millisecond, Source: path, CacheEnabled: true, Mode: config.FileModeRandom, Seed: 7},
	}

	picks := func() []string {
		cons := &capturingLogConsumer{}
		mod, err := ForEmbed(zap.NewNop(), cfg, EmbedConsumers{LogConsumer: cons}, nil, nil, embed.NopTelemetry())
		require.NoError(t, err)
		require.NoError(t, mod.Start(context.Background()))
		require.Eventually(t, func() bool { return len(cons.snapshot()) >= 15 }, 5*time.Second, time.Millisecond)
		require.NoError(t, mod.Stop(context.Background()))
		var out []string
		for _, r := range cons.snapshot()[:15] {
			out = append(out, r.Message)
		}
		return out
	}
	assert.Equal(t, picks(), picks())
}

// TestForEmbedFilegenReplayRejectsWorkers pins that dispatch applies replay
// mode, which rejects more than one worker.
func TestForEmbedFilegenReplayRejectsWorkers(t *testing.T) {
	cfg := config.Generator{
		Type:    config.GeneratorTypeFile,
		Filegen: config.FileGeneratorConfig{Workers: 2, Rate: time.Millisecond, Source: "capture.log", Mode: config.FileModeReplay},
	}
	_, err := ForEmbed(zap.NewNop(), cfg, EmbedConsumers{LogConsumer: noopConsumer{}}, nil, nil, embed.NopTelemetry())
	assert.ErrorContains(t, err, "replay mode requires exactly 1 worker")
}
