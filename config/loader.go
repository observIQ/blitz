// Package config is the public entry point for embedding hosts that want
// to consume blitz programmatically. It parses blitz YAML configuration
// bytes (the same shape the standalone CLI accepts via --config) and
// either returns the parsed *Config for hosts that want to do their own
// module construction (Load), or constructs the corresponding
// embed.ProducerModule slice directly (LoadModules).
//
// The latter is the recommended path: it avoids the embedding host
// having to re-implement the generator-type dispatch and keeps the
// per-generator construction details in blitz where they belong.
//
// Standalone blitz CLI users do NOT use this package — they go through
// cmd/blitz directly. This package exists for hosts that import blitz
// as a library (e.g., the OTel telemetrygeneratorreceiver).
package config

import (
	"bytes"
	"fmt"
	"io/fs"

	"github.com/observiq/blitz/embed"
	internalconfig "github.com/observiq/blitz/internal/config"
	"github.com/observiq/blitz/internal/dispatch"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

// Config is the parsed blitz configuration. Re-exported from
// internal/config so embedding hosts can inspect the parsed YAML
// without crossing Go's internal/ visibility boundary.
type Config = internalconfig.Config

// LoadOpts configures Load. The zero value loads pure YAML with no
// overlay.
type LoadOpts struct {
	// EnvOverrides is an optional map of YAML-key → value overlays the
	// host supplies on top of the parsed YAML, mirroring the override
	// path the standalone blitz CLI uses for BLITZ_* environment
	// variables. Keys use the dotted YAML path (e.g. "generator.type"
	// overrides cfg.Generator.Type; "output.otlp-grpc.host" overrides
	// cfg.Output.OTLPGrpc.Host).
	//
	// Hosts populate this from their own environment-loading mechanism
	// — typically by scanning their process env for BLITZ_*-prefixed
	// variables, stripping the prefix, and mapping each to its YAML
	// path before passing them in. blitz does NOT read os.Environ()
	// directly in embedded mode; that remains the host's responsibility
	// so the host's env-loading, secret-resolution, and prefix
	// conventions are not bypassed.
	//
	// CLI flag bindings are out of scope for embedded mode and are not
	// honored via this map. Hosts that need flag-like overrides should
	// translate them into YAML paths themselves.
	EnvOverrides map[string]string
}

// Load parses blitz YAML bytes into a *Config and optionally overlays
// host-supplied env-style overrides on top of the YAML. The returned
// config has been validated.
//
// Use this when the embedding host wants to inspect the parsed config
// (or do its own module construction) rather than going through the
// LoadModules pipeline.
//
// This differs from the standalone CLI loader in two intentional ways:
// (1) blitz does not read os.Environ() directly — see LoadOpts.EnvOverrides
// for the host-driven equivalent; (2) cobra/CLI flag bindings are not
// applied, since embedded hosts have no flags to bind.
func Load(yamlBytes []byte, opts LoadOpts) (*Config, error) {
	v := viper.New()
	v.SetConfigType("yaml")
	if err := v.ReadConfig(bytes.NewReader(yamlBytes)); err != nil {
		return nil, fmt.Errorf("parse blitz YAML: %w", err)
	}
	internalconfig.MigrateDeprecatedKeys(v)
	for k, val := range opts.EnvOverrides {
		v.Set(k, val)
	}
	cfg := internalconfig.NewConfig()
	if err := v.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("unmarshal blitz config: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validate blitz config: %w", err)
	}
	return cfg, nil
}

// EmbedOpts configures the modules built by LoadModules.
type EmbedOpts struct {
	// Logger is the zap logger handed to each constructed generator.
	// Nil yields a no-op logger.
	Logger *zap.Logger

	// LogConsumer is the destination for every record any constructed
	// log generator produces. Required.
	LogConsumer embed.LogConsumer

	// FileGenLibrary is the optional filesystem the filegen generator
	// consults for `package:`-prefixed source references and bare-name
	// library fallbacks. Pass embeddedlibrary.FS() (with the
	// `embed_library` build tag set) to use the data_library snapshot
	// shipped in the blitz module. Nil falls back to reading
	// ./data_library/ from the process cwd, matching what the
	// standalone CLI does.
	FileGenLibrary fs.FS

	// EnvOverrides is forwarded to Load (see LoadOpts.EnvOverrides).
	// Use it when the embedding host wants YAML overlays equivalent to
	// the CLI's BLITZ_* env-var path; blitz never reads os.Environ()
	// itself in embedded mode.
	EnvOverrides map[string]string
}

// LoadModules parses blitz YAML bytes, constructs the corresponding
// embed.ProducerModule instances wired to the host's LogConsumer, and
// returns them ready to be passed to embed.New as
// `embed.Config{Modules: ...}`.
//
// Only Producer-class log generators are supported in embed mode:
// apache-common, apache-combined, apache-error, filegen, json,
// kubernetes, nginx, okta, palo-alto, postgres. Generator types that
// produce metrics (hostmetrics) or traces (traces), or that aren't yet
// migrated to the embed contract (winevt, nop), return an error with a
// pointer at the relevant follow-up.
//
// Returns a non-nil error if any generator in the parsed config is not
// embed-eligible; partial results are NOT returned, so callers don't
// silently lose generators they thought would be wired up.
func LoadModules(yamlBytes []byte, opts EmbedOpts) ([]embed.ProducerModule, error) {
	if opts.LogConsumer == nil {
		return nil, fmt.Errorf("EmbedOpts.LogConsumer is required")
	}
	logger := opts.Logger
	if logger == nil {
		logger = zap.NewNop()
	}
	cfg, err := Load(yamlBytes, LoadOpts{EnvOverrides: opts.EnvOverrides})
	if err != nil {
		return nil, err
	}
	gens := cfg.EffectiveGenerators()
	modules := make([]embed.ProducerModule, 0, len(gens))
	for i, gen := range gens {
		mod, err := dispatch.ForEmbed(logger, gen, opts.LogConsumer, opts.FileGenLibrary)
		if err != nil {
			return nil, fmt.Errorf("generator[%d] type=%q: %w", i, gen.Type, err)
		}
		modules = append(modules, mod)
	}
	return modules, nil
}
