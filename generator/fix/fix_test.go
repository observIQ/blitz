package fix

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/log/logtest"
	"go.opentelemetry.io/otel/metric"
	metricnoop "go.opentelemetry.io/otel/metric/noop"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/observiq/blitz/embed"
	"github.com/observiq/blitz/generator/count"
	"github.com/observiq/blitz/generator/fix/catalog"
	"github.com/observiq/blitz/generator/fix/state"
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

// failingConsumer always returns an error, to exercise the write-error path.
type failingConsumer struct {
	mu    sync.Mutex
	calls int
}

func (c *failingConsumer) ConsumeLogs(context.Context, []embed.LogRecord) error {
	c.mu.Lock()
	c.calls++
	c.mu.Unlock()
	return errors.New("consume failed")
}

func (c *failingConsumer) Count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.calls
}

func hasMetric(rm metricdata.ResourceMetrics, name string) bool {
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name == name {
				return true
			}
		}
	}
	return false
}

// TestFIX_SelfTelemetry confirms the FIX generator routes its own metrics
// through the injected MeterProvider and bridges its logs into the injected
// LoggerProvider, matching every other embed-eligible generator. The failing
// consumer also exercises the write-error metric path.
func TestFIX_SelfTelemetry(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	logRec := logtest.NewRecorder()
	tel := embed.TelemetrySettings{
		Logger:         zap.NewNop(),
		MeterProvider:  mp,
		LoggerProvider: logRec,
	}

	cons := &failingConsumer{}
	// A caller constructing a generator directly bridges the logger itself,
	// as config.LoadModules does; blitz no longer re-bridges per component.
	g, err := New(tel.BridgedLogger(zap.NewNop()), Config{Workers: 1, Rate: 5 * time.Millisecond}, cons, tel)
	require.NoError(t, err)
	require.NoError(t, g.Start(context.Background()))

	require.Eventually(t,
		func() bool { return cons.Count() >= 1 },
		2*time.Second, 10*time.Millisecond,
		"expected the generator to attempt at least one emit",
	)

	stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, g.Stop(stopCtx))

	// Self-metrics reached the injected provider.
	var rm metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(context.Background(), &rm))
	require.True(t, hasMetric(rm, "blitz.generator.active_workers"), "active_workers gauge")
	require.True(t, hasMetric(rm, "blitz.generator.entries"), "entries counter")
	require.True(t, hasMetric(rm, "blitz.generator.write_errors"), "write_errors counter")

	// Internal logs bridged into the injected LoggerProvider.
	require.NotEmpty(t, logRec.Result(), "expected bridged log records")
}

// failingMeter embeds a no-op Meter and overrides one instrument constructor to
// return an error, so generator.NewMetrics fails.
type failingMeter struct {
	metric.Meter
}

func (failingMeter) Int64Gauge(string, ...metric.Int64GaugeOption) (metric.Int64Gauge, error) {
	return nil, errors.New("instrument error")
}

// failingMeterProvider hands out a failingMeter.
type failingMeterProvider struct {
	metric.MeterProvider
}

func (failingMeterProvider) Meter(string, ...metric.MeterOption) metric.Meter {
	return failingMeter{Meter: metricnoop.NewMeterProvider().Meter("test")}
}

// TestNew_MetricsBuildError covers the metric-construction error path in New.
func TestNew_MetricsBuildError(t *testing.T) {
	_, err := New(zap.NewNop(), DefaultConfig(), &captureConsumer{},
		embed.TelemetrySettings{MeterProvider: failingMeterProvider{}})
	require.Error(t, err)
	require.Contains(t, err.Error(), "build generator metrics")
}

func TestDefaultConfig(t *testing.T) {
	c := DefaultConfig()
	assert.Equal(t, 1, c.Workers)
	assert.Equal(t, time.Second, c.Rate)
	assert.Equal(t, catalog.V44, c.Version)
}

func TestNewRejectsNilLogger(t *testing.T) {
	_, err := New(nil, DefaultConfig(), &captureConsumer{}, embed.NopTelemetry())
	require.Error(t, err)
}

func TestNewRejectsNilConsumer(t *testing.T) {
	_, err := New(zap.NewNop(), DefaultConfig(), nil, embed.NopTelemetry())
	require.Error(t, err)
}

