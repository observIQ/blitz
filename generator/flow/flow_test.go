package flow

import (
	"context"
	"errors"
	"math"
	"math/rand"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/observiq/blitz/embed"
	"github.com/observiq/blitz/generator"
	"github.com/observiq/blitz/generator/count"
	"github.com/observiq/blitz/internal/datagen"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/metric"
	metricnoop "go.opentelemetry.io/otel/metric/noop"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

// captureConsumer records every flow it receives; err, when set, fails each call.
type captureConsumer struct {
	mu   sync.Mutex
	recs []embed.FlowRecord
	err  error
}

func (c *captureConsumer) ConsumeFlows(_ context.Context, r []embed.FlowRecord) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.recs = append(c.recs, r...)
	return c.err
}

func (c *captureConsumer) got() []embed.FlowRecord {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]embed.FlowRecord(nil), c.recs...)
}

func TestNewRejectsBadInput(t *testing.T) {
	c := &captureConsumer{}
	_, err := New(nil, 1, time.Millisecond, ScenarioDefault, 1, c, embed.NopTelemetry())
	require.Error(t, err)
	_, err = New(zap.NewNop(), 0, time.Millisecond, ScenarioDefault, 1, c, embed.NopTelemetry())
	require.Error(t, err)
	_, err = New(zap.NewNop(), 1, time.Millisecond, ScenarioDefault, 1, nil, embed.NopTelemetry())
	require.Error(t, err)
	_, err = New(zap.NewNop(), 1, time.Millisecond, "bogus", 1, c, embed.NopTelemetry())
	require.Error(t, err)
	_, err = New(zap.NewNop(), 1, 0, ScenarioDefault, 1, c, embed.NopTelemetry())
	require.ErrorContains(t, err, "rate")
	_, err = New(zap.NewNop(), 1, -time.Second, ScenarioDefault, 1, c, embed.NopTelemetry())
	require.ErrorContains(t, err, "rate")
}

func TestLifecycleEmitsFlowsWithResource(t *testing.T) {
	c := &captureConsumer{}
	dev := &datagen.NetworkSystemIdentity{
		Hostname: "cisco-rtr-01", Vendor: datagen.VendorCisco, Model: "Catalyst 9500-48Y4C",
		ManagementInterface: &datagen.NetworkInterface{Name: "mgmt0", IPv4: "10.10.40.7"},
		L3Routing:           &datagen.L3RoutingCapability{},
	}
	g, err := New(zap.NewNop(), 2, time.Millisecond, "", 5, c, embed.NopTelemetry(), generator.WithAppliance(dev))
	require.NoError(t, err)
	require.Equal(t, "flow", g.Name())
	require.NoError(t, g.Start(context.Background()))
	require.Eventually(t, func() bool { return len(c.got()) >= 10 }, 5*time.Second, time.Millisecond)
	require.NoError(t, g.Stop(context.Background()))

	for _, r := range c.got() {
		require.Equal(t, "cisco-rtr-01", r.Metadata.Resource["host.name"])
		require.Equal(t, "Cisco", r.Metadata.Resource["device.manufacturer"])
		require.Equal(t, "flow", r.Metadata.Resource["telemetry.source"])
		require.Equal(t, "10.10.40.7", r.ExporterIP.String(), "the device's management address exports the flows")
		require.False(t, r.Metadata.Timestamp.IsZero())
	}
}

func TestDefaultResourceUsesProcessHost(t *testing.T) {
	c := &captureConsumer{}
	g, err := New(zap.NewNop(), 1, time.Millisecond, ScenarioDefault, 1, c, embed.NopTelemetry())
	require.NoError(t, err)
	require.NoError(t, g.Start(context.Background()))
	require.Eventually(t, func() bool { return len(c.got()) >= 1 }, 5*time.Second, time.Millisecond)
	require.NoError(t, g.Stop(context.Background()))
	require.NotEmpty(t, c.got()[0].Metadata.Resource["host.name"])
}

func TestCountTrackerStopsEmission(t *testing.T) {
	c := &captureConsumer{}
	g, err := New(zap.NewNop(), 3, time.Millisecond, ScenarioDefault, 1, c, embed.NopTelemetry())
	require.NoError(t, err)
	tr := count.NewTracker(7)
	g.SetCountTracker(tr)
	require.NoError(t, g.Start(context.Background()))
	require.Eventually(t, func() bool { return len(c.got()) == 7 }, 5*time.Second, time.Millisecond)
	require.Never(t, func() bool { return len(c.got()) > 7 }, 50*time.Millisecond, time.Millisecond)
	require.NoError(t, g.Stop(context.Background()))
}

