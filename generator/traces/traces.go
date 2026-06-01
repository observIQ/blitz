package traces

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	mathrand "math/rand"
	"sync"
	"time"

	"github.com/observiq/blitz/embed"
	"github.com/observiq/blitz/generator"
	"github.com/observiq/blitz/generator/count"
	"github.com/observiq/blitz/output"
	"github.com/observiq/blitz/telemetry"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"
)

const generatorType = "traces"

// Generator implements generator.TraceGenerator for synthetic distributed traces.
type Generator struct {
	embed.ProducerMarker

	logger  *zap.Logger
	workers int
	rate    time.Duration
	wg      sync.WaitGroup
	stopCh  chan struct{}
	tracker *count.Tracker
}

// Compile-time assertion that Generator implements TraceGenerator.
var _ generator.TraceGenerator = (*Generator)(nil)

// New creates a new traces generator.
func New(logger *zap.Logger, workers int, rate time.Duration) (*Generator, error) {
	if logger == nil {
		return nil, fmt.Errorf("logger cannot be nil")
	}
	if workers < 1 {
		return nil, fmt.Errorf("workers must be 1 or greater, got %d", workers)
	}

	return &Generator{
		logger:  logger.Named("generator-traces"),
		workers: workers,
		rate:    rate,
		stopCh:  make(chan struct{}),
	}, nil
}

// SetCountTracker sets the count tracker for finite generation.
func (g *Generator) SetCountTracker(tracker *count.Tracker) {
	g.tracker = tracker
}

// Start starts the traces generator.
func (g *Generator) Start(writer output.TraceWriter) error {
	g.logger.Info("Starting traces generator",
		zap.Int("workers", g.workers),
		zap.Duration("rate", g.rate),
	)

	generator.BlitzGeneratorActiveWorkersGauge.Record(context.Background(), int64(g.workers), generatorType)

	for i := range g.workers {
		g.wg.Add(1)
		go g.worker(i, writer)
	}

	return nil
}

