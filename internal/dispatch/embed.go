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
// Returns an error when the configured generator type requires a
// consumer that is nil in `consumers` (e.g. hostmetrics without a
// MetricConsumer, traces without a TraceConsumer), and for generator
// types that are not embed-eligible at all (nop, winevt — see PIPE-1032).
func ForEmbed(logger *zap.Logger, genCfg config.Generator, consumers EmbedConsumers, fileGenLibrary fs.FS) (embed.ProducerModule, error) {
	if logger == nil {
		return nil, fmt.Errorf("logger cannot be nil")
	}
	switch genCfg.Type {
	case config.GeneratorTypeJSON:
		if err := consumers.requireLog(genCfg.Type); err != nil {
			return nil, err
		}
		return jsongen.New(logger, genCfg.JSON.Workers, genCfg.JSON.Rate, genCfg.JSON.Type, consumers.LogConsumer)
	case config.GeneratorTypePaloAlto:
		if err := consumers.requireLog(genCfg.Type); err != nil {
			return nil, err
		}
		return paloalto.New(logger, genCfg.PaloAlto.Workers, genCfg.PaloAlto.Rate, consumers.LogConsumer)
	case config.GeneratorTypeApache:
		if err := consumers.requireLog(genCfg.Type); err != nil {
			return nil, err
		}
		return apachegen.New(logger, genCfg.Apache.Workers, genCfg.Apache.Rate, consumers.LogConsumer)
	case config.GeneratorTypeApacheCombined:
		if err := consumers.requireLog(genCfg.Type); err != nil {
			return nil, err
		}
		return apachecombinedgen.New(logger, genCfg.ApacheCombined.Workers, genCfg.ApacheCombined.Rate, consumers.LogConsumer)
	case config.GeneratorTypeApacheError:
		if err := consumers.requireLog(genCfg.Type); err != nil {
			return nil, err
		}
		return apacheerrorgen.New(logger, genCfg.ApacheError.Workers, genCfg.ApacheError.Rate, consumers.LogConsumer)
	case config.GeneratorTypeNginx:
		if err := consumers.requireLog(genCfg.Type); err != nil {
			return nil, err
		}
		return nginx.New(logger, genCfg.Nginx.Workers, genCfg.Nginx.Rate, consumers.LogConsumer)
	case config.GeneratorTypePostgres:
		if err := consumers.requireLog(genCfg.Type); err != nil {
			return nil, err
		}
		return postgres.New(logger, genCfg.Postgres.Workers, genCfg.Postgres.Rate, consumers.LogConsumer)
	case config.GeneratorTypeKubernetes:
		if err := consumers.requireLog(genCfg.Type); err != nil {
			return nil, err
		}
		return kubernetes.New(logger, genCfg.Kubernetes.Workers, genCfg.Kubernetes.Rate, genCfg.Kubernetes.Format, consumers.LogConsumer)
	case config.GeneratorTypeFile:
		if err := consumers.requireLog(genCfg.Type); err != nil {
			return nil, err
		}
		return filegen.New(logger, genCfg.Filegen.Workers, genCfg.Filegen.Rate, genCfg.Filegen.Source, genCfg.Filegen.CacheEnabled, genCfg.Filegen.CacheTTL, consumers.LogConsumer, fileGenLibrary)
	case config.GeneratorTypeOkta:
		if err := consumers.requireLog(genCfg.Type); err != nil {
			return nil, err
		}
		return okta.New(logger, genCfg.Okta.Workers, genCfg.Okta.Rate, consumers.LogConsumer)
	case config.GeneratorTypeWel:
		if err := consumers.requireLog(genCfg.Type); err != nil {
			return nil, err
		}
		role := welcatalog.MachineRole(genCfg.Wel.Role)
		if role == "" {
			role = welcatalog.RoleMember
		}
		return wel.New(wel.Config{
			Logger:   logger,
			Workers:  genCfg.Wel.Workers,
			Rate:     genCfg.Wel.Rate,
			Computer: genCfg.Wel.Computer,
			Domain:   genCfg.Wel.Domain,
			Role:     role,
			Channels: genCfg.Wel.Channels,
			Consumer: consumers.LogConsumer,
		})
	case config.GeneratorTypeFIX:
		if err := consumers.requireLog(genCfg.Type); err != nil {
			return nil, err
		}
		return newFIX(logger, genCfg.FIX, consumers.LogConsumer)
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
		})
	case config.GeneratorTypeTraces:
		if err := consumers.requireTrace(genCfg.Type); err != nil {
			return nil, err
		}
		return tracesgen.New(tracesgen.Config{
			Logger:   logger,
			Workers:  genCfg.Traces.Workers,
			Rate:     genCfg.Traces.Rate,
			Hostname: genCfg.Traces.Hostname,
			Consumer: consumers.TraceConsumer,
			Seed:     yamlSeedDefault(genCfg.Traces.Seed),
		})
	case config.GeneratorTypeNop:
		return nil, fmt.Errorf("generator type %q does not produce records; not embed-eligible", genCfg.Type)
	case config.GeneratorTypeWinevt:
		return nil, fmt.Errorf("generator type %q is DEPRECATED and is not available via embed; the legacy single-template Windows Event XML generator has been superseded by the multi-channel `wel` generator (see docs/generator/wel.md). The standalone blitz CLI still accepts `winevt` with a deprecation warning", genCfg.Type)
	default:
		return nil, fmt.Errorf("unknown generator type %q", genCfg.Type)
	}
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
func newFIX(logger *zap.Logger, cfg config.FIXGeneratorConfig, consumer embed.LogConsumer) (embed.ProducerModule, error) {
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
	return fixgen.New(logger, fc, consumer)
}
