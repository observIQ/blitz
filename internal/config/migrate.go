package config

import (
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

// MigrateDeprecatedKeys rewrites legacy YAML key names to their current
// canonical equivalents in the supplied viper instance, so old configs
// continue to load after a key rename.
//
// Viper aliases (`viper.RegisterAlias`) are not used here because they
// are consulted only by Get / IsSet; Unmarshal traverses the underlying
// settings tree directly and ignores aliases. The only way to make a
// renamed key honored by Unmarshal is to physically move the value
// before decoding.
//
// Currently migrated keys (all forms case-insensitive in viper):
//   - generator.paloAlto → generator.palo-alto
//   - output.otlpGrpc    → output.otlp-grpc
//
// Behavior:
//   - If only the legacy key is set in the config file, its value
//     moves to the canonical key.
//   - If only the canonical key is set in the config file, nothing
//     changes.
//   - If both are set in the config file, the canonical key wins and
//     the legacy key is left in place (no-op, since Unmarshal will
//     read the canonical).
//
// The "is set in the config file" check uses `v.InConfig`, not
// `v.IsSet`. The CLI path binds defaults for every override via
// `Override.Bind` (e.g. `v.SetDefault("output.otlp-grpc.host", "")`)
// before the YAML is read, which makes `v.IsSet("output.otlp-grpc")`
// return true even when the user only wrote the legacy `otlpGrpc:`
// sub-tree — causing the migration guard to skip and silently drop
// the user's values. `v.InConfig` ignores defaults / env / flags and
// reflects only what the parsed config sources contain, which is the
// semantic this migration actually wants.
//
// These deprecated keys will be removed in a future release; users
// should update their configs to the canonical form.
func MigrateDeprecatedKeys(v *viper.Viper) {
	// viper lowercases keys when storing; the lookup tokens here are
	// pre-lowercased to match what's actually in the settings map.
	renames := []struct{ from, to string }{
		{"generator.paloalto", "generator.palo-alto"},
		{"output.otlpgrpc", "output.otlp-grpc"},
	}
	for _, r := range renames {
		if v.InConfig(r.from) && !v.InConfig(r.to) {
			v.Set(r.to, v.Get(r.from))
		}
	}
}

// LogGeneratorDeprecations emits a Warn-level banner once per startup
// for every configured generator type that has been deprecated. The
// generator itself continues to function — this is a migration nudge,
// not a refusal. Embed users get a hard refusal at construction time
// in dispatch.ForEmbed; this function exists for the standalone CLI
// path where backward compatibility is preserved.
//
// Currently emits warnings for:
//   - generator.type: winevt → replaced by the multi-channel `wel` generator
func LogGeneratorDeprecations(logger *zap.Logger, cfg *Config) {
	if logger == nil || cfg == nil {
		return
	}
	for _, g := range cfg.EffectiveGenerators() {
		if g.Type == GeneratorTypeWinevt {
			logger.Warn(
				"DEPRECATED: the `winevt` generator is deprecated and not available via embed; " +
					"migrate to the `wel` generator (multi-channel, embed-friendly Windows Event Log). " +
					"See docs/generator/wel.md. The `winevt` generator continues to function in standalone " +
					"CLI mode for now.",
			)
		}
	}
}
