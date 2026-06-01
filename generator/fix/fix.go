// Package fix is the FIX (Financial Information eXchange) protocol
// generator. Emits FIX-formatted SOH-delimited messages at a
// configurable rate via the standard blitz embed.LogConsumer contract.
//
// Architecture:
//   - catalog: protocol versions, asset categories, SecurityTypes,
//     framing primitives, MessageDefinitions (one per Version × MsgType
//     × AssetCategory triple)
//   - per-category subpackages register V44 MessageDefinitions with
//     asset-specific Instrument component fields
//   - v42 / v50sp2 subpackages mirror V44 into their version-specific
//     entries with appropriate deltas
//   - state.Session tracks per-session sequence numbers, ExecID
//     counter, per-category open-order books (used to make
//     ExecutionReports and cancels reference real prior NewOrderSingles)
//   - this top-level package wires it all together: a Generator that
//     spawns workers, each running a deterministic emit loop seeded
//     from the user's SeedConfig and pushing records into an
//     embed.LogConsumer
//
// Determinism: same SeedConfig + same start time = byte-identical
// output stream. Verified by `TestGoldenOutputDeterministicFromSeed`.
package fix

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/observiq/blitz/embed"
	"github.com/observiq/blitz/generator/fix/catalog"
	"github.com/observiq/blitz/generator/fix/catalog/v44/app"
	"github.com/observiq/blitz/generator/fix/state"
	"github.com/observiq/blitz/generator/resource"

	// Bring in per-category and per-version registrations.
	_ "github.com/observiq/blitz/generator/fix/catalog/v42"
	_ "github.com/observiq/blitz/generator/fix/catalog/v44/corpbonds"
	_ "github.com/observiq/blitz/generator/fix/catalog/v44/equities"
	_ "github.com/observiq/blitz/generator/fix/catalog/v44/futures"
	_ "github.com/observiq/blitz/generator/fix/catalog/v44/fx"
	_ "github.com/observiq/blitz/generator/fix/catalog/v44/govbonds"
	_ "github.com/observiq/blitz/generator/fix/catalog/v44/moneymarket"
	_ "github.com/observiq/blitz/generator/fix/catalog/v44/options"
	_ "github.com/observiq/blitz/generator/fix/catalog/v44/otcderivs"
	_ "github.com/observiq/blitz/generator/fix/catalog/v44/repos"
	_ "github.com/observiq/blitz/generator/fix/catalog/v44/session"
	_ "github.com/observiq/blitz/generator/fix/catalog/v44/structured"
	_ "github.com/observiq/blitz/generator/fix/catalog/v50sp2"
)

const componentName = "fix"

// Config configures the FIX generator.
type Config struct {
	// Workers spawned for parallel emission. Each worker maintains its
	// own Session and RNG.
	Workers int
	// Rate is the per-worker emission rate (one message per Rate).
	Rate time.Duration
	// Version selects the FIX protocol version emitted. If
	// VersionUnknown, defaults to V44.
	Version catalog.Version
	// SenderCompID and TargetCompID identify the FIX endpoints in the
	// emitted header. Workers add a worker-index suffix to SenderCompID.
	SenderCompID string
	TargetCompID string
	// EnabledCategories restricts emission to a subset. Empty = all.
	EnabledCategories []catalog.AssetCategory
	// Seed is the base RNG seed. Negative = randomize per worker;
	// 0+ = deterministic (worker N gets seed Seed+N).
	Seed int64
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		Workers:      1,
		Rate:         time.Second,
		Version:      catalog.V44,
		SenderCompID: "BLITZ",
		TargetCompID: "VENUE",
		Seed:         -1, // randomize
	}
}

// Generator emits FIX messages at the configured rate, pushing each
// framed message as a single log record into the configured consumer.
type Generator struct {
	embed.ProducerMarker

	logger   *zap.Logger
	cfg      Config
	consumer embed.LogConsumer

	wg     sync.WaitGroup
	stopCh chan struct{}
}

// New constructs a FIX Generator. The consumer receives each generated
// FIX message as a size-1 batch via ConsumeLogs. Returns an error for
// invalid inputs (nil logger, nil consumer, workers < 1, non-positive
// rate).
func New(logger *zap.Logger, cfg Config, consumer embed.LogConsumer) (*Generator, error) {
	if logger == nil {
		return nil, fmt.Errorf("logger cannot be nil")
	}
	if consumer == nil {
		return nil, fmt.Errorf("consumer cannot be nil")
	}
	if cfg.Version == catalog.VersionUnknown {
		cfg.Version = catalog.V44
	}
	if cfg.Workers < 1 {
		return nil, fmt.Errorf("workers must be 1 or greater, got %d", cfg.Workers)
	}
	if cfg.Rate <= 0 {
		return nil, fmt.Errorf("rate must be positive, got %v", cfg.Rate)
	}
	if cfg.SenderCompID == "" {
		cfg.SenderCompID = "BLITZ"
	}
	if cfg.TargetCompID == "" {
		cfg.TargetCompID = "VENUE"
	}
	if len(cfg.EnabledCategories) == 0 {
		cfg.EnabledCategories = catalog.AllAssetCategories()
	}
	return &Generator{
		logger:   logger,
		cfg:      cfg,
		consumer: consumer,
		stopCh:   make(chan struct{}),
	}, nil
}

// Name returns the module identifier.
func (g *Generator) Name() string { return componentName }

// Start launches the worker goroutines.
func (g *Generator) Start(_ context.Context) error {
	g.logger.Info("Starting FIX generator",
		zap.Int("workers", g.cfg.Workers),
		zap.Duration("rate", g.cfg.Rate),
		zap.String("version", g.cfg.Version.String()),
	)
	for i := 0; i < g.cfg.Workers; i++ {
		g.wg.Add(1)
		go g.runWorker(i)
	}
	return nil
}

