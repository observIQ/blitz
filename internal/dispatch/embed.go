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
	"github.com/observiq/blitz/generator"
	apachegen "github.com/observiq/blitz/generator/apache"
	apachecombinedgen "github.com/observiq/blitz/generator/apache_combined"
	apacheerrorgen "github.com/observiq/blitz/generator/apache_error"
	"github.com/observiq/blitz/generator/filegen"
	fixgen "github.com/observiq/blitz/generator/fix"
	"github.com/observiq/blitz/generator/fix/catalog"
	flowgen "github.com/observiq/blitz/generator/flow"
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
	FlowConsumer   embed.FlowConsumer
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

func (c EmbedConsumers) requireFlow(typ config.GeneratorType) error {
	if c.FlowConsumer == nil {
		return fmt.Errorf("generator type %q requires EmbedConsumers.FlowConsumer", typ)
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
	return ForEmbedAt(logger, genCfg, 0, consumers, fileGenLibrary, env, tel)
}

// ForEmbedAt is ForEmbed for the entry at ordinal among the configured
// generators of its type (0 for the first), so same-type entries in a
// generators: list describe distinct hosts.
func ForEmbedAt(logger *zap.Logger, genCfg config.Generator, ordinal int, consumers EmbedConsumers, fileGenLibrary fs.FS, env *datagen.Environment, tel embed.TelemetrySettings) (embed.ProducerModule, error) {
	if logger == nil {
		return nil, fmt.Errorf("logger cannot be nil")
	}
	switch genCfg.Type {
	case config.GeneratorTypeJSON:
		if err := consumers.requireLog(genCfg.Type); err != nil {
			return nil, err
		}
		mod, err := jsongen.New(logger, genCfg.JSON.Workers, genCfg.JSON.Rate, genCfg.JSON.Type, consumers.LogConsumer, tel, generator.WithIdentity(hostIdentity(env, genCfg, ordinal)))
		return module(mod, err)
	case config.GeneratorTypePaloAlto:
		if err := consumers.requireLog(genCfg.Type); err != nil {
			return nil, err
		}
		mod, err := paloalto.New(logger, genCfg.PaloAlto.Workers, genCfg.PaloAlto.Rate, consumers.LogConsumer, tel, generator.WithAppliance(applianceIdentity(env, genCfg, ordinal)))
		return module(mod, err)
	case config.GeneratorTypeApache:
		if err := consumers.requireLog(genCfg.Type); err != nil {
			return nil, err
		}
		mod, err := apachegen.New(logger, genCfg.Apache.Workers, genCfg.Apache.Rate, consumers.LogConsumer, tel, generator.WithIdentity(hostIdentity(env, genCfg, ordinal)))
		return module(mod, err)
	case config.GeneratorTypeApacheCombined:
		if err := consumers.requireLog(genCfg.Type); err != nil {
			return nil, err
		}
		mod, err := apachecombinedgen.New(logger, genCfg.ApacheCombined.Workers, genCfg.ApacheCombined.Rate, consumers.LogConsumer, tel, generator.WithIdentity(hostIdentity(env, genCfg, ordinal)))
		return module(mod, err)
	case config.GeneratorTypeApacheError:
		if err := consumers.requireLog(genCfg.Type); err != nil {
			return nil, err
		}
		mod, err := apacheerrorgen.New(logger, genCfg.ApacheError.Workers, genCfg.ApacheError.Rate, consumers.LogConsumer, tel, generator.WithIdentity(hostIdentity(env, genCfg, ordinal)))
		return module(mod, err)
	case config.GeneratorTypeNginx:
		if err := consumers.requireLog(genCfg.Type); err != nil {
			return nil, err
		}
		mod, err := nginx.New(logger, genCfg.Nginx.Workers, genCfg.Nginx.Rate, consumers.LogConsumer, tel, generator.WithIdentity(hostIdentity(env, genCfg, ordinal)))
		return module(mod, err)
	case config.GeneratorTypePostgres:
		if err := consumers.requireLog(genCfg.Type); err != nil {
			return nil, err
		}
		mod, err := postgres.New(logger, genCfg.Postgres.Workers, genCfg.Postgres.Rate, consumers.LogConsumer, tel, generator.WithIdentity(hostIdentity(env, genCfg, ordinal)))
		return module(mod, err)
	case config.GeneratorTypeKubernetes:
		if err := consumers.requireLog(genCfg.Type); err != nil {
			return nil, err
		}
		mod, err := kubernetes.New(logger, genCfg.Kubernetes.Workers, genCfg.Kubernetes.Rate, genCfg.Kubernetes.Format, consumers.LogConsumer, tel, generator.WithIdentity(hostIdentity(env, genCfg, ordinal)))
		return module(mod, err)
	case config.GeneratorTypeFile:
		if err := consumers.requireLog(genCfg.Type); err != nil {
			return nil, err
		}
		mod, err := filegen.New(logger, genCfg.Filegen.Workers, genCfg.Filegen.Rate, genCfg.Filegen.Source, genCfg.Filegen.CacheEnabled, genCfg.Filegen.CacheTTL, consumers.LogConsumer, fileGenLibrary, tel, generator.WithIdentity(hostIdentity(env, genCfg, ordinal)))
		return module(mod, err)
	case config.GeneratorTypeOkta:
		if err := consumers.requireLog(genCfg.Type); err != nil {
			return nil, err
		}
		mod, err := okta.New(logger, genCfg.Okta.Workers, genCfg.Okta.Rate, consumers.LogConsumer, tel, generator.WithIdentity(hostIdentity(env, genCfg, ordinal)))
		return module(mod, err)
	case config.GeneratorTypeWel:
		if err := consumers.requireLog(genCfg.Type); err != nil {
			return nil, err
		}
		// An unset role follows the resolved host (wel.New).
		role := welcatalog.MachineRole(genCfg.Wel.Role)
		mod, err := wel.New(wel.Config{
			Logger:    logger,
			Workers:   genCfg.Wel.Workers,
			Rate:      genCfg.Wel.Rate,
			Computer:  genCfg.Wel.Computer,
			Domain:    genCfg.Wel.Domain,
			Role:      role,
			Channels:  genCfg.Wel.Channels,
			Consumer:  consumers.LogConsumer,
			Identity:  hostIdentity(env, genCfg, ordinal),
			Telemetry: tel,
		})
		return module(mod, err)
	case config.GeneratorTypeFIX:
		if err := consumers.requireLog(genCfg.Type); err != nil {
			return nil, err
		}
		mod, err := newFIX(logger, genCfg.FIX, consumers.LogConsumer, hostIdentity(env, genCfg, ordinal), tel)
		return module(mod, err)
	case config.GeneratorTypeFlow:
		if err := consumers.requireFlow(genCfg.Type); err != nil {
			return nil, err
		}
		mod, err := flowgen.New(logger, genCfg.Flow.Workers, genCfg.Flow.Rate, flowgen.Scenario(genCfg.Flow.Scenario), genCfg.Flow.Seed, consumers.FlowConsumer, tel,
			generator.WithAppliance(flowExporter(env, genCfg, ordinal)))
		return module(mod, err)
	case config.GeneratorTypeHostMetrics:
		if err := consumers.requireMetric(genCfg.Type); err != nil {
			return nil, err
		}
		return hostmetrics.New(hostmetrics.Config{
			Logger:       logger,
			Rate:         genCfg.HostMetrics.Rate,
			OS:           genCfg.HostMetrics.OS,
			Hostname:     genCfg.HostMetrics.Hostname,
			ScraperNames: genCfg.HostMetrics.Scrapers,
			Consumer:     consumers.MetricConsumer,
			Seed:         genCfg.HostMetrics.Seed,
			Identity:     hostIdentity(env, genCfg, ordinal),
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
			Seed:      genCfg.Traces.Seed,
			Identity:  hostIdentity(env, genCfg, ordinal),
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

// module returns a just-constructed generator as a ProducerModule, or a nil
// module on error, so a typed-nil pointer never escapes as a non-nil interface.
func module[T embed.ProducerModule](mod T, err error) (embed.ProducerModule, error) {
	if err != nil {
		return nil, err
	}
	return mod, nil
}

// hostIdentity resolves the simulated host a generator's records describe: the
// environment's deterministic SystemFor selection keyed by the generator type,
// so the same component always maps to the same host and distinct components
// spread across the fleet. The selection honors the generator's
// hostRequirement, so an OS-limited generator only ever gets a host of its OS.
// Returns nil only without an environment: an environment built with
// RequiredHosts always holds a host that meets the requirement.
//
// Keying by component gives one host per generator (the default granularity).
// Finer per-worker granularity — one host per worker — is a future opt-in that
// would key SystemFor by component plus worker index.
func hostIdentity(env *datagen.Environment, genCfg config.Generator, ordinal int) *datagen.SystemIdentity {
	if env == nil {
		return nil
	}
	return env.SystemFor(string(genCfg.Type), ordinal, hostRequirement(genCfg))
}

// applianceVendor is the network appliance a generator describes, or "" for a
// generator that describes a host.
func applianceVendor(t config.GeneratorType) datagen.ApplianceVendor {
	switch t {
	case config.GeneratorTypePaloAlto:
		return datagen.VendorPaloAlto
	default:
		return ""
	}
}

// applianceIdentity resolves the environment appliance an appliance generator
// describes, or nil without an environment.
func applianceIdentity(env *datagen.Environment, genCfg config.Generator, ordinal int) *datagen.NetworkSystemIdentity {
	if env == nil {
		return nil
	}
	return env.ApplianceFor(string(genCfg.Type), ordinal, applianceVendor(genCfg.Type))
}

// RequiredAppliances lists the appliance vendor of each configured generator
// entry that describes one, once per entry, so the environment holds a
// separate device for every entry.
func RequiredAppliances(gens []config.Generator) []datagen.ApplianceVendor {
	var out []datagen.ApplianceVendor
	for _, g := range gens {
		if v := applianceVendor(g.Type); v != "" {
			out = append(out, v)
		}
		if v := exporterVendor(g); v != "" {
			out = append(out, v)
		}
	}
	return out
}

// RequiredRouters counts the flow generators, so the environment holds a
// routing device for each, pinned or not, and every entry gets its own.
func RequiredRouters(gens []config.Generator) int {
	n := 0
	for _, g := range gens {
		if g.Type == config.GeneratorTypeFlow {
			n++
		}
	}
	return n
}

// exporterVendor is a flow generator's configured exporter vendor, or "" for
// any vendor (and for every other generator type). Validate has already
// rejected an unknown spelling.
func exporterVendor(g config.Generator) datagen.ApplianceVendor {
	if g.Type != config.GeneratorTypeFlow {
		return ""
	}
	v, _ := datagen.ParseExporterVendor(g.Flow.ExporterVendor)
	return v
}

// flowExporter resolves the environment routing device a flow generator's
// records come from, or nil without an environment.
func flowExporter(env *datagen.Environment, genCfg config.Generator, ordinal int) *datagen.NetworkSystemIdentity {
	if env == nil {
		return nil
	}
	return env.FlowExporterFor(string(genCfg.Type), ordinal, exporterVendor(genCfg))
}

// BuildEnvironment builds the simulated environment for gens: envCfg's fleet,
// plus a device for every appliance and a system for every host requirement
// the generators need. Both loaders build through it so they cannot drift.
func BuildEnvironment(logger *zap.Logger, envCfg config.EnvironmentConfig, gens []config.Generator) (*datagen.Environment, error) {
	env, err := envCfg.Build(logger, RequiredAppliances(gens), RequiredHosts(gens), RequiredRouters(gens))
	if err != nil {
		return nil, err
	}
	claimPinnedExporters(env, gens)
	return env, nil
}

// claimPinnedExporters resolves the vendor-pinned flow entries' exporters
// before any other, so an entry that takes any routing device can never take
// the device a pinned entry needs. The loaders' later calls return these same
// devices.
func claimPinnedExporters(env *datagen.Environment, gens []config.Generator) {
	ordinal := 0
	for _, g := range gens {
		if g.Type != config.GeneratorTypeFlow {
			continue
		}
		if exporterVendor(g) != "" {
			flowExporter(env, g, ordinal)
		}
		ordinal++
	}
}

// RequiredHosts lists the host requirements of the configured generators, so
// the environment can be built to hold a system meeting each. Generators that
// accept any system (and appliance-only generators) add nothing.
func RequiredHosts(gens []config.Generator) []datagen.HostRequirement {
	var out []datagen.HostRequirement
	for _, g := range gens {
		if req := hostRequirement(g); req.OS != "" || len(req.Roles) > 0 {
			out = append(out, req)
		}
	}
	return out
}

// hostRequirement is the host a generator can truthfully describe. Windows
// Event Log only exists on Windows; CRI-O container logs come from Linux nodes;
// hostmetrics emits the metric set of its configured OS. Server software runs
// on a server that runs applications, of any OS; other generators accept any
// system.
func hostRequirement(genCfg config.Generator) datagen.HostRequirement {
	// Server software runs on servers: never a router, workstation, or DC.
	servers := []datagen.SystemRole{datagen.RoleServer}
	switch genCfg.Type {
	case config.GeneratorTypeWel:
		return datagen.HostRequirement{OS: datagen.OSWindows, Roles: welRoles(genCfg.Wel.Role)}
	case config.GeneratorTypeKubernetes:
		return datagen.HostRequirement{OS: datagen.OSLinux, Roles: servers}
	case config.GeneratorTypeHostMetrics:
		// Validate accepts any spelling ParseOSType does; an unset os is linux.
		osType, err := datagen.ParseOSType(genCfg.HostMetrics.OS)
		if err != nil {
			osType = datagen.OSLinux
		}
		return datagen.HostRequirement{OS: osType}
	case config.GeneratorTypeApache, config.GeneratorTypeApacheCombined, config.GeneratorTypeApacheError,
		config.GeneratorTypeNginx, config.GeneratorTypePostgres, config.GeneratorTypeJSON,
		config.GeneratorTypeFIX, config.GeneratorTypeTraces:
		return datagen.HostRequirement{Roles: servers, Apps: true}
	default:
		return datagen.HostRequirement{}
	}
}

// welRoles maps a WEL machine role to the host roles that emit its event
// pool. An unset role accepts any Windows role; the generator then takes its
// event pool from the host it lands on.
func welRoles(role string) []datagen.SystemRole {
	switch welcatalog.MachineRole(role) {
	case welcatalog.RoleDC:
		return []datagen.SystemRole{datagen.RoleDC}
	case welcatalog.RoleMember:
		return []datagen.SystemRole{datagen.RoleServer}
	case welcatalog.RoleWorkstation:
		return []datagen.SystemRole{datagen.RoleWorkstation}
	default:
		return []datagen.SystemRole{datagen.RoleDC, datagen.RoleServer, datagen.RoleWorkstation}
	}
}

// newFIX translates the YAML-shaped FIXGeneratorConfig into the
// catalog-typed fix.Config and constructs a FIX generator. Version and
// EnabledCategories strings are validated; an empty version defaults to
// FIX 4.4 and an empty EnabledCategories means "all 10 categories".
func newFIX(logger *zap.Logger, cfg config.FIXGeneratorConfig, consumer embed.LogConsumer, id *datagen.SystemIdentity, tel embed.TelemetrySettings) (embed.ProducerModule, error) {
	fc := fixgen.Config{
		Workers:      cfg.Workers,
		Rate:         cfg.Rate,
		SenderCompID: cfg.SenderCompID,
		TargetCompID: cfg.TargetCompID,
		Seed:         cfg.Seed,
		Identity:     id,
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
