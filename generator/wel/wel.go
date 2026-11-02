package wel

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/observiq/blitz/embed"
	"github.com/observiq/blitz/generator"
	"github.com/observiq/blitz/generator/count"
	"github.com/observiq/blitz/generator/resource"
	"github.com/observiq/blitz/generator/wel/catalog"
	"github.com/observiq/blitz/internal/datagen"
	"github.com/observiq/blitz/internal/generator/pacing"
	"github.com/observiq/blitz/telemetry"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"
)

const componentName = "wel"

// Compile-time assertion: WEL XML mode is a Producer module.
var _ embed.ProducerModule = (*Generator)(nil)

// Generator generates Windows Event Log entries (XML mode) and yields
// them as embed.LogRecord batches through a host-supplied consumer.
type Generator struct {
	embed.ProducerMarker

	logger   *zap.Logger
	workers  int
	rate     time.Duration
	computer string
	domain   string
	role     catalog.MachineRole
	channels []string
	consumer embed.LogConsumer
	static   *resource.StaticResources

	registry *catalog.Registry
	state    *catalog.StateTracker
	opts     *catalog.GenerateOpts

	metrics *generator.Metrics
	tracker *count.Tracker

	wg     sync.WaitGroup
	stopCh chan struct{}

	recordID atomic.Int64
}

// Config holds the WEL generator configuration. Consumer is mandatory:
// it is the destination for every record this generator produces, the
// same way the consumer wiring works for every other Producer module.
type Config struct {
	Logger   *zap.Logger
	Workers  int
	Rate     time.Duration
	Computer string
	Domain   string
	Role     catalog.MachineRole
	Channels []string
	Consumer embed.LogConsumer

	// Identity is the simulated host every record describes.
	// Its hostname is host.name and the payload computer name unless Computer
	// is set, in which case Computer replaces only the hostname; with
	// neither, it is BLITZ-WEL. A domain-joined identity writes its FQDN in
	// <Computer> and its NetBIOS domain as the account domain unless Domain
	// is set; otherwise <Computer> is the bare name and Domain WORKGROUP.
	Identity *datagen.SystemIdentity

	// Telemetry supplies the OTel providers blitz routes its own
	// self-telemetry through. The zero value falls back to the process
	// global providers.
	Telemetry embed.TelemetrySettings

	// Environment data — caller-owned slices. The generator stores the
	// reference; callers must not mutate the underlying arrays after
	// passing them in.
	Usernames []string
	IPs       []string
	Hostnames []string
}

// workgroup is the account domain of a machine joined to no domain.
const workgroup = "WORKGROUP"

// roleFor maps the simulated host's role to the WEL event pool it emits, so an
// unset role follows the host: a DC emits DC events, a workstation workstation
// events. Servers and a missing identity emit the member-server pool.
func roleFor(id *datagen.SystemIdentity) catalog.MachineRole {
	if id == nil {
		return catalog.RoleMember
	}
	switch id.Role {
	case datagen.RoleDC:
		return catalog.RoleDC
	case datagen.RoleWorkstation:
		return catalog.RoleWorkstation
	default:
		return catalog.RoleMember
	}
}

