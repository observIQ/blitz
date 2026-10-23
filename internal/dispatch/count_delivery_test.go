package dispatch

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/observiq/blitz/embed"
	"github.com/observiq/blitz/generator/count"
	"github.com/observiq/blitz/internal/config"
)

// flakyConsumer rejects its first failFirst deliveries, then accepts every
// one, for every signal.
type flakyConsumer struct {
	failFirst int64
	calls     atomic.Int64
	delivered atomic.Int64
}

func (c *flakyConsumer) consume() error {
	if c.calls.Add(1) <= c.failFirst {
		return errors.New("destination unavailable")
	}
	c.delivered.Add(1)
	return nil
}

func (c *flakyConsumer) ConsumeLogs(context.Context, []embed.LogRecord) error { return c.consume() }
func (c *flakyConsumer) ConsumeMetrics(context.Context, []embed.MetricPoint) error {
	return c.consume()
}
func (c *flakyConsumer) ConsumeTraces(context.Context, []embed.Span) error      { return c.consume() }
func (c *flakyConsumer) ConsumeFlows(context.Context, []embed.FlowRecord) error { return c.consume() }

// fastDefaults is the declared defaults with every generator at one worker
// and a 1ms rate.
func fastDefaults(t *testing.T) config.Generator {
	t.Helper()
	v := viper.New()
	config.SetDefaults(v)
	for _, o := range config.DefaultOverrides() {
		switch {
		case strings.HasSuffix(o.Field, ".rate"):
			v.Set(o.Field, time.Millisecond)
		case strings.HasSuffix(o.Field, ".workers"):
			v.Set(o.Field, 1)
		}
	}
	cfg := config.NewConfig()
	require.NoError(t, v.Unmarshal(cfg))
	return cfg.Generator
}

// TestCountRunEndsOnDeliveredRecords pins that a count run ends only once
// its records are delivered: failed deliveries give their permit back, so
// Done does not close on attempts that never reached the destination.
func TestCountRunEndsOnDeliveredRecords(t *testing.T) {
	source := filepath.Join(t.TempDir(), "in.log")
	require.NoError(t, os.WriteFile(source, []byte("a\nb\nc\n"), 0o600))

	types := []config.GeneratorType{
		config.GeneratorTypeJSON,
		config.GeneratorTypePaloAlto,
		config.GeneratorTypeApache,
		config.GeneratorTypeApacheCombined,
		config.GeneratorTypeApacheError,
		config.GeneratorTypeNginx,
		config.GeneratorTypePostgres,
		config.GeneratorTypeKubernetes,
		config.GeneratorTypeFile,
		config.GeneratorTypeOkta,
		config.GeneratorTypeHostMetrics,
		config.GeneratorTypeTraces,
		config.GeneratorTypeWel,
		config.GeneratorTypeFIX,
		config.GeneratorTypeFlow,
		config.GeneratorTypeF5,
	}
	for _, typ := range types {
		for _, mode := range []string{config.FileModeRandom, config.FileModeReplay} {
			if mode == config.FileModeReplay && typ != config.GeneratorTypeFile {
				continue
			}
			t.Run(string(typ)+"/"+mode, func(t *testing.T) {
				t.Parallel()
				const want = 3
				genCfg := fastDefaults(t)
				genCfg.Type = typ
				genCfg.Filegen.Source = source
				genCfg.Filegen.Mode = mode

				// One rejection is enough: under close-on-take the run ends with
				// want-1 records delivered.
				c := &flakyConsumer{failFirst: 1}
				mod, err := ForEmbed(zap.NewNop(), genCfg, EmbedConsumers{
					LogConsumer: c, MetricConsumer: c, TraceConsumer: c, FlowConsumer: c,
				}, nil, nil, embed.TelemetrySettings{})
				require.NoError(t, err)

				tracker := count.NewTracker(want)
				mod.(interface{ SetCountTracker(*count.Tracker) }).SetCountTracker(tracker)
				require.NoError(t, mod.Start(context.Background()))
				t.Cleanup(func() { _ = mod.Stop(context.Background()) })

				select {
				case <-tracker.Done():
				case <-time.After(10 * time.Second):
					t.Fatalf("count never completed: %d calls, %d delivered", c.calls.Load(), c.delivered.Load())
				}
				require.GreaterOrEqual(t, c.delivered.Load(), int64(want),
					"the run finished before %d records were delivered", want)
				require.Equal(t, int64(want), tracker.Emitted())
			})
		}
	}
}