// Stop stops the traces generator.
func (g *Generator) Stop(ctx context.Context) error {
	g.logger.Info("Stopping traces generator")

	close(g.stopCh)

	generator.BlitzGeneratorActiveWorkersGauge.Record(ctx, 0, generatorType)

	done := make(chan struct{})
	go func() {
		g.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		g.logger.Info("Traces generator stopped")
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// SupportedTelemetry returns the telemetry types this generator produces.
func (g *Generator) SupportedTelemetry() []telemetry.Type {
	return []telemetry.Type{telemetry.Traces}
}

func (g *Generator) worker(id int, writer output.TraceWriter) {
	defer g.wg.Done()

	r := mathrand.New(mathrand.NewSource(time.Now().UnixNano() + int64(id))) // #nosec G404

	ticker := time.NewTicker(g.rate)
	defer ticker.Stop()

	for {
		select {
		case <-g.stopCh:
			return
		case <-ticker.C:
			g.generateTrace(r, writer)
		}
	}
}

func (g *Generator) generateTrace(r *mathrand.Rand, writer output.TraceWriter) {
	// Check count tracker (1 trace = 1 acquire)
	if g.tracker != nil {
		if !g.tracker.Acquire() {
			return
		}
	}

	ctx := context.Background()
	traceID := generateTraceID()
	now := time.Now()

	// Generate root span: HTTP server
	rootSpanID := generateSpanID()
	method := httpMethods[r.Intn(len(httpMethods))]              // #nosec G404
	path := httpPaths[r.Intn(len(httpPaths))]                    // #nosec G404
	statusCode := httpStatusCodes[r.Intn(len(httpStatusCodes))]  // #nosec G404
	duration := time.Duration(r.Intn(500)+10) * time.Millisecond // #nosec G404

	rootSpan := output.TraceRecord{
		TraceID:    traceID,
		SpanID:     rootSpanID,
		Name:       fmt.Sprintf("%s %s", method, path),
		Kind:       output.SpanKindServer,
		StartTime:  now,
		EndTime:    now.Add(duration),
		StatusCode: statusCodeFromHTTP(statusCode),
		Metadata: output.SpanMetadata{
			Attributes: map[string]any{
				"http.method":      method,
				"http.target":      path,
				"http.status_code": statusCode,
				"http.scheme":      "https",
				"net.host.name":    "api.example.com",
				"net.host.port":    443,
			},
		},
	}

	if err := writer.WriteTrace(ctx, rootSpan); err != nil {
		g.logger.Debug("Write trace error", zap.Error(err))
		generator.BlitzGeneratorWriteErrorsCounter.Add(ctx, 1, generatorType,
			metric.WithAttributeSet(attribute.NewSet(attribute.String("span_kind", "server"))),
		)
	} else {
		generator.BlitzGeneratorEntriesCounter.Add(ctx, 1, generatorType)
	}

	// Generate child span: DB query
	dbSpanID := generateSpanID()
	dbOp := dbOperations[r.Intn(len(dbOperations))]                  // #nosec G404
	dbDuration := time.Duration(r.Intn(100)+1) * time.Millisecond    // #nosec G404
	dbStart := now.Add(time.Duration(r.Intn(50)) * time.Millisecond) // #nosec G404

	dbSpan := output.TraceRecord{
		TraceID:      traceID,
		SpanID:       dbSpanID,
		ParentSpanID: rootSpanID,
		Name:         dbOp,
		Kind:         output.SpanKindClient,
		StartTime:    dbStart,
		EndTime:      dbStart.Add(dbDuration),
		StatusCode:   0,
		Metadata: output.SpanMetadata{
			Attributes: map[string]any{
				"db.system":     "postgresql",
				"db.statement":  dbOp,
				"db.name":       "production",
				"net.peer.name": "db.internal",
				"net.peer.port": 5432,
			},
		},
	}

	if err := writer.WriteTrace(ctx, dbSpan); err != nil {
		g.logger.Debug("Write trace error", zap.Error(err))
		generator.BlitzGeneratorWriteErrorsCounter.Add(ctx, 1, generatorType,
			metric.WithAttributeSet(attribute.NewSet(attribute.String("span_kind", "client"))),
		)
	} else {
		generator.BlitzGeneratorEntriesCounter.Add(ctx, 1, generatorType)
	}

	// Optionally generate a processing span (50% chance)
	if r.Float64() < 0.5 { // #nosec G404
		procSpanID := generateSpanID()
		procDuration := time.Duration(r.Intn(200)+5) * time.Millisecond // #nosec G404
		procStart := dbStart.Add(dbDuration)

		procSpan := output.TraceRecord{
			TraceID:      traceID,
			SpanID:       procSpanID,
			ParentSpanID: rootSpanID,
			Name:         "process_request",
			Kind:         output.SpanKindInternal,
			StartTime:    procStart,
			EndTime:      procStart.Add(procDuration),
			StatusCode:   0,
			Metadata: output.SpanMetadata{
				Attributes: map[string]any{
					"processing.stage": "validation",
				},
			},
		}

		if err := writer.WriteTrace(ctx, procSpan); err != nil {
			g.logger.Debug("Write trace error", zap.Error(err))
			generator.BlitzGeneratorWriteErrorsCounter.Add(ctx, 1, generatorType,
				metric.WithAttributeSet(attribute.NewSet(attribute.String("span_kind", "internal"))),
			)
		} else {
			generator.BlitzGeneratorEntriesCounter.Add(ctx, 1, generatorType)
		}
	}
}

var (
	httpMethods     = []string{"GET", "POST", "PUT", "DELETE", "PATCH"}
	httpPaths       = []string{"/api/users", "/api/orders", "/api/products", "/api/health", "/api/auth/login", "/api/search"}
	httpStatusCodes = []int{200, 200, 200, 201, 204, 301, 400, 401, 403, 404, 500}
	dbOperations    = []string{"SELECT users", "INSERT orders", "UPDATE products", "DELETE sessions", "SELECT COUNT(*) FROM events"}
)

func statusCodeFromHTTP(code int) int {
	if code >= 500 {
		return 2 // Error
	}
	return 0 // Unset
}

func generateTraceID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func generateSpanID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
