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
	jsongen "github.com/observiq/blitz/generator/json"
	"github.com/observiq/blitz/generator/kubernetes"
	"github.com/observiq/blitz/generator/nginx"
	"github.com/observiq/blitz/generator/okta"
	"github.com/observiq/blitz/generator/paloalto"
	"github.com/observiq/blitz/generator/postgres"
	"github.com/observiq/blitz/generator/wel"
	welcatalog "github.com/observiq/blitz/generator/wel/catalog"
	"github.com/observiq/blitz/internal/config"
	"go.uber.org/zap"
)

// ForEmbed constructs an embed.ProducerModule for the given generator
// config wired to the supplied log consumer. fileGenLibrary is optional
// and only consulted by the filegen generator: pass embeddedlibrary.FS()
// (with the `embed_library` build tag set) to use the snapshot shipped
// in the blitz module, or nil to fall back to reading ./data_library/
// from the process cwd.
//
// Returns an error for non-Producer generator types (nop, winevt,
// hostmetrics, traces) — these either don't yield logs at all or
// are not yet migrated to the embed.LogConsumer contract.
func ForEmbed(logger *zap.Logger, genCfg config.Generator, consumer embed.LogConsumer, fileGenLibrary fs.FS) (embed.ProducerModule, error) {
	if logger == nil {
		return nil, fmt.Errorf("logger cannot be nil")
	}
	if consumer == nil {
		return nil, fmt.Errorf("consumer cannot be nil")
	}
	switch genCfg.Type {
	case config.GeneratorTypeJSON:
		return jsongen.New(logger, genCfg.JSON.Workers, genCfg.JSON.Rate, genCfg.JSON.Type, consumer)
	case config.GeneratorTypePaloAlto:
		return paloalto.New(logger, genCfg.PaloAlto.Workers, genCfg.PaloAlto.Rate, consumer)
	case config.GeneratorTypeApache:
		return apachegen.New(logger, genCfg.Apache.Workers, genCfg.Apache.Rate, consumer)
	case config.GeneratorTypeApacheCombined:
		return apachecombinedgen.New(logger, genCfg.ApacheCombined.Workers, genCfg.ApacheCombined.Rate, consumer)
	case config.GeneratorTypeApacheError:
		return apacheerrorgen.New(logger, genCfg.ApacheError.Workers, genCfg.ApacheError.Rate, consumer)
	case config.GeneratorTypeNginx:
		return nginx.New(logger, genCfg.Nginx.Workers, genCfg.Nginx.Rate, consumer)
	case config.GeneratorTypePostgres:
		return postgres.New(logger, genCfg.Postgres.Workers, genCfg.Postgres.Rate, consumer)
	case config.GeneratorTypeKubernetes:
		return kubernetes.New(logger, genCfg.Kubernetes.Workers, genCfg.Kubernetes.Rate, genCfg.Kubernetes.Format, consumer)
	case config.GeneratorTypeFile:
		return filegen.New(logger, genCfg.Filegen.Workers, genCfg.Filegen.Rate, genCfg.Filegen.Source, genCfg.Filegen.CacheEnabled, genCfg.Filegen.CacheTTL, consumer, fileGenLibrary)
	case config.GeneratorTypeOkta:
		return okta.New(logger, genCfg.Okta.Workers, genCfg.Okta.Rate, consumer)
	case config.GeneratorTypeWel:
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
			Consumer: consumer,
		})
	case config.GeneratorTypeFIX:
		return newFIX(logger, genCfg.FIX, consumer)
	case config.GeneratorTypeNop:
		return nil, fmt.Errorf("generator type %q does not produce log records; not embed-eligible", genCfg.Type)
	case config.GeneratorTypeWinevt:
		return nil, fmt.Errorf("generator type %q is the legacy single-template Windows Event XML generator; use the multi-channel `wel` generator instead, or run the standalone blitz CLI", genCfg.Type)
	case config.GeneratorTypeHostMetrics:
		return nil, fmt.Errorf("generator type %q produces metrics, not logs; the embed contract supports a separate MetricConsumer path that is not yet wired for this generator", genCfg.Type)
	case config.GeneratorTypeTraces:
		return nil, fmt.Errorf("generator type %q produces traces, not logs; the embed contract supports a separate TraceConsumer path that is not yet wired for this generator", genCfg.Type)
	default:
		return nil, fmt.Errorf("unknown generator type %q", genCfg.Type)
	}
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