// New creates a new WEL generator.
func New(cfg Config) (*Generator, error) {
	if cfg.Logger == nil {
		return nil, fmt.Errorf("logger cannot be nil")
	}
	if cfg.Consumer == nil {
		return nil, fmt.Errorf("consumer cannot be nil")
	}
	if cfg.Workers < 1 {
		return nil, fmt.Errorf("workers must be 1 or greater, got %d", cfg.Workers)
	}
	if cfg.Role == "" {
		cfg.Role = roleFor(cfg.Identity)
	}
	// Resolve the one hostname: a set Computer wins and replaces only the
	// identity's hostname; else the identity's; else BLITZ-WEL.
	sys := cfg.Identity.WithHostname(cfg.Computer)
	if sys == nil {
		sys = &datagen.SystemIdentity{Hostname: "BLITZ-WEL"}
	}
	cfg.Computer = sys.Hostname
	// A domain-joined host writes its FQDN in <Computer> and its NetBIOS
	// domain as the account domain; a workgroup host its bare name and
	// WORKGROUP. A set Domain is the DNS domain the host is joined to (so it
	// drives both), or WORKGROUP for a workgroup machine.
	dnsDomain := sys.Domain
	if cfg.Domain != "" {
		dnsDomain = strings.ToLower(cfg.Domain)
		switch {
		case strings.EqualFold(cfg.Domain, workgroup):
			dnsDomain = ""
		case !strings.Contains(cfg.Domain, "."):
			// Earlier releases took a NetBIOS name here.
			cfg.Logger.Warn("wel domain has no dot; reading it as a single-label DNS domain, not a NetBIOS name",
				zap.String("domain", cfg.Domain))
		}
	}
	var fqdn string
	cfg.Domain = workgroup
	if dnsDomain != "" {
		fqdn = sys.Hostname + "." + dnsDomain
		cfg.Domain = datagen.NetBIOSDomainName(dnsDomain)
	}

	// Build registry filtered by role and channels
	reg := catalog.DefaultRegistry(cfg.Role)
	if len(cfg.Channels) > 0 {
		reg = reg.FilterChannels(cfg.Channels)
	}

	channels := reg.Channels()
	if len(channels) == 0 {
		return nil, fmt.Errorf("no events available for role %q with channels %v", cfg.Role, cfg.Channels)
	}

	metrics, err := generator.NewMetrics(cfg.Telemetry.MeterProvider)
	if err != nil {
		return nil, fmt.Errorf("build generator metrics: %w", err)
	}

	state := catalog.NewStateTracker(1000)

	// Defensive clone of the caller-supplied slices. Workers read from
	// these concurrently; a caller that retains its own reference and
	// later mutates the slice would race with the workers otherwise.
	opts := &catalog.GenerateOpts{
		Computer:   cfg.Computer,
		FQDN:       fqdn,
		DomainName: cfg.Domain,
		Role:       cfg.Role,
		Usernames:  cloneStrings(cfg.Usernames),
		IPs:        cloneStrings(cfg.IPs),
		Hostnames:  cloneStrings(cfg.Hostnames),
		State:      state,
	}

	return &Generator{
		logger:   cfg.Logger,
		workers:  cfg.Workers,
		rate:     cfg.Rate,
		computer: cfg.Computer,
		domain:   cfg.Domain,
		role:     cfg.Role,
		channels: channels,
		consumer: cfg.Consumer,
		static: resource.FromIdentity(sys, componentName,
			"wel.computer", cfg.Computer,
			"wel.domain", cfg.Domain,
			"wel.role", string(cfg.Role),
		),
		registry: reg,
		state:    state,
		opts:     opts,
		metrics:  metrics,
		stopCh:   make(chan struct{}),
	}, nil
}

// Name returns the module identifier.
func (g *Generator) Name() string { return componentName }

// Start launches the worker goroutines that yield generated records
// to the configured consumer. Start returns once workers are running.
func (g *Generator) Start(_ context.Context) error {
	g.logger.Info("Starting WEL generator",
		zap.Int("workers", g.workers),
		zap.Duration("rate", g.rate),
		zap.String("role", string(g.role)),
		zap.Strings("channels", g.channels),
	)

	g.metrics.BlitzGeneratorActiveWorkersGauge.Record(context.Background(), int64(g.workers), componentName)

	for i := 0; i < g.workers; i++ {
		g.wg.Add(1)
		go g.worker(i) // #nosec G118 -- workers are bounded by Stop() and the WaitGroup, not the Start context
	}
	return nil
}

// Stop stops the generator.
func (g *Generator) Stop(ctx context.Context) error {
	g.logger.Info("Stopping WEL generator")

	g.metrics.BlitzGeneratorActiveWorkersGauge.Record(ctx, 0, componentName)

	close(g.stopCh)

	done := make(chan struct{})
	go func() {
		g.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		g.logger.Info("All WEL workers stopped gracefully")
		return nil
	case <-ctx.Done():
		return fmt.Errorf("stop cancelled due to context cancellation: %w", ctx.Err())
	}
}

