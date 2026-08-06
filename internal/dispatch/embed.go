// Package dispatch maps blitz generator configuration to constructed
// generator instances. The functions here are the canonical mapping
// from `config.Generator.Type` to the corresponding generator package's
// constructor — used both by the standalone CLI (cmd/blitz) and by the
// public embed entry point (github.com/observiq/blitz/config).
package dispatch

import (
	"fmt"
	"io/fs"

	"github.com/observiq/blitz/embed"
	apachegen "github.com/observiq/blitz/generator/apache"
	apachecombinedgen "github.com/observiq/blitz/generator/apache_combined"
	apacheerrorgen "github.com/observiq/blitz/generator/apache_error"
	"github.com/observiq/blitz/generator/filegen"
	fixgen "github.com/observiq/blitz/generator/fix"
	"github.com/observiq/blitz/generator/fix/catalog"
	"github.com/observiq/blitz/generator/hostmetrics"
	jsongen "github.com/observiq/blitz/generator/json"
	"github.com/observiq/blitz/generator/kubernetes"
	"github.com/observiq/blitz/generator/nginx"
	"github.com/observiq/blitz/generator/okta"
	"github.com/observiq/blitz/generator/paloalto"
	"github.com/observiq/blitz/generator/postgres"
	tracesgen "github.com/observiq/blitz/generator/traces"
	"github.com/observiq/blitz/generator/wel"
	welcatalog "github.com/observiq/blitz/generator/wel/catalog"
	"github.com/observiq/blitz/internal/config"
	"github.com/observiq/blitz/internal/datagen"
	"go.uber.org/zap"
)

// EmbedConsumers bundles the per-signal consumers an embedding host can
// supply. LogConsumer is required for any log-yielding generator;
// MetricConsumer is required for metric-yielding generators
// (hostmetrics); TraceConsumer is required for trace-yielding
// generators (traces). ForEmbed only consults the field that matches
// the requested generator type — pass nil for the others when their
// signal isn't needed.
type EmbedConsumers struct {
	LogConsumer    embed.LogConsumer
	MetricConsumer embed.MetricConsumer
	TraceConsumer  embed.TraceConsumer
}

func (c EmbedConsumers) requireLog(typ config.GeneratorType) error {
	if c.LogConsumer == nil {
		return fmt.Errorf("generator type %q requires EmbedConsumers.LogConsumer", typ)
	}
	return nil
}

func (c EmbedConsumers) requireMetric(typ config.GeneratorType) error {
	if c.MetricConsumer == nil {
		return fmt.Errorf("generator type %q requires EmbedConsumers.MetricConsumer", typ)
	}
	return nil
}

func (c EmbedConsumers) requireTrace(typ config.GeneratorType) error {
	if c.TraceConsumer == nil {
		return fmt.Errorf("generator type %q requires EmbedConsumers.TraceConsumer", typ)
	}
	return nil
}

