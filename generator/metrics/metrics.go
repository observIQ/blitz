package metrics

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/observiq/blitz/output"
	"github.com/observiq/blitz/telemetry"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"
)

const componentName = "generator_metrics"

// MetricDefinition describes a single metric to generate.
type MetricDefinition struct {
	Name        string
	Type        output.MetricType
	Description string
	Unit        string
	Attributes  map[string]string
	ValueMin    float64
	ValueMax    float64
}

// Generator generates synthetic metric data points.
type Generator struct {
	logger  *zap.Logger
	workers int
	rate    time.Duration

	// resourceCombos is the pre-computed cartesian product of resource
	// attribute values. Each entry is a flat map for a single resource.
	resourceCombos []map[string]string
	metrics        []MetricDefinition
	rng            *rand.Rand

	wg     sync.WaitGroup
	stopCh chan struct{}
	meter  metric.Meter

	metricsGenerated metric.Int64Counter
	activeWorkers    metric.Int64Gauge
	writeErrors      metric.Int64Counter
}

// New creates a new metrics generator.
func New(
	logger *zap.Logger,
	workers int,
	rate time.Duration,
	resourceAttrs map[string][]string,
	metricDefs []MetricDefinition,
) (*Generator, error) {
	if logger == nil {
		return nil, fmt.Errorf("logger cannot be nil")
	}
	if workers < 1 {
		return nil, fmt.Errorf("workers must be 1 or greater, got %d", workers)
	}
	if len(metricDefs) == 0 {
		return nil, fmt.Errorf("at least one metric definition is required")
	}

	meter := otel.Meter("blitz-generator")

	metricsGenerated, err := meter.Int64Counter(
		"blitz.generator.metrics.generated",
		metric.WithDescription("Total number of metric data points generated"),
	)
	if err != nil {
		return nil, fmt.Errorf("create metrics generated counter: %w", err)
	}

	activeWorkers, err := meter.Int64Gauge(
		"blitz.generator.workers.active",
		metric.WithDescription("Number of active worker goroutines"),
	)
	if err != nil {
		return nil, fmt.Errorf("create active workers gauge: %w", err)
	}

	writeErrors, err := meter.Int64Counter(
		"blitz.generator.write.errors",
		metric.WithDescription("Total number of write errors"),
	)
	if err != nil {
		return nil, fmt.Errorf("create write errors counter: %w", err)
	}

	resourceCombos := cartesianProduct(resourceAttrs)

	return &Generator{
		logger:           logger.Named("generator-metrics"),
		workers:          workers,
		rate:             rate,
		resourceCombos:   resourceCombos,
		metrics:          metricDefs,
		rng:              rand.New(rand.NewSource(time.Now().UnixNano())),
		stopCh:           make(chan struct{}),
		meter:            meter,
		metricsGenerated: metricsGenerated,
		activeWorkers:    activeWorkers,
		writeErrors:      writeErrors,
	}, nil
}

// SupportedTelemetry returns the telemetry types this generator supports.
func (g *Generator) SupportedTelemetry() []telemetry.Type {
	return []telemetry.Type{telemetry.Metrics}
}

// Start starts the metrics generator.
func (g *Generator) Start(writer output.Writer) error {
	g.logger.Info("Starting metrics generator",
		zap.Int("workers", g.workers),
		zap.Duration("rate", g.rate),
		zap.Int("metric_definitions", len(g.metrics)),
		zap.Int("resource_combos", len(g.resourceCombos)),
	)

	g.activeWorkers.Record(context.Background(), int64(g.workers),
		metric.WithAttributeSet(attribute.NewSet(attribute.String("component", componentName))),
	)

	for i := 0; i < g.workers; i++ {
		g.wg.Add(1)
		go g.worker(i, writer)
	}
	return nil
}

// Stop stops the generator.
func (g *Generator) Stop(ctx context.Context) error {
	g.logger.Info("Stopping metrics generator")

	g.activeWorkers.Record(ctx, 0,
		metric.WithAttributeSet(attribute.NewSet(attribute.String("component", componentName))),
	)

	close(g.stopCh)

	done := make(chan struct{})
	go func() {
		g.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		g.logger.Info("All workers stopped gracefully")
		return nil
	case <-ctx.Done():
		return fmt.Errorf("stop cancelled due to context cancellation: %w", ctx.Err())
	}
}