func TestNewRejectsZeroWorkers(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Workers = 0
	_, err := New(zap.NewNop(), cfg, &captureConsumer{}, embed.NopTelemetry())
	require.Error(t, err)
}

func TestNewRejectsNonPositiveRate(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Rate = 0
	_, err := New(zap.NewNop(), cfg, &captureConsumer{}, embed.NopTelemetry())
	require.Error(t, err)
}

func TestNewDefaultsVersionAndCompIDs(t *testing.T) {
	cfg := Config{Workers: 1, Rate: time.Second}
	g, err := New(zap.NewNop(), cfg, &captureConsumer{}, embed.NopTelemetry())
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
	}, cons, embed.NopTelemetry())
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
	// Same seed + same Start + same workers + same rate must produce identical
	// byte streams across two runs (within the prefix length common to
	// both).
	cfg := Config{Workers: 1, Rate: 5 * time.Millisecond, Version: catalog.V44, Seed: 12345, Start: testStart}

	runOnce := func() [][]byte {
		cons := &captureConsumer{}
		g, err := New(zap.NewNop(), cfg, cons, embed.NopTelemetry())
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
	}, cons, embed.NopTelemetry())
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
	}, cons, embed.NopTelemetry())
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
			}, cons, embed.NopTelemetry())
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
	g, err := New(zap.NewNop(), DefaultConfig(), &captureConsumer{}, embed.NopTelemetry())
	require.NoError(t, err)
	var _ embed.ProducerModule = g
}

// TestLiveSendingTimeFollowsTheClock pins that an unpinned run stamps
// SendingTime from the wall clock at send time, so a run that falls behind
// its rate (backpressure) never drifts behind real time.
func TestLiveSendingTimeFollowsTheClock(t *testing.T) {
	var mu sync.Mutex
	now := time.Date(2026, 9, 24, 14, 0, 0, 0, time.UTC)
	cons := &captureConsumer{}
	g, err := New(zap.NewNop(), Config{Workers: 1, Rate: time.Millisecond, Version: catalog.V44, Seed: 1}, cons, embed.NopTelemetry())
	require.NoError(t, err)
	fclock := func() time.Time {
		mu.Lock()
		defer mu.Unlock()
		return now
	}
	g.clock = fclock
	require.NoError(t, g.Start(context.Background()))
	require.Eventually(t, func() bool { return cons.Count() >= 1 }, 2*time.Second, time.Millisecond)
	mu.Lock()
	now = now.Add(time.Hour) // the run fell an hour behind its tick schedule
	mu.Unlock()
	seen := cons.Count()
	require.Eventually(t, func() bool { return cons.Count() > seen+1 }, 2*time.Second, time.Millisecond)
	require.NoError(t, g.Stop(context.Background()))
	msgs := cons.Snapshot()
	assert.Contains(t, string(msgs[len(msgs)-1]), "\x0152=20260924-15:00:00.000\x01")
}

// TestRandomSeedIsLoggedAndReproduces pins the seed contract: a negative seed
// is resolved once and logged at Start, and rerunning with it reproduces the
// stream.
func TestRandomSeedIsLoggedAndReproduces(t *testing.T) {
	run := func(seed int64) ([][]byte, int64) {
		core, logs := observer.New(zap.InfoLevel)
		cons := &captureConsumer{}
		g, err := New(zap.New(core), Config{Workers: 1, Rate: time.Millisecond, Version: catalog.V44, Seed: seed, Start: testStart}, cons, embed.NopTelemetry())
		require.NoError(t, err)
		require.NoError(t, g.Start(context.Background()))
		require.Eventually(t, func() bool { return cons.Count() >= 10 }, 2*time.Second, time.Millisecond)
		require.NoError(t, g.Stop(context.Background()))
		start := logs.FilterMessage("Starting FIX generator").All()
		require.Len(t, start, 1)
		logged, ok := start[0].ContextMap()["seed"].(int64)
		require.True(t, ok, "start log has no int64 seed field")
		return cons.Snapshot()[:10], logged
	}
	first, seed := run(-1)
	require.Greater(t, seed, int64(math.MaxInt32), "a random seed uses the full clock, not 31 bits")
	again, logged := run(seed)
	require.Equal(t, seed, logged)
	require.Equal(t, first, again)
}

