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
	"github.com/observiq/blitz/generator/resource"
	"github.com/observiq/blitz/internal/datagen"
	"github.com/observiq/blitz/telemetry"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"
)

const (
	generatorType = "traces"

	// stopDrainTimeout caps how long Stop waits for already-scheduled
	// span-emission timers to fire and drain to the consumer when the
	// caller's ctx has no earlier deadline. Sized well above the longest
	// possible scheduled span (root span <= 510ms + db <= 150ms + proc
	// <= 205ms + per-child variants) plus headroom for a backed-up
	// consumer.
	stopDrainTimeout = 20 * time.Second
)

// Config configures the traces generator.
type Config struct {
	// Logger is the zap logger used for diagnostic output. Required.
	Logger *zap.Logger
	// Workers is the number of worker goroutines (each emits one trace
	// per Rate). Required, >= 1.
	Workers int
	// Rate is the per-worker trace-start interval. Required, > 0.
	Rate time.Duration
	// Hostname is the simulated hostname this generator's traces
	// describe (populates Span.Metadata.Resource["host.name"]). If empty,
	// a deterministic Linux-style hostname is generated from Seed via
	// datagen.GenerateHostname, matching the hostmetrics convention so
	// records from both signals attribute consistently to the same
	// simulated machine when configured with the same Seed.
	Hostname string
	// Consumer receives each emitted span individually as it "completes"
	// (its EndTime is reached on wall-clock). Required.
	Consumer embed.TraceConsumer
	// Seed controls per-worker RNG seeding for span content.
	// Negative → randomize (worker N gets time.Now().UnixNano()+N).
	// 0 or positive → deterministic (worker N gets seed Seed+N), so
	// trace structure — span Name, Kind, StatusCode, Attributes,
	// child count — is reproducible across runs.
	//
	// Programmatic Go callers see the literal value here. The YAML
	// path additionally translates `seed: 0` (or omitted) into
	// randomize at the dispatch layer so YAML users get stochastic
	// data by default.
	//
	// TraceID and SpanID are NOT governed by Seed: they come from
	// crypto/rand and are always globally unique. This preserves
	// uniqueness across blitz instances that share a Seed but
	// participate in the same downstream pipeline.
	Seed int64
}

// Generator implements embed.ProducerModule for synthetic distributed
// traces.
//
// Each generated trace consists of 2–5 spans: a root HTTP server span
// plus a randomly-chosen subset of child spans (database query, cache
// lookup, internal processing, downstream HTTP client call, input
// validation). The child count and child kinds are seeded by Config.Seed.
//
// Each span is emitted **individually** at its EndTime via time.AfterFunc,
// so the emission timeline mimics a real distributed system where spans
// complete at different wall-clock moments — not a synthetic batch that
// arrives all at once. This is a load-bearing design choice for
// downstream distributed-blitz simulation where a single trace may span
// multiple blitz instances.
type Generator struct {
	embed.ProducerMarker

	logger   *zap.Logger
	workers  int
	rate     time.Duration
	hostname string
	consumer embed.TraceConsumer
	seed     int64

	wg      sync.WaitGroup
	stopCh  chan struct{}
	tracker *count.Tracker
}

// Compile-time assertion that *Generator implements embed.ProducerModule.
var _ embed.ProducerModule = (*Generator)(nil)

// New creates a new traces generator. The supplied embed.TraceConsumer
// receives spans individually as each span's EndTime is reached on
// wall-clock — see Generator's doc for the rationale.
func New(cfg Config) (*Generator, error) {
	if cfg.Logger == nil {
		return nil, fmt.Errorf("logger cannot be nil")
	}
	if cfg.Consumer == nil {
		return nil, fmt.Errorf("TraceConsumer cannot be nil")
	}
	if cfg.Workers < 1 {
		return nil, fmt.Errorf("workers must be 1 or greater, got %d", cfg.Workers)
	}
	if cfg.Rate <= 0 {
		return nil, fmt.Errorf("rate must be greater than 0, got %s", cfg.Rate)
	}

	hostname := cfg.Hostname
	if hostname == "" {
		// Hostname-only RNG; seeded once at construction since the
		// simulated host is fixed for the lifetime of the generator.
		seed := cfg.Seed
		if seed < 0 {
			seed = time.Now().UnixNano()
		}
		hostname = datagen.GenerateHostname(
			mathrand.New(mathrand.NewSource(seed)), // #nosec G404 -- seeded for determinism contract
			datagen.StyleLinux,
			datagen.AllMythologyNames,
		)
	}

	return &Generator{
		logger:   cfg.Logger.Named("generator-traces"),
		workers:  cfg.Workers,
		rate:     cfg.Rate,
		hostname: hostname,
		consumer: cfg.Consumer,
		seed:     cfg.Seed,
		stopCh:   make(chan struct{}),
	}, nil
}