func TestConsumerErrorKeepsRunning(t *testing.T) {
	c := &captureConsumer{err: errors.New("collector down")}
	g, err := New(zap.NewNop(), 1, time.Millisecond, ScenarioDefault, 1, c, embed.NopTelemetry())
	require.NoError(t, err)
	require.NoError(t, g.Start(context.Background()))
	require.Eventually(t, func() bool { return len(c.got()) >= 3 }, 5*time.Second, time.Millisecond)
	require.NoError(t, g.Stop(context.Background()))
}

func TestSupportedTelemetryIsFlows(t *testing.T) {
	g, err := New(zap.NewNop(), 1, time.Second, ScenarioDefault, 1, &captureConsumer{}, embed.NopTelemetry())
	require.NoError(t, err)
	require.Equal(t, "flows", string(g.SupportedTelemetry()[0]))
}

// TestTrackerResetResumes: a worker idling on a spent tracker resumes on Reset.
func TestTrackerResetResumes(t *testing.T) {
	c := &captureConsumer{}
	g, err := New(zap.NewNop(), 1, time.Millisecond, ScenarioDefault, 1, c, embed.NopTelemetry())
	require.NoError(t, err)
	tr := count.NewTracker(2)
	g.SetCountTracker(tr)
	require.NoError(t, g.Start(context.Background()))
	require.Eventually(t, func() bool { return len(c.got()) == 2 }, 5*time.Second, time.Millisecond)
	tr.Reset()
	require.Eventually(t, func() bool { return len(c.got()) == 4 }, 5*time.Second, time.Millisecond)
	require.NoError(t, g.Stop(context.Background()))
}

// blockingConsumer holds every ConsumeFlows call until release is closed.
type blockingConsumer struct{ entered, release chan struct{} }

func (b blockingConsumer) ConsumeFlows(context.Context, []embed.FlowRecord) error {
	select {
	case b.entered <- struct{}{}:
	default:
	}
	<-b.release
	return nil
}

func TestStopTimesOutOnStuckConsumer(t *testing.T) {
	b := blockingConsumer{entered: make(chan struct{}, 1), release: make(chan struct{})}
	g, err := New(zap.NewNop(), 1, time.Millisecond, ScenarioDefault, 1, b, embed.NopTelemetry())
	require.NoError(t, err)
	require.NoError(t, g.Start(context.Background()))
	<-b.entered
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	require.ErrorContains(t, g.Stop(ctx), "context cancel")
	close(b.release)
}

// TestRandomSeedIsLoggedAndReproduces pins the seed contract: a negative seed
// is resolved once and logged at Start, and rerunning with it reproduces the
// flows (compared without their wall-clock timestamps).
func TestRandomSeedIsLoggedAndReproduces(t *testing.T) {
	run := func(seed int64) ([]embed.FlowRecord, int64) {
		core, logs := observer.New(zap.InfoLevel)
		cc := &captureConsumer{}
		g, err := New(zap.New(core), 1, time.Millisecond, ScenarioDefault, seed, cc, embed.NopTelemetry())
		require.NoError(t, err)
		require.NoError(t, g.Start(context.Background()))
		require.Eventually(t, func() bool { return len(cc.got()) >= 10 }, 2*time.Second, time.Millisecond)
		require.NoError(t, g.Stop(context.Background()))
		start := logs.FilterMessage("Starting flow generator").All()
		require.Len(t, start, 1)
		logged, ok := start[0].ContextMap()["seed"].(int64)
		require.True(t, ok, "start log has no int64 seed field")
		recs := cc.got()[:10]
		for i := range recs {
			recs[i].StartTime, recs[i].EndTime, recs[i].Metadata = time.Time{}, time.Time{}, embed.FlowRecordMetadata{}
		}
		return recs, logged
	}
	first, seed := run(-1)
	require.Greater(t, seed, int64(math.MaxInt32), "a random seed uses the full clock, like the other generators, not 31 bits that repeat every ~2 s")
	again, logged := run(seed)
	require.Equal(t, seed, logged)
	require.Equal(t, first, again)
}