func TestNewOrderExecutionReportsLeaveTheWholeOrder(t *testing.T) {
	var checked int
	for _, def := range catalog.AllDefinitions() {
		if def.MsgType != "8" {
			continue
		}
		r := rand.New(rand.NewSource(1)) // #nosec G404 -- deterministic test RNG
		for range 50 {
			ctx := &catalog.GenerateCtx{Version: def.Version, AssetCategory: def.AssetCategory}
			tags := map[catalog.Tag]string{}
			for _, gen := range def.Fields {
				f := gen(r, ctx)
				tags[f.Tag] = f.Value
			}
			if tags[14] != "0" {
				continue
			}
			checked++
			require.Equal(t, tags[38], tags[151], "%s %s: LeavesQty must equal OrderQty when CumQty is 0", def.Version, def.AssetCategory)
		}
	}
	require.NotZero(t, checked)
}

// TestCountBoundsTheRun pins that generator.count bounds a FIX run: the
// generator takes the run's count tracker and emits exactly count messages.
func TestCountBoundsTheRun(t *testing.T) {
	cons := &captureConsumer{}
	g, err := New(zap.NewNop(), Config{Workers: 2, Rate: time.Millisecond, Version: catalog.V44, Seed: 42}, cons, embed.NopTelemetry())
	require.NoError(t, err)
	counted, ok := any(g).(interface{ SetCountTracker(*count.Tracker) })
	require.True(t, ok, "the FIX generator must accept the run's count tracker")
	tracker := count.NewTracker(5)
	counted.SetCountTracker(tracker)

	require.NoError(t, g.Start(context.Background()))
	t.Cleanup(func() { _ = g.Stop(context.Background()) })
	require.Eventually(t, func() bool {
		select {
		case <-tracker.Done():
			return true
		default:
			return false
		}
	}, 5*time.Second, time.Millisecond)
	require.Never(t, func() bool { return cons.Count() > 5 }, 50*time.Millisecond, time.Millisecond)
	require.Equal(t, 5, cons.Count())
}

// TestCountResumeKeepsTheSequence pins that a worker out of count builds
// nothing: after a Reset its stream continues at the next MsgSeqNum, with no
// gap a counterparty would answer with a ResendRequest.
func TestCountResumeKeepsTheSequence(t *testing.T) {
	cons := &captureConsumer{}
	core, logs := observer.New(zap.DebugLevel)
	g, err := New(zap.New(core), Config{Workers: 1, Rate: time.Millisecond, Version: catalog.V44, Seed: 42}, cons, embed.NopTelemetry())
	require.NoError(t, err)
	tracker := count.NewTracker(2)
	g.SetCountTracker(tracker)
	require.NoError(t, g.Start(context.Background()))
	t.Cleanup(func() { _ = g.Stop(context.Background()) })

	// The worker has ticked past the spent count before the Reset.
	require.Eventually(t, func() bool {
		return logs.FilterMessage("FIX worker at the count limit, waiting").Len() > 0
	}, 5*time.Second, time.Millisecond)
	require.Equal(t, 2, cons.Count())
	tracker.Reset()
	require.Eventually(t, func() bool { return cons.Count() >= 3 }, 5*time.Second, time.Millisecond)

	for i, m := range cons.Snapshot()[:3] {
		require.Contains(t, string(m), fmt.Sprintf("\x0134=%d\x01", i+1))
	}
}

// TestEveryPickedMessageHasASkeleton pins the catalog coverage that makes the
// worker's nil-message branch a safety net: every message type a category
// can pick has a skeleton in every version.
func TestEveryPickedMessageHasASkeleton(t *testing.T) {
	r := rand.New(rand.NewSource(1)) // #nosec G404
	for _, v := range catalog.AllVersions() {
		for _, cat := range catalog.AllAssetCategories() {
			cfg := DefaultConfig()
			cfg.Version, cfg.EnabledCategories = v, []catalog.AssetCategory{cat}
			g, err := New(zap.NewNop(), cfg, &captureConsumer{}, embed.NopTelemetry())
			require.NoError(t, err)
			sess := state.NewSession("SENDER", "TARGET")
			for range 500 {
				require.NotNil(t, g.buildOneMessage(r, sess), "%s %s", v, cat)
			}
		}
	}
}