// Name returns the module identifier for ProducerModule.
func (g *Generator) Name() string { return generatorType }

// SetCountTracker sets the count tracker for finite generation.
//
// **Counting semantics:** one Acquire equals one *trace*, not one
// span. A trace emits 2–5 spans, all of which flow to the consumer
// even after Acquire returns false on subsequent ticks. Hosts that
// want a span-count budget should multiply their intended cap by the
// max span count.
func (g *Generator) SetCountTracker(tracker *count.Tracker) {
	g.tracker = tracker
}

// Start launches the worker goroutines.
func (g *Generator) Start(_ context.Context) error {
	g.logger.Info("Starting traces generator",
		zap.Int("workers", g.workers),
		zap.Duration("rate", g.rate),
		zap.String("hostname", g.hostname),
	)

	generator.BlitzGeneratorActiveWorkersGauge.Record(context.Background(), int64(g.workers), generatorType)

	for i := range g.workers {
		g.wg.Add(1)
		go g.worker(i)
	}

	return nil
}

// Stop signals workers to exit and attempts to drain any pending
// span-emission timers — every span scheduled before Stop is delivered
// to the consumer at its EndTime as if Stop had not been called. This
// keeps trace shapes coherent for the consumer (no half-emitted traces)
// and matches the distributed-blitz contract that spans are not silently
// dropped at shutdown.
//
// Stop returns when either (a) all pending timers have fired and their
// emissions completed, or (b) the bounding context (ctx.Done() or
// stopDrainTimeout, whichever fires first) elapses. On (b), Stop returns
// the bounding context's error — but any time.AfterFunc callbacks still
// pending at that moment will continue to fire on their original
// schedule. Their emissions complete asynchronously after Stop has
// returned; the Generator object stays reachable from the runtime until
// the last pending callback runs, at which point it becomes
// garbage-collectible. This is not a leak — every callback eventually
// resolves and the wg drains — but callers should be aware that "Stop
// returned a timeout error" does not mean "no further emissions will
// reach the consumer".
//
// If the strict "no emissions after Stop returns" semantic is required,
// the caller can wrap the supplied consumer in a guard that drops
// post-Stop ConsumeTraces calls.
func (g *Generator) Stop(ctx context.Context) error {
	g.logger.Info("Stopping traces generator")

	close(g.stopCh)

	generator.BlitzGeneratorActiveWorkersGauge.Record(ctx, 0, generatorType)

	drainCtx, cancel := context.WithTimeout(ctx, stopDrainTimeout)
	defer cancel()

	done := make(chan struct{})
	go func() {
		g.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		g.logger.Info("Traces generator stopped")
		return nil
	case <-drainCtx.Done():
		g.logger.Warn("Traces generator stop drain bounded",
			zap.Duration("bound", stopDrainTimeout),
			zap.Error(drainCtx.Err()),
		)
		return drainCtx.Err()
	}
}

// SupportedTelemetry returns the telemetry types this generator produces.
func (g *Generator) SupportedTelemetry() []telemetry.Type {
	return []telemetry.Type{telemetry.Traces}
}

func (g *Generator) worker(id int) {
	defer g.wg.Done()

	seed := g.seed
	if seed < 0 {
		seed = time.Now().UnixNano() + int64(id)
	} else {
		seed += int64(id)
	}
	r := mathrand.New(mathrand.NewSource(seed)) // #nosec G404 -- seeded for determinism contract

	ticker := time.NewTicker(g.rate)
	defer ticker.Stop()

	for {
		select {
		case <-g.stopCh:
			return
		case <-ticker.C:
			g.startTrace(r)
		}
	}
}

// childKind enumerates the kinds of child spans a trace may contain.
// startTrace picks 1–4 of these per trace to produce a 2–5 span shape.
type childKind int