func (g *Generator) worker(workerID int, writer output.Writer) {
	defer g.wg.Done()
	g.logger.Debug("Starting worker", zap.Int("worker_id", workerID))

	backoffConfig := backoff.NewExponentialBackOff()
	backoffConfig.InitialInterval = g.rate
	backoffConfig.MaxInterval = 5 * time.Second
	backoffConfig.MaxElapsedTime = 0

	backoffTicker := backoff.NewTicker(backoffConfig)
	defer backoffTicker.Stop()

	for {
		select {
		case <-g.stopCh:
			g.logger.Debug("Worker stopping", zap.Int("worker_id", workerID))
			return
		case <-backoffTicker.C:
			if err := g.generateAndWrite(writer); err != nil {
				g.logger.Error("Failed to write metric", zap.Int("worker_id", workerID), zap.Error(err))
				continue
			}
			backoffConfig.Reset()
		}
	}
}

// generateAndWrite emits one data point for every combination of
// resource attributes × metric definition.
func (g *Generator) generateAndWrite(writer output.Writer) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	now := time.Now()

	for _, resAttrs := range g.resourceCombos {
		for i := range g.metrics {
			def := &g.metrics[i]

			record := output.MetricRecord{
				Name:               def.Name,
				Description:        def.Description,
				Unit:               def.Unit,
				Type:               def.Type,
				Attributes:         def.Attributes,
				ResourceAttributes: resAttrs,
				Timestamp:          now,
			}

			if def.Type == output.MetricTypeHistogram {
				g.populateHistogram(&record, def)
			} else {
				value := def.ValueMin + g.rng.Float64()*(def.ValueMax-def.ValueMin)
				record.DoubleValue = &value
			}

			if err := writer.WriteMetric(ctx, record); err != nil {
				errorType := "unknown"
				if ctx.Err() == context.DeadlineExceeded {
					errorType = "timeout"
				}
				g.recordWriteError(errorType)
				return err
			}

			g.metricsGenerated.Add(context.Background(), 1,
				metric.WithAttributeSet(attribute.NewSet(attribute.String("component", componentName))),
			)
		}
	}

	return nil
}

func (g *Generator) recordWriteError(errorType string) {
	g.writeErrors.Add(context.Background(), 1,
		metric.WithAttributeSet(attribute.NewSet(
			attribute.String("component", componentName),
			attribute.String("error_type", errorType),
		)),
	)
}

// populateHistogram fills a MetricRecord with synthetic histogram data.
// It generates bucket boundaries evenly spaced between ValueMin and ValueMax,
// random counts per bucket, and computes sum/min/max from the distribution.
func (g *Generator) populateHistogram(record *output.MetricRecord, def *MetricDefinition) {
	const numBuckets = 5
	span := def.ValueMax - def.ValueMin
	step := span / float64(numBuckets)

	bounds := make([]float64, numBuckets)
	for i := range bounds {
		bounds[i] = def.ValueMin + step*float64(i+1)
	}

	// Generate random counts for each bucket (numBuckets + 1 including overflow).
	bucketCounts := make([]uint64, numBuckets+1)
	var totalCount uint64
	var sum float64
	minVal := def.ValueMax
	maxVal := def.ValueMin

	for i := range bucketCounts {
		c := uint64(g.rng.Intn(20) + 1)
		bucketCounts[i] = c
		totalCount += c

		// Estimate a representative value for this bucket to compute sum/min/max.
		var representative float64
		switch {
		case i == 0:
			representative = def.ValueMin + step*0.5
		case i == numBuckets:
			representative = bounds[numBuckets-1] + step*0.5
		default:
			representative = bounds[i-1] + step*0.5
		}

		sum += representative * float64(c)
		if representative < minVal {
			minVal = representative
		}
		if representative > maxVal {
			maxVal = representative
		}
	}

	record.HistogramBucketBounds = bounds
	record.HistogramBucketCounts = bucketCounts
	record.HistogramCount = &totalCount
	record.HistogramSum = &sum
	record.HistogramMin = &minVal
	record.HistogramMax = &maxVal
}

// cartesianProduct computes the cartesian product of a map of keys to
// value lists. Each returned map has exactly one value per key. If the
// input is nil or empty, a single empty map is returned so callers
// always iterate at least once.
func cartesianProduct(attrs map[string][]string) []map[string]string {
	if len(attrs) == 0 {
		return []map[string]string{{}}
	}

	keys := make([]string, 0, len(attrs))
	for k := range attrs {
		keys = append(keys, k)
	}

	results := []map[string]string{{}}
	for _, key := range keys {
		vals := attrs[key]
		var next []map[string]string
		for _, existing := range results {
			for _, v := range vals {
				combo := make(map[string]string, len(existing)+1)
				for ek, ev := range existing {
					combo[ek] = ev
				}
				combo[key] = v
				next = append(next, combo)
			}
		}
		results = next
	}
	return results
}
