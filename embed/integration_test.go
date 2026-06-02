package embed_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/observiq/blitz/embed"
	"github.com/observiq/blitz/generator/apache"
	"github.com/observiq/blitz/generator/hostmetrics"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// memoryLogConsumer captures every record pushed through ConsumeLogs.
// Used by embed integration tests to assert end-to-end record flow from
// a ProducerModule through the Runner into a host-supplied consumer.
type memoryLogConsumer struct {
	mu      sync.Mutex
	records []embed.LogRecord
}

func (c *memoryLogConsumer) ConsumeLogs(_ context.Context, records []embed.LogRecord) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.records = append(c.records, records...)
	return nil
}

func (c *memoryLogConsumer) snapshot() []embed.LogRecord {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]embed.LogRecord, len(c.records))
	copy(out, c.records)
	return out
}

// TestEmbed_ApacheRecordsFlowToMemoryConsumer is the end-to-end smoke
// test for the embed seam: a ProducerModule constructed against a host
// consumer, wrapped in embed.New, started via the Runner, produces
// records that the host can observe in-process.
func TestEmbed_ApacheRecordsFlowToMemoryConsumer(t *testing.T) {
	logger := zaptest.NewLogger(t)
	consumer := &memoryLogConsumer{}

	gen, err := apache.New(logger, 1, 10*time.Millisecond, consumer)
	require.NoError(t, err)

	runner, err := embed.New(embed.Config{
		Modules: []embed.ProducerModule{gen},
	})
	require.NoError(t, err)

	host := embed.Host{
		Logs:   consumer,
		Logger: logger,
	}
	require.NoError(t, runner.Start(context.Background(), host))

	// Let the generator emit a few records.
	require.Eventually(t,
		func() bool { return len(consumer.snapshot()) >= 3 },
		2*time.Second, 20*time.Millisecond,
		"expected at least 3 records to flow through the embed seam",
	)

	stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, runner.Stop(stopCtx))

	records := consumer.snapshot()
	assert.GreaterOrEqual(t, len(records), 3)
	for _, rec := range records {
		assert.NotEmpty(t, rec.Message, "expected non-empty Message on captured record")
	}
}

func TestEmbed_NewRejectsEmptyModules(t *testing.T) {
	_, err := embed.New(embed.Config{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Modules cannot be empty")
}

// memoryMetricConsumer captures every metric-point batch pushed through
// ConsumeMetrics.
type memoryMetricConsumer struct {
	mu     sync.Mutex
	points []embed.MetricPoint
}

func (c *memoryMetricConsumer) ConsumeMetrics(_ context.Context, batch []embed.MetricPoint) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.points = append(c.points, batch...)
	return nil
}

func (c *memoryMetricConsumer) snapshot() []embed.MetricPoint {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]embed.MetricPoint, len(c.points))
	copy(out, c.points)
	return out
}

// TestEmbed_HostMetricsPointsFlowToMemoryConsumer is the parallel
// metrics-path smoke test for PIPE-1023: a hostmetrics generator
// constructed against an embed.MetricConsumer, wrapped in embed.New,
// started via the Runner, produces points the host observes in-process.
func TestEmbed_HostMetricsPointsFlowToMemoryConsumer(t *testing.T) {
	logger := zaptest.NewLogger(t)
	consumer := &memoryMetricConsumer{}

	gen, err := hostmetrics.New(hostmetrics.Config{
		Logger:       logger,
		Workers:      1,
		Rate:         50 * time.Millisecond,
		OS:           "linux",
		Hostname:     "test-host",
		ScraperNames: []string{"cpu", "memory"},
		Consumer:     consumer,
	})
	require.NoError(t, err)

	runner, err := embed.New(embed.Config{
		Modules: []embed.ProducerModule{gen},
	})
	require.NoError(t, err)

	host := embed.Host{
		Metrics: consumer,
		Logger:  logger,
	}
	require.NoError(t, runner.Start(context.Background(), host))

	require.Eventually(t,
		func() bool { return len(consumer.snapshot()) >= 3 },
		2*time.Second, 20*time.Millisecond,
		"expected at least 3 metric points to flow through the embed seam",
	)

	stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, runner.Stop(stopCtx))

	points := consumer.snapshot()
	assert.GreaterOrEqual(t, len(points), 3)
	for _, p := range points {
		assert.NotEmpty(t, p.Name, "expected non-empty Name on metric point")
		assert.NotZero(t, p.Metadata.Timestamp, "expected non-zero Timestamp")
	}
}

func TestEmbed_RunnerRejectsDoubleStart(t *testing.T) {
	logger := zaptest.NewLogger(t)
	consumer := &memoryLogConsumer{}
	gen, err := apache.New(logger, 1, 10*time.Millisecond, consumer)
	require.NoError(t, err)

	runner, err := embed.New(embed.Config{Modules: []embed.ProducerModule{gen}})
	require.NoError(t, err)

	host := embed.Host{Logs: consumer, Logger: logger}
	require.NoError(t, runner.Start(context.Background(), host))
	defer func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = runner.Stop(stopCtx)
	}()

	err = runner.Start(context.Background(), host)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already started")
}
