package f5

import (
	"context"
	"math/rand"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/observiq/blitz/embed"
	"github.com/observiq/blitz/telemetry"
)

func rngFor(seed int64) *rand.Rand { return rand.New(rand.NewSource(seed)) }

type captureConsumer struct {
	mu  sync.Mutex
	got []string
}

func (c *captureConsumer) ConsumeLogs(_ context.Context, records []embed.LogRecord) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, r := range records {
		c.got = append(c.got, r.Message)
	}
	return nil
}

func (c *captureConsumer) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.got)
}

func newGen(t *testing.T, cfg Config) (*Generator, *captureConsumer) {
	t.Helper()
	cc := &captureConsumer{}
	g, err := New(zap.NewNop(), cfg, cc, embed.TelemetrySettings{})
	require.NoError(t, err)
	return g, cc
}

func TestNewValidates(t *testing.T) {
	cc := &captureConsumer{}
	_, err := New(nil, DefaultConfig(), cc, embed.TelemetrySettings{})
	require.Error(t, err)
	_, err = New(zap.NewNop(), DefaultConfig(), nil, embed.TelemetrySettings{})
	require.Error(t, err)
	_, err = New(zap.NewNop(), Config{Workers: 0, Rate: time.Second}, cc, embed.TelemetrySettings{})
	require.Error(t, err)
	_, err = New(zap.NewNop(), Config{Workers: 1, Rate: 0}, cc, embed.TelemetrySettings{})
	require.Error(t, err)
}

func TestUnknownProductErrors(t *testing.T) {
	cc := &captureConsumer{}
	_, err := New(zap.NewNop(), Config{Workers: 1, Rate: time.Second, EnabledProducts: []string{"nope"}}, cc, embed.TelemetrySettings{})
	require.ErrorContains(t, err, "unknown f5 product")
}

func TestAllTenProductsRegistered(t *testing.T) {
	g, _ := newGen(t, DefaultConfig())
	require.Len(t, g.products, 10)
}

func TestEnabledProductsFilter(t *testing.T) {
	g, _ := newGen(t, Config{Workers: 1, Rate: time.Second, EnabledProducts: []string{"asm"}})
	require.Len(t, g.products, 1)
	// Every built line must be an ASM line.
	for i := 0; i < 50; i++ {
		line := g.buildLine(rngFor(int64(i)))
		require.Contains(t, line, " ASM: ")
	}
}

func TestDeterministicFromSeed(t *testing.T) {
	g, _ := newGen(t, Config{Workers: 1, Rate: time.Second, Hostname: "bigip1"})
	a := g.buildLine(rngFor(99))
	b := g.buildLine(rngFor(99))
	require.Equal(t, a, b)
}

func TestWeightsBiasSelection(t *testing.T) {
	// Weight asm >> the rest; asm should dominate the mix.
	g, _ := newGen(t, Config{Workers: 1, Rate: time.Second, Weights: map[string]float64{"asm": 1000}})
	asm := 0
	for i := 0; i < 500; i++ {
		if strings.Contains(g.buildLine(rngFor(int64(i))), " ASM: ") {
			asm++
		}
	}
	require.Greater(t, asm, 450, "asm weight 1000 should dominate")
}

func TestStartStopEmits(t *testing.T) {
	g, cc := newGen(t, Config{Workers: 2, Rate: 5 * time.Millisecond, Seed: 1})
	require.NoError(t, g.Start(context.Background()))
	require.Eventually(t, func() bool { return cc.count() > 0 }, 2*time.Second, 10*time.Millisecond)
	require.NoError(t, g.Stop(context.Background()))
}

func TestSupportedTelemetry(t *testing.T) {
	g, _ := newGen(t, DefaultConfig())
	require.Equal(t, []telemetry.Type{telemetry.Logs}, g.SupportedTelemetry())
}