const (
	childDBQuery childKind = iota
	childCacheLookup
	childInternalProcessing
	childDownstreamHTTP
	childInputValidation
)

var allChildKinds = []childKind{
	childDBQuery,
	childCacheLookup,
	childInternalProcessing,
	childDownstreamHTTP,
	childInputValidation,
}

// startTrace generates the spans for one trace and schedules each span
// to be emitted individually at its EndTime via time.AfterFunc. The
// trace's spans land in the consumer at different wall-clock moments,
// matching how spans arrive from a real distributed system.
func (g *Generator) startTrace(r *mathrand.Rand) {
	// Count tracker semantics: 1 trace = 1 acquire (see SetCountTracker).
	if g.tracker != nil {
		if !g.tracker.Acquire() {
			return
		}
	}

	res := resource.Default(generatorType)
	res["host.name"] = g.hostname

	traceID := generateTraceID()
	now := time.Now()

	// Root span: HTTP server
	rootSpanID := generateSpanID()
	method := httpMethods[r.Intn(len(httpMethods))]              // #nosec G404
	path := httpPaths[r.Intn(len(httpPaths))]                    // #nosec G404
	statusCode := httpStatusCodes[r.Intn(len(httpStatusCodes))]  // #nosec G404
	duration := time.Duration(r.Intn(500)+10) * time.Millisecond // #nosec G404

	rootSpan := embed.Span{
		TraceID:    traceID,
		SpanID:     rootSpanID,
		Name:       fmt.Sprintf("%s %s", method, path),
		Kind:       embed.SpanKindServer,
		StartTime:  now,
		EndTime:    now.Add(duration),
		StatusCode: statusCodeFromHTTP(statusCode),
		Metadata: embed.SpanMetadata{
			Resource: cloneResource(res),
			Attributes: map[string]any{
				"http.method":      method,
				"http.target":      path,
				"http.status_code": statusCode,
				"http.scheme":      "https",
				"net.host.name":    g.hostname,
				"net.host.port":    443,
			},
		},
	}
	g.scheduleEmission(rootSpan)

	// Pick 1-4 child kinds (random subset, no duplicates) → total 2-5 spans.
	childCount := r.Intn(4) + 1 // #nosec G404
	shuffled := make([]childKind, len(allChildKinds))
	copy(shuffled, allChildKinds)
	r.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })

	cursorStart := now
	for i := range childCount {
		child := g.buildChildSpan(r, shuffled[i], traceID, rootSpanID, cursorStart, res)
		g.scheduleEmission(child)
		// Stagger child start times within the root's window so spans
		// don't all start at `now` — keeps the shape realistic.
		cursorStart = cursorStart.Add(time.Duration(r.Intn(40)) * time.Millisecond) // #nosec G404
	}
}