// Stop signals workers to drain and waits for them to exit. Returns
// when all workers have stopped or ctx is canceled.
func (g *Generator) Stop(ctx context.Context) error {
	g.logger.Info("Stopping FIX generator")
	close(g.stopCh)

	done := make(chan struct{})
	go func() {
		g.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("stop cancelled due to context cancellation: %w", ctx.Err())
	}
}

func (g *Generator) runWorker(workerIdx int) {
	defer g.wg.Done()

	seed := g.cfg.Seed
	if seed < 0 {
		seed = time.Now().UnixNano() + int64(workerIdx)
	} else {
		seed += int64(workerIdx)
	}
	r := rand.New(rand.NewSource(seed)) // #nosec G404 -- seeded for determinism contract

	sender := fmt.Sprintf("%s-W%d", g.cfg.SenderCompID, workerIdx)
	target := g.cfg.TargetCompID
	sess := state.NewSession(sender, target)

	ticker := time.NewTicker(g.cfg.Rate)
	defer ticker.Stop()

	for {
		select {
		case <-g.stopCh:
			return
		case <-ticker.C:
			msg := g.buildOneMessage(r, sess)
			if msg == nil {
				continue
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			rec := embed.LogRecord{
				Message: string(msg),
				Metadata: embed.LogRecordMetadata{
					Severity: "INFO",
					Resource: resource.Default(componentName,
						"fix.version", g.cfg.Version.String(),
					),
				},
			}
			if err := g.consumer.ConsumeLogs(ctx, []embed.LogRecord{rec}); err != nil {
				g.logger.Debug("FIX emit failed", zap.Error(err))
			}
			cancel()
		}
	}
}

// buildOneMessage selects a random enabled category, a random MsgType
// for that category, and constructs one fully-framed FIX message.
// Returns nil if no MessageDefinition is registered for the chosen
// (Version, MsgType, AssetCategory) — should not happen in practice.
func (g *Generator) buildOneMessage(r *rand.Rand, sess *state.Session) []byte {
	cat := g.cfg.EnabledCategories[r.Intn(len(g.cfg.EnabledCategories))] // #nosec G404
	msgType := pickMsgTypeForCategory(r, cat, sess)

	def := catalog.Get(catalog.MessageKey{
		Version:       g.cfg.Version,
		MsgType:       msgType,
		AssetCategory: cat,
	})
	if def == nil {
		// Fall back to asset-agnostic skeleton.
		def = catalog.Get(catalog.MessageKey{
			Version:       g.cfg.Version,
			MsgType:       msgType,
			AssetCategory: catalog.AssetCategoryUnknown,
		})
	}
	if def == nil {
		return nil
	}

	seqNum := sess.NextOutSeqNum()
	sendingTime := time.Unix(0, int64(seqNum)*int64(time.Millisecond)).UTC().Format("20060102-15:04:05.000")

	ctx := &catalog.GenerateCtx{
		Version:       g.cfg.Version,
		AssetCategory: cat,
		SenderCompID:  sess.SenderCompID,
		TargetCompID:  sess.TargetCompID,
		SeqNum:        seqNum,
		SendingTime:   sendingTime,
	}

	header := []catalog.Field{
		{Tag: catalog.TagMsgType, Value: msgType},
		{Tag: catalog.TagSenderCompID, Value: sess.SenderCompID},
		{Tag: catalog.TagTargetCompID, Value: sess.TargetCompID},
		{Tag: catalog.TagMsgSeqNum, Value: fmt.Sprintf("%d", seqNum)},
		{Tag: catalog.TagSendingTime, Value: sendingTime},
	}

	body := make([]catalog.Field, 0, len(header)+len(def.Fields))
	body = append(body, header...)
	for _, gen := range def.Fields {
		body = append(body, gen(r, ctx))
	}

	// Track NewOrderSingle in the per-category book so subsequent
	// messages (ExecutionReport / cancel) can reference it.
	if msgType == app.MsgTypeNewOrderSingle {
		var clOrdID, symbol, side, qty string
		for _, f := range body {
			switch f.Tag {
			case app.TagClOrdID:
				clOrdID = f.Value
			case app.TagSymbol:
				symbol = f.Value
			case app.TagSide:
				side = f.Value
			case app.TagOrderQty:
				qty = f.Value
			}
		}
		if clOrdID != "" {
			sess.AddOrder(cat, state.Order{
				ClOrdID:   clOrdID,
				OrderID:   sess.NextExecID(),
				Symbol:    symbol,
				Side:      side,
				OrderQty:  state.ParseOrderQty(qty),
				LeavesQty: state.ParseOrderQty(qty),
				Status:    state.OrderStatusNew,
				Submitted: time.Now(),
			})
		}
	}

	return catalog.BuildMessage(g.cfg.Version.BeginString(), body)
}

// pickMsgTypeForCategory chooses a MsgType for emission, biased toward
// realistic distributions: most messages are NewOrderSingle (~50%) and
// ExecutionReport (~40%); cancel/replace/status are rarer.
func pickMsgTypeForCategory(r *rand.Rand, _ catalog.AssetCategory, sess *state.Session) string {
	roll := r.Intn(100) // #nosec G404
	switch {
	case roll < 50:
		return app.MsgTypeNewOrderSingle
	case roll < 90:
		return app.MsgTypeExecutionReport
	case roll < 95:
		return app.MsgTypeOrderCancelRequest
	case roll < 98:
		return app.MsgTypeOrderCancelReplaceRequest
	default:
		_ = sess
		return app.MsgTypeOrderStatusRequest
	}
}