// SupportedTelemetry returns the telemetry types this generator produces.
func (g *Generator) SupportedTelemetry() []telemetry.Type {
	return []telemetry.Type{telemetry.Logs}
}

// SetCountTracker wires the finite-generation count tracker. With it set, each
// worker acquires from the tracker before emitting, so the generator honors the
// configured record budget instead of running unbounded (PIPE-1111).
func (g *Generator) SetCountTracker(t *count.Tracker) {
	g.tracker = t
}

func (g *Generator) worker(workerID int) {
	defer g.wg.Done()
	g.logger.Debug("Starting WEL worker", zap.Int("worker_id", workerID))

	rng := rand.New(rand.NewSource(time.Now().UnixNano() + int64(workerID))) // #nosec G404

	backoffConfig := pacing.NewWriteBackoff(g.rate)

	// Drive the timer from this goroutine only. backoff.ExponentialBackOff is
	// not safe for concurrent use, so we never hand it to backoff.NewTicker's
	// internal goroutine; instead we own every NextBackOff/Reset call here.
	timer := time.NewTimer(backoffConfig.NextBackOff())
	defer timer.Stop()

	for {
		select {
		case <-g.stopCh:
			g.logger.Debug("WEL worker stopping", zap.Int("worker_id", workerID))
			return
		case <-timer.C:
			// Honor the finite-count budget: when the tracker is exhausted,
			// wait for a returned permit, a restart, or shutdown.
			permit, wait, ok := g.tracker.Acquire()
			if !ok {
				if !pacing.AwaitWake(g.stopCh, wait, backoffConfig, timer) {
					return
				}
				continue
			}
			if err := g.generateAndWrite(count.WithPermit(context.Background(), permit), rng); err != nil {
				g.logger.Error("Failed to write WEL event",
					zap.Int("worker_id", workerID),
					zap.Error(err),
				)
				permit.Release()
				timer.Reset(backoffConfig.NextBackOff())
				continue
			}
			permit.Commit()
			backoffConfig.Reset()
			timer.Reset(backoffConfig.NextBackOff())
		}
	}
}

func (g *Generator) generateAndWrite(ctx context.Context, rng *rand.Rand) error {
	// Pick a random event definition
	def := g.registry.RandomEvent(rng)
	if def == nil {
		return fmt.Errorf("no event definitions available")
	}

	// Generate the event record
	recordID := g.recordID.Add(1)
	record := GenerateRecord(rng, def, g.opts, recordID)

	// Render as XML for the consumer
	xml := record.ToXML()

	g.metrics.BlitzGeneratorEntriesCounter.Add(context.Background(), 1, componentName,
		metric.WithAttributeSet(attribute.NewSet(attribute.String("channel", record.Channel))),
	)

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	logRecord := embed.LogRecord{
		Message: xml,
		Metadata: embed.LogRecordMetadata{
			Severity: record.LevelName,
			Resource: g.static.Record("wel.channel", record.Channel),
		},
	}

	if err := g.consumer.ConsumeLogs(ctx, []embed.LogRecord{logRecord}); err != nil {
		errorType := "unknown"
		if ctx.Err() == context.DeadlineExceeded {
			errorType = "timeout"
		}
		g.metrics.BlitzGeneratorWriteErrorsCounter.Add(context.Background(), 1, componentName,
			metric.WithAttributeSet(attribute.NewSet(attribute.String("error_type", errorType))),
		)
		return err
	}

	return nil
}

// cloneStrings returns a defensive copy of s. Constructor uses it so a
// caller that retains a reference to a passed-in slice and later mutates
// it cannot race with worker goroutines reading from opts.
func cloneStrings(s []string) []string {
	if s == nil {
		return nil
	}
	out := make([]string, len(s))
	copy(out, s)
	return out
}