func (g *Generator) buildChildSpan(r *mathrand.Rand, kind childKind, traceID, parentID string, earliestStart time.Time, res map[string]any) embed.Span {
	spanID := generateSpanID()
	startOffset := time.Duration(r.Intn(50)) * time.Millisecond // #nosec G404
	start := earliestStart.Add(startOffset)

	sp := embed.Span{
		TraceID:      traceID,
		SpanID:       spanID,
		ParentSpanID: parentID,
		StartTime:    start,
		StatusCode:   0,
		Metadata: embed.SpanMetadata{
			Resource:   cloneResource(res),
			Attributes: map[string]any{},
		},
	}

	switch kind {
	case childDBQuery:
		op := dbOperations[r.Intn(len(dbOperations))]          // #nosec G404
		dur := time.Duration(r.Intn(100)+1) * time.Millisecond // #nosec G404
		sp.Name = op
		sp.Kind = embed.SpanKindClient
		sp.EndTime = start.Add(dur)
		sp.Metadata.Attributes["db.system"] = "postgresql"
		sp.Metadata.Attributes["db.statement"] = op
		sp.Metadata.Attributes["db.name"] = "production"
		sp.Metadata.Attributes["net.peer.name"] = "db.internal"
		sp.Metadata.Attributes["net.peer.port"] = 5432
	case childCacheLookup:
		op := cacheOperations[r.Intn(len(cacheOperations))]   // #nosec G404
		dur := time.Duration(r.Intn(20)+1) * time.Millisecond // #nosec G404
		sp.Name = op
		sp.Kind = embed.SpanKindClient
		sp.EndTime = start.Add(dur)
		sp.Metadata.Attributes["db.system"] = "redis"
		sp.Metadata.Attributes["db.operation"] = op
		sp.Metadata.Attributes["net.peer.name"] = "cache.internal"
		sp.Metadata.Attributes["net.peer.port"] = 6379
	case childInternalProcessing:
		stage := processingStages[r.Intn(len(processingStages))] // #nosec G404
		dur := time.Duration(r.Intn(200)+5) * time.Millisecond   // #nosec G404
		sp.Name = "process_request"
		sp.Kind = embed.SpanKindInternal
		sp.EndTime = start.Add(dur)
		sp.Metadata.Attributes["processing.stage"] = stage
	case childDownstreamHTTP:
		method := httpMethods[r.Intn(len(httpMethods))]        // #nosec G404
		host := downstreamHosts[r.Intn(len(downstreamHosts))]  // #nosec G404
		path := httpPaths[r.Intn(len(httpPaths))]              // #nosec G404
		dur := time.Duration(r.Intn(300)+5) * time.Millisecond // #nosec G404
		sp.Name = fmt.Sprintf("%s %s", method, path)
		sp.Kind = embed.SpanKindClient
		sp.EndTime = start.Add(dur)
		sp.Metadata.Attributes["http.method"] = method
		sp.Metadata.Attributes["http.url"] = fmt.Sprintf("https://%s%s", host, path)
		sp.Metadata.Attributes["net.peer.name"] = host
	case childInputValidation:
		dur := time.Duration(r.Intn(10)+1) * time.Millisecond // #nosec G404
		sp.Name = "validate_input"
		sp.Kind = embed.SpanKindInternal
		sp.EndTime = start.Add(dur)
		sp.Metadata.Attributes["validation.scheme"] = "json-schema"
	}

	return sp
}

// scheduleEmission arranges for `sp` to be delivered to the consumer at
// `sp.EndTime` on wall-clock. If EndTime is already in the past (rare,
// e.g. a span with zero duration), the emission happens immediately on
// a fresh goroutine to keep the caller non-blocking.
//
// Each pending emission is tracked in g.wg so Stop drains all in-flight
// timers within the bound. Stop does NOT cancel pending emissions: the
// emission fires whether or not stopCh is closed, because dropping spans
// silently at shutdown produces incoherent traces downstream.
func (g *Generator) scheduleEmission(sp embed.Span) {
	g.wg.Add(1)
	delay := time.Until(sp.EndTime)
	if delay <= 0 {
		go func() {
			defer g.wg.Done()
			g.emitSpan(sp)
		}()
		return
	}
	time.AfterFunc(delay, func() {
		defer g.wg.Done()
		g.emitSpan(sp)
	})
}

func (g *Generator) emitSpan(sp embed.Span) {
	ctx := context.Background()
	if err := g.consumer.ConsumeTraces(ctx, []embed.Span{sp}); err != nil {
		g.logger.Debug("Consume trace error", zap.Error(err))
		generator.BlitzGeneratorWriteErrorsCounter.Add(ctx, 1, generatorType,
			metric.WithAttributeSet(attribute.NewSet(attribute.String("span_kind", string(sp.Kind)))),
		)
		return
	}
	generator.BlitzGeneratorEntriesCounter.Add(ctx, 1, generatorType)
}

// cloneResource returns a defensive copy so per-span mutations (e.g. a
// future host-base merge in the runner) can't bleed across spans.
func cloneResource(src map[string]any) map[string]any {
	out := make(map[string]any, len(src))
	for k, v := range src {
		out[k] = v
	}
	return out
}

var (
	httpMethods      = []string{"GET", "POST", "PUT", "DELETE", "PATCH"}
	httpPaths        = []string{"/api/users", "/api/orders", "/api/products", "/api/health", "/api/auth/login", "/api/search"}
	httpStatusCodes  = []int{200, 200, 200, 201, 204, 301, 400, 401, 403, 404, 500}
	dbOperations     = []string{"SELECT users", "INSERT orders", "UPDATE products", "DELETE sessions", "SELECT COUNT(*) FROM events"}
	cacheOperations  = []string{"GET session", "SET session", "DEL session", "GET feature_flag"}
	processingStages = []string{"validation", "authorization", "serialization", "transform"}
	downstreamHosts  = []string{"auth.example.com", "inventory.example.com", "billing.example.com"}
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