// ForEmbed constructs an embed.ProducerModule for the given generator
// config wired to the relevant consumer in `consumers`. fileGenLibrary
// is optional and only consulted by the filegen generator: pass
// embeddedlibrary.FS() (with the `embed_library` build tag set) to use
// the snapshot shipped in the blitz module, or nil to fall back to
// reading ./data_library/ from the process cwd.
//
// env is the simulated identity environment (PIPE-1036). When non-nil, the
// generator's host identity is resolved from it (see hostIdentity) so emitted
// records carry the simulated host's host.* / os.* / deployment.* attributes;
// when nil, generators fall back to the running process's hostname.
//
// Returns an error when the configured generator type requires a
// consumer that is nil in `consumers` (e.g. hostmetrics without a
// MetricConsumer, traces without a TraceConsumer), and for generator
// types that are not embed-eligible at all (nop, winevt — see PIPE-1032).
func ForEmbed(logger *zap.Logger, genCfg config.Generator, consumers EmbedConsumers, fileGenLibrary fs.FS, env *datagen.Environment, tel embed.TelemetrySettings) (embed.ProducerModule, error) {
	if logger == nil {
		return nil, fmt.Errorf("logger cannot be nil")
	}
	switch genCfg.Type {
	case config.GeneratorTypeJSON:
		if err := consumers.requireLog(genCfg.Type); err != nil {
			return nil, err
		}
		mod, err := jsongen.New(logger, genCfg.JSON.Workers, genCfg.JSON.Rate, genCfg.JSON.Type, consumers.LogConsumer, tel)
		return applyHostIdentity(mod, err, env, genCfg.Type)
	case config.GeneratorTypePaloAlto:
		if err := consumers.requireLog(genCfg.Type); err != nil {
			return nil, err
		}
		mod, err := paloalto.New(logger, genCfg.PaloAlto.Workers, genCfg.PaloAlto.Rate, consumers.LogConsumer, tel)
		return applyHostIdentity(mod, err, env, genCfg.Type)
	case config.GeneratorTypeApache:
		if err := consumers.requireLog(genCfg.Type); err != nil {
			return nil, err
		}
		mod, err := apachegen.New(logger, genCfg.Apache.Workers, genCfg.Apache.Rate, consumers.LogConsumer, tel)
		return applyHostIdentity(mod, err, env, genCfg.Type)
	case config.GeneratorTypeApacheCombined:
		if err := consumers.requireLog(genCfg.Type); err != nil {
			return nil, err
		}
		mod, err := apachecombinedgen.New(logger, genCfg.ApacheCombined.Workers, genCfg.ApacheCombined.Rate, consumers.LogConsumer, tel)
		return applyHostIdentity(mod, err, env, genCfg.Type)
	case config.GeneratorTypeApacheError:
		if err := consumers.requireLog(genCfg.Type); err != nil {
			return nil, err
		}
		mod, err := apacheerrorgen.New(logger, genCfg.ApacheError.Workers, genCfg.ApacheError.Rate, consumers.LogConsumer, tel)
		return applyHostIdentity(mod, err, env, genCfg.Type)
	case config.GeneratorTypeNginx:
		if err := consumers.requireLog(genCfg.Type); err != nil {
			return nil, err
		}
		mod, err := nginx.New(logger, genCfg.Nginx.Workers, genCfg.Nginx.Rate, consumers.LogConsumer, tel)
		return applyHostIdentity(mod, err, env, genCfg.Type)
	case config.GeneratorTypePostgres:
		if err := consumers.requireLog(genCfg.Type); err != nil {
			return nil, err
		}
		mod, err := postgres.New(logger, genCfg.Postgres.Workers, genCfg.Postgres.Rate, consumers.LogConsumer, tel)
		return applyHostIdentity(mod, err, env, genCfg.Type)
	case config.GeneratorTypeKubernetes:
		if err := consumers.requireLog(genCfg.Type); err != nil {
			return nil, err
		}
		mod, err := kubernetes.New(logger, genCfg.Kubernetes.Workers, genCfg.Kubernetes.Rate, genCfg.Kubernetes.Format, consumers.LogConsumer, tel)
		return applyHostIdentity(mod, err, env, genCfg.Type)
	case config.GeneratorTypeFile:
		if err := consumers.requireLog(genCfg.Type); err != nil {
			return nil, err
		}
		mod, err := filegen.New(logger, genCfg.Filegen.Workers, genCfg.Filegen.Rate, genCfg.Filegen.Source, genCfg.Filegen.CacheEnabled, genCfg.Filegen.CacheTTL, consumers.LogConsumer, fileGenLibrary, tel)
		return applyHostIdentity(mod, err, env, genCfg.Type)
	case config.GeneratorTypeOkta:
		if err := consumers.requireLog(genCfg.Type); err != nil {
			return nil, err
		}
		mod, err := okta.New(logger, genCfg.Okta.Workers, genCfg.Okta.Rate, consumers.LogConsumer, tel)
		return applyHostIdentity(mod, err, env, genCfg.Type)
	case config.GeneratorTypeWel:
		if err := consumers.requireLog(genCfg.Type); err != nil {
			return nil, err
		}
		role := welcatalog.MachineRole(genCfg.Wel.Role)
		if role == "" {
			role = welcatalog.RoleMember
		}
		mod, err := wel.New(wel.Config{
			Logger:    logger,
			Workers:   genCfg.Wel.Workers,
			Rate:      genCfg.Wel.Rate,
			Computer:  genCfg.Wel.Computer,
			Domain:    genCfg.Wel.Domain,
			Role:      role,
			Channels:  genCfg.Wel.Channels,
			Consumer:  consumers.LogConsumer,
			Telemetry: tel,
		})
		return applyHostIdentity(mod, err, env, genCfg.Type)
	case config.GeneratorTypeFIX:
		if err := consumers.requireLog(genCfg.Type); err != nil {
			return nil, err
		}
		mod, err := newFIX(logger, genCfg.FIX, consumers.LogConsumer, tel)
		return applyHostIdentity(mod, err, env, genCfg.Type)
	case config.GeneratorTypeHostMetrics:
		if err := consumers.requireMetric(genCfg.Type); err != nil {
			return nil, err
		}
		return hostmetrics.New(hostmetrics.Config{
			Logger:       logger,
			Workers:      genCfg.HostMetrics.Workers,
			Rate:         genCfg.HostMetrics.Rate,
			OS:           genCfg.HostMetrics.OS,
			Hostname:     genCfg.HostMetrics.Hostname,
			ScraperNames: genCfg.HostMetrics.Scrapers,
			Consumer:     consumers.MetricConsumer,
			Seed:         yamlSeedDefault(genCfg.HostMetrics.Seed),
			Identity:     hostIdentity(env, genCfg.Type),
			Telemetry:    tel,
		})
	case config.GeneratorTypeTraces:
		if err := consumers.requireTrace(genCfg.Type); err != nil {
			return nil, err
		}
		return tracesgen.New(tracesgen.Config{
			Logger:    logger,
			Workers:   genCfg.Traces.Workers,
			Rate:      genCfg.Traces.Rate,
			Hostname:  genCfg.Traces.Hostname,
			Consumer:  consumers.TraceConsumer,
			Seed:      yamlSeedDefault(genCfg.Traces.Seed),
			Identity:  hostIdentity(env, genCfg.Type),
			Telemetry: tel,
		})
	case config.GeneratorTypeNop:
		return nil, fmt.Errorf("generator type %q does not produce records; not embed-eligible", genCfg.Type)
	case config.GeneratorTypeWinevt:
		return nil, fmt.Errorf("generator type %q is DEPRECATED and is not available via embed; the legacy single-template Windows Event XML generator has been superseded by the multi-channel `wel` generator (see docs/generator/wel.md). The standalone blitz CLI still accepts `winevt` with a deprecation warning", genCfg.Type)
	default:
		return nil, fmt.Errorf("unknown generator type %q", genCfg.Type)
	}
}