// TestFlowsNameOneExporter pins that a generator describes one exporting
// device: every record carries the same private ExporterIP, drawn from the
// seed so a seeded run reproduces it.
func TestFlowsNameOneExporter(t *testing.T) {
	run := func(seed int64) []embed.FlowRecord {
		cc := &captureConsumer{}
		g, err := New(zap.NewNop(), 2, time.Millisecond, ScenarioDefault, seed, cc, embed.NopTelemetry())
		require.NoError(t, err)
		require.NoError(t, g.Start(context.Background()))
		require.Eventually(t, func() bool { return len(cc.got()) >= 20 }, 2*time.Second, time.Millisecond)
		require.NoError(t, g.Stop(context.Background()))
		return cc.got()
	}
	recs := run(7)
	exporter := recs[0].ExporterIP
	require.NotNil(t, exporter)
	require.True(t, exporter.IsPrivate(), "exporter %s is a private device address", exporter)
	for _, r := range recs {
		require.True(t, r.ExporterIP.Equal(exporter), "one exporter per generator")
	}
	require.True(t, run(7)[0].ExporterIP.Equal(exporter), "a seed reproduces the exporter")
}

// TestExporterDrawIsIndependentOfWorkers pins that the exporter is drawn from
// its own stream, not worker 0's (seed+0), so it never mirrors that worker's
// first private address.
func TestExporterDrawIsIndependentOfWorkers(t *testing.T) {
	for seed := int64(0); seed < 20; seed++ {
		g, err := New(zap.NewNop(), 1, time.Hour, ScenarioDefault, seed, &captureConsumer{}, embed.NopTelemetry())
		require.NoError(t, err)
		require.NoError(t, g.Start(context.Background()))
		require.NoError(t, g.Stop(context.Background()))
		worker0 := datagen.RandomPrivateIPv4(rand.New(rand.NewSource(seed))) // #nosec G404 -- test
		require.NotEqual(t, worker0, g.exporter.String(), "seed %d", seed)
	}
}

// TestDDoSVictimIsAPublicAddressFromTheSeed pins that the ddos-target victim is
// a routable public address drawn from the seed, not one fixed documentation
// address (TEST-NET-3) shared by every run.
func TestDDoSVictimIsAPublicAddressFromTheSeed(t *testing.T) {
	victim := func(seed int64) net.IP {
		cc := &captureConsumer{}
		g, err := New(zap.NewNop(), 1, time.Millisecond, ScenarioDDOSTarget, seed, cc, embed.NopTelemetry())
		require.NoError(t, err)
		require.NoError(t, g.Start(context.Background()))
		require.Eventually(t, func() bool { return len(cc.got()) >= 5 }, 2*time.Second, time.Millisecond)
		require.NoError(t, g.Stop(context.Background()))
		recs := cc.got()
		for _, r := range recs {
			require.True(t, r.DstIP.Equal(recs[0].DstIP), "one victim per run")
		}
		return recs[0].DstIP
	}
	_, testNet3, _ := net.ParseCIDR("203.0.113.0/24")
	a, b := victim(1), victim(2)
	for _, v := range []net.IP{a, b} {
		require.False(t, testNet3.Contains(v), "victim %s is a documentation address", v)
		require.False(t, v.IsPrivate() || v.IsLoopback() || v.IsUnspecified(), "victim %s is public", v)
	}
	require.False(t, a.Equal(b), "the victim comes from the seed")
}

// timedFailConsumer fails every write and records when each was attempted.
type timedFailConsumer struct {
	mu    sync.Mutex
	calls []time.Time
}

func (c *timedFailConsumer) ConsumeFlows(context.Context, []embed.FlowRecord) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls = append(c.calls, time.Now())
	return errors.New("connection refused")
}

func (c *timedFailConsumer) snapshot() []time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]time.Time(nil), c.calls...)
}

// TestFailedWritesBackOff pins that a worker whose writes fail backs off
// exponentially, like the log generators, instead of retrying every tick.
func TestFailedWritesBackOff(t *testing.T) {
	const rate = time.Millisecond
	cc := &timedFailConsumer{}
	g, err := New(zap.NewNop(), 1, rate, ScenarioDefault, 1, cc, embed.NopTelemetry())
	require.NoError(t, err)
	require.NoError(t, g.Start(context.Background()))
	require.Eventually(t, func() bool { return len(cc.snapshot()) >= 8 }, 5*time.Second, time.Millisecond)
	require.NoError(t, g.Stop(context.Background()))
	calls := cc.snapshot()
	// The 8th retry waits 1.5^7 x rate (17 ms; no jitter), far past one tick.
	require.GreaterOrEqual(t, calls[7].Sub(calls[6]), 4*rate, "retries back off")
}

