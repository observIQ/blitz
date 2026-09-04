package fix

import (
	"bytes"
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/observiq/blitz/embed"
	"github.com/observiq/blitz/generator/fix/catalog"
	"github.com/observiq/blitz/internal/datagen"
)

// captureConsumer buffers every record passed to ConsumeLogs.
type captureConsumer struct {
	mu        sync.Mutex
	got       [][]byte
	resources []map[string]any
}

func (c *captureConsumer) ConsumeLogs(_ context.Context, records []embed.LogRecord) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, r := range records {
		cpy := make([]byte, len(r.Message))
		copy(cpy, r.Message)
		c.got = append(c.got, cpy)
		c.resources = append(c.resources, r.Metadata.Resource)
	}
	return nil
}

func (c *captureConsumer) Snapshot() [][]byte {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([][]byte, len(c.got))
	copy(out, c.got)
	return out
}

func (c *captureConsumer) ResourceAt(i int) map[string]any {
	c.mu.Lock()
	defer c.mu.Unlock()
	if i < 0 || i >= len(c.resources) {
		return nil
	}
	return c.resources[i]
}

func (c *captureConsumer) Count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.got)
}

func TestDefaultConfig(t *testing.T) {
	c := DefaultConfig()
	assert.Equal(t, 1, c.Workers)
	assert.Equal(t, time.Second, c.Rate)
	assert.Equal(t, catalog.V44, c.Version)
}

func TestNewRejectsNilLogger(t *testing.T) {
	_, err := New(nil, DefaultConfig(), &captureConsumer{})
	require.Error(t, err)
}

func TestNewRejectsNilConsumer(t *testing.T) {
	_, err := New(zap.NewNop(), DefaultConfig(), nil)
	require.Error(t, err)
}

func TestNewRejectsZeroWorkers(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Workers = 0
	_, err := New(zap.NewNop(), cfg, &captureConsumer{})
	require.Error(t, err)
}

func TestNewRejectsNonPositiveRate(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Rate = 0
	_, err := New(zap.NewNop(), cfg, &captureConsumer{})
	require.Error(t, err)
}

func TestNewDefaultsVersionAndCompIDs(t *testing.T) {
	cfg := Config{Workers: 1, Rate: time.Second}
	g, err := New(zap.NewNop(), cfg, &captureConsumer{})
	require.NoError(t, err)
	assert.Equal(t, catalog.V44, g.cfg.Version)
	assert.Equal(t, "BLITZ", g.cfg.SenderCompID)
	assert.Equal(t, "VENUE", g.cfg.TargetCompID)
	assert.Equal(t, catalog.AllAssetCategories(), g.cfg.EnabledCategories)
}

func TestEmitsMessagesAtRate(t *testing.T) {
	cons := &captureConsumer{}
	g, err := New(zap.NewNop(), Config{
		Workers: 1,
		Rate:    20 * time.Millisecond,
		Version: catalog.V44,
		Seed:    42,
	}, cons)
	require.NoError(t, err)

	require.NoError(t, g.Start(context.Background()))
	require.Eventually(t, func() bool { return cons.Count() >= 3 }, 2*time.Second, 10*time.Millisecond)
	stopCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	require.NoError(t, g.Stop(stopCtx))

	msgs := cons.Snapshot()
	for i, m := range msgs {
		assert.True(t, bytes.HasPrefix(m, []byte("8=FIX.4.4\x01")),
			"message %d does not start with FIX.4.4 BeginString: %q", i, m)
	}
}

func TestGoldenOutputDeterministicFromSeed(t *testing.T) {
	// Same seed + same workers + same rate must produce identical
	// byte streams across two runs (within the prefix length common to
	// both).
	cfg := Config{Workers: 1, Rate: 5 * time.Millisecond, Version: catalog.V44, Seed: 12345}

	runOnce := func() [][]byte {
		cons := &captureConsumer{}
		g, err := New(zap.NewNop(), cfg, cons)
		require.NoError(t, err)
		require.NoError(t, g.Start(context.Background()))
		require.Eventually(t, func() bool { return cons.Count() >= 5 }, 2*time.Second, 5*time.Millisecond)
		stopCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		require.NoError(t, g.Stop(stopCtx))
		return cons.Snapshot()
	}

	a := runOnce()
	b := runOnce()

	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	require.Positive(t, n, "neither run produced messages")
	for i := 0; i < n; i++ {
		assert.True(t, bytes.Equal(a[i], b[i]),
			"seed-12345 message %d differs across runs:\nA=%q\nB=%q", i, a[i], b[i])
	}
}