// hostIdentitySetter is implemented by the log generators, whose positional
// constructors take identity after construction (via SetHostIdentity) rather
// than as a constructor argument. Metric- and trace-yielding generators take
// their identity as a Config field instead, so they do not implement this.
type hostIdentitySetter interface {
	SetHostIdentity(*datagen.SystemIdentity)
}

// applyHostIdentity resolves the component's simulated host from env and applies
// it to a just-constructed log generator, then returns it for the ForEmbed case
// to hand back. It is a no-op when construction failed (err != nil) or when the
// module does not accept a post-construction identity.
func applyHostIdentity(mod embed.ProducerModule, err error, env *datagen.Environment, component config.GeneratorType) (embed.ProducerModule, error) {
	if err != nil {
		return nil, err
	}
	if setter, ok := mod.(hostIdentitySetter); ok {
		setter.SetHostIdentity(hostIdentity(env, component))
	}
	return mod, nil
}

// hostIdentity resolves the simulated host a generator component's records
// describe: the environment's deterministic SystemForKey selection keyed by the
// generator type, so the same component always maps to the same host and
// distinct components spread across the fleet. Returns nil when no environment
// is configured, leaving the generator on its process-hostname fallback.
//
// Keying by component gives one host per generator (the default granularity).
// Finer per-worker granularity — one host per worker — is a future opt-in that
// would key SystemForKey by component plus worker index.
func hostIdentity(env *datagen.Environment, component config.GeneratorType) *datagen.SystemIdentity {
	if env == nil {
		return nil
	}
	return env.SystemForKey(string(component))
}

// yamlSeedDefault translates a YAML-loaded Seed value into the
// generator-Config Seed value, applying the "stochastic by default"
// architectural intent for YAML users. YAML zero-value (omitted `seed:`
// key) and an explicit `seed: 0` both map to -1 (randomize per worker).
// Any other value passes through unchanged.
//
// Programmatic Go callers bypass this translation and observe whatever
// literal value they pass — useful for tests that want to pin seed=0.
//
// PIPE-1036 will route per-machine identity determinism through the
// top-level `environment.seed_config` instead; this knob will then
// govern only the generator's record-content RNG, not host identity.
func yamlSeedDefault(yamlSeed int64) int64 {
	if yamlSeed == 0 {
		return -1
	}
	return yamlSeed
}

// newFIX translates the YAML-shaped FIXGeneratorConfig into the
// catalog-typed fix.Config and constructs a FIX generator. Version and
// EnabledCategories strings are validated; an empty version defaults to
// FIX 4.4 and an empty EnabledCategories means "all 10 categories".
func newFIX(logger *zap.Logger, cfg config.FIXGeneratorConfig, consumer embed.LogConsumer, tel embed.TelemetrySettings) (embed.ProducerModule, error) {
	fc := fixgen.Config{
		Workers:      cfg.Workers,
		Rate:         cfg.Rate,
		SenderCompID: cfg.SenderCompID,
		TargetCompID: cfg.TargetCompID,
		Seed:         cfg.Seed,
	}
	if cfg.Version != "" {
		v, err := catalog.VersionFromString(cfg.Version)
		if err != nil {
			return nil, fmt.Errorf("fix: %w", err)
		}
		fc.Version = v
	}
	for _, s := range cfg.EnabledCategories {
		c, err := catalog.AssetCategoryFromString(s)
		if err != nil {
			return nil, fmt.Errorf("fix: %w", err)
		}
		fc.EnabledCategories = append(fc.EnabledCategories, c)
	}
	return fixgen.New(logger, fc, consumer, tel)
}