// TestSendsFollowTheRate pins that a healthy worker waits exactly rate
// between sends: the write backoff adds no jitter.
func TestSendsFollowTheRate(t *testing.T) {
	bo := newWriteBackoff(100 * time.Millisecond)
	for range 5 {
		require.Equal(t, 100*time.Millisecond, bo.NextBackOff())
		bo.Reset()
	}
}

// TestResumeSendsAtTheRate pins that a worker resumed after its count ran
// out sends one rate after the Reset, not the backoff's next, longer step.
func TestResumeSendsAtTheRate(t *testing.T) {
	const rate = time.Second
	ctx := context.Background()
	core, logs := observer.New(zap.DebugLevel)
	c := &captureConsumer{}
	g, err := New(zap.New(core), 1, rate, ScenarioDefault, 1, c, embed.NopTelemetry())
	require.NoError(t, err)
	clk := clockwork.NewFakeClock()
	g.clock = clk
	tr := count.NewTracker(1)
	g.SetCountTracker(tr)
	require.NoError(t, g.Start(ctx))
	t.Cleanup(func() { _ = g.Stop(ctx) })

	require.NoError(t, clk.BlockUntilContext(ctx, 1))
	clk.Advance(rate) // the one permitted send
	require.Eventually(t, func() bool { return len(c.got()) == 1 }, 5*time.Second, time.Millisecond)
	require.NoError(t, clk.BlockUntilContext(ctx, 1))
	clk.Advance(rate) // the next tick finds the count spent and parks
	require.Eventually(t, func() bool { return logs.FilterMessage("Flow worker at the count limit, waiting").Len() == 1 }, 5*time.Second, time.Millisecond)

	tr.Reset()
	require.NoError(t, clk.BlockUntilContext(ctx, 1))
	clk.Advance(rate)
	require.Eventually(t, func() bool { return len(c.got()) == 2 }, time.Second, time.Millisecond,
		"the first send after a resume waits rate, not 1.5x rate")
}

// TestFailedWritesNeverRetryFasterThanTheRate pins that the backoff after a
// failed write never drops below the configured rate, even past 5 s.
func TestFailedWritesNeverRetryFasterThanTheRate(t *testing.T) {
	bo := newWriteBackoff(30 * time.Second)
	for range 4 {
		require.GreaterOrEqual(t, bo.NextBackOff(), 30*time.Second)
	}
}

// TestStopTwiceIsSafe pins that a second Stop returns instead of panicking on
// the already-closed stop channel.
func TestStopTwiceIsSafe(t *testing.T) {
	g, err := New(zap.NewNop(), 1, time.Millisecond, ScenarioDefault, 1, &captureConsumer{}, embed.NopTelemetry())
	require.NoError(t, err)
	require.NoError(t, g.Start(context.Background()))
	require.NoError(t, g.Stop(context.Background()))
	require.NotPanics(t, func() { require.NoError(t, g.Stop(context.Background())) })
}

// TestNewReportsMetricsError pins that New fails when the meter cannot build
// the generator metrics.
func TestNewReportsMetricsError(t *testing.T) {
	_, err := New(zap.NewNop(), 1, time.Millisecond, ScenarioDefault, 1, &captureConsumer{}, embed.TelemetrySettings{MeterProvider: failingMeterProvider{}})
	require.ErrorContains(t, err, "build generator metrics")
}

// failingMeterProvider's meter fails the first instrument the generator
// metrics build.
type failingMeterProvider struct{ metric.MeterProvider }

func (failingMeterProvider) Meter(string, ...metric.MeterOption) metric.Meter {
	return failingMeter{Meter: metricnoop.NewMeterProvider().Meter("test")}
}

type failingMeter struct{ metric.Meter }

func (failingMeter) Int64Gauge(string, ...metric.Int64GaugeOption) (metric.Int64Gauge, error) {
	return nil, errors.New("instrument error")
}