func TestV50SP2EmitsFIXTBeginString(t *testing.T) {
	cons := &captureConsumer{}
	g, err := New(zap.NewNop(), Config{
		Workers: 1,
		Rate:    20 * time.Millisecond,
		Version: catalog.V50SP2,
		Seed:    42,
	}, cons)
	require.NoError(t, err)

	require.NoError(t, g.Start(context.Background()))
	require.Eventually(t, func() bool { return cons.Count() >= 1 }, 2*time.Second, 10*time.Millisecond)
	stopCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	require.NoError(t, g.Stop(stopCtx))

	for i, m := range cons.Snapshot() {
		assert.True(t, bytes.HasPrefix(m, []byte("8=FIXT.1.1\x01")),
			"v50sp2 message %d should start with FIXT.1.1, got %q", i, m)
	}
}

func TestV42EmitsFIX42BeginString(t *testing.T) {
	cons := &captureConsumer{}
	g, err := New(zap.NewNop(), Config{
		Workers: 1,
		Rate:    20 * time.Millisecond,
		Version: catalog.V42,
		Seed:    42,
	}, cons)
	require.NoError(t, err)

	require.NoError(t, g.Start(context.Background()))
	require.Eventually(t, func() bool { return cons.Count() >= 1 }, 2*time.Second, 10*time.Millisecond)
	stopCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	require.NoError(t, g.Stop(stopCtx))

	for i, m := range cons.Snapshot() {
		assert.True(t, bytes.HasPrefix(m, []byte("8=FIX.4.2\x01")),
			"v42 message %d should start with FIX.4.2, got %q", i, m)
	}
}

// TestEmitsResourceWithHostNameAndFixVersion asserts that every emitted
// LogRecord carries the conventional Resource keys per PIPE-1021: at
// minimum host.name and telemetry.source, plus the version-specific
// fix.version key so downstream consumers can pivot on protocol version
// without parsing the message body.
func TestEmitsResourceWithHostNameAndFixVersion(t *testing.T) {
	for _, tc := range []struct {
		name        string
		version     catalog.Version
		wantVersion string
	}{
		{"4.2", catalog.V42, "FIX.4.2"},
		{"4.4", catalog.V44, "FIX.4.4"},
		{"5.0sp2", catalog.V50SP2, "FIX.5.0SP2"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cons := &captureConsumer{}
			g, err := New(zap.NewNop(), Config{
				Workers: 1,
				Rate:    20 * time.Millisecond,
				Version: tc.version,
				Seed:    42,
			}, cons)
			require.NoError(t, err)

			require.NoError(t, g.Start(context.Background()))
			require.Eventually(t, func() bool { return cons.Count() >= 1 }, 2*time.Second, 10*time.Millisecond)
			stopCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			require.NoError(t, g.Stop(stopCtx))

			res := cons.ResourceAt(0)
			require.NotNil(t, res, "emitted record must carry a non-nil Resource")
			assert.NotEmpty(t, res["host.name"], "Resource must include host.name")
			assert.Equal(t, "fix", res["telemetry.source"], "telemetry.source must be %q", "fix")
			assert.Equal(t, tc.wantVersion, res["fix.version"], "fix.version must reflect the configured Version")
		})
	}
}

// TestGeneratorSatisfiesProducerModule is a compile-time check that
// *Generator is embed-eligible. If embed.ProducerMarker is removed or
// the Module interface changes, this fails to compile.
func TestGeneratorSatisfiesProducerModule(t *testing.T) {
	g, err := New(zap.NewNop(), DefaultConfig(), &captureConsumer{})
	require.NoError(t, err)
	var _ embed.ProducerModule = g
}

func TestSetHostIdentity(t *testing.T) {
	g, err := New(zap.NewNop(), DefaultConfig(), &captureConsumer{})
	require.NoError(t, err)

	g.SetHostIdentity(&datagen.SystemIdentity{
		Hostname: "IDENTITY-HOST",
		OSInfo:   datagen.OSInfo{Type: datagen.OSLinux},
	})
	assert.Equal(t, "IDENTITY-HOST", g.static.Record()["host.name"])

	g.SetHostIdentity(nil)
	assert.NotEmpty(t, g.static.Record()["host.name"])
}
