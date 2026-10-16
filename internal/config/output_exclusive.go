package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// ValidateExclusiveOutputConfig rejects a config that sets both the singular
// `output` and the plural `outputs`. The two are one-or-the-other by design:
// a config describes its outputs in exactly one form, and neither form reads
// from the other (list entries take only their type's defaults).
//
// It also rejects a malformed `outputs`: anything but a non-empty list of
// entries that each name a `type`.
//
// It runs at load, before Unmarshal. A singular value from any source (config
// file, env var, flag, embedded override) counts, even one equal to its
// default, but the declared defaults for `output.*` alone do not, so writing
// only `outputs` never fires it. explicit reports whether an override's env
// var, flag, or embedded override was set; nil means none were.
func ValidateExclusiveOutputConfig(v *viper.Viper, explicit func(*Override) bool) error {
	if err := rejectBothForms(v, explicit, "output", "outputs"); err != nil {
		return err
	}
	return validateListForm(v, "output", "outputs")
}

// ValidateExclusiveGeneratorConfig rejects a config that sets both the
// singular `generator` and the plural `generators`, which are
// one-or-the-other by design in the same way as `output` and `outputs`. It
// also rejects a malformed `generators`, as ValidateExclusiveOutputConfig
// does for `outputs`. explicit is as for ValidateExclusiveOutputConfig.
func ValidateExclusiveGeneratorConfig(v *viper.Viper, explicit func(*Override) bool) error {
	if err := rejectBothForms(v, explicit, "generator", "generators"); err != nil {
		return err
	}
	return validateListForm(v, "generator", "generators")
}

// runLevelKeys sit under the singular generator's prefix but configure the
// whole run, so they may be set next to a generators: list.
var runLevelKeys = map[string]bool{"generator.count": true}

// rejectBothForms errors when the plural form is in the config and the
// singular form is set anywhere: in the config file, by a source explicit
// reports, or by any value that differs from the declared default. Viper's
// IsSet counts defaults, so it cannot tell an explicit default from none.
func rejectBothForms(v *viper.Viper, explicit func(*Override) bool, singular, plural string) error {
	if !v.InConfig(plural) {
		return nil
	}
	if v.InConfig(singular) {
		return fmt.Errorf("set either %s or %s, not both", singular, plural)
	}
	prefix := singular + "."
	for _, o := range DefaultOverrides() {
		if !strings.HasPrefix(o.Field, prefix) || runLevelKeys[o.Field] {
			continue
		}
		if explicit != nil && explicit(o) {
			return fmt.Errorf("set either %s or %s, not both (%s is set by env, flag, or override)", singular, plural, o.Field)
		}
		if val := v.Get(o.Field); val != nil && fmt.Sprint(val) != fmt.Sprint(o.Default) {
			return fmt.Errorf("set either %s or %s, not both (%s is set by env, flag, or override)", singular, plural, o.Field)
		}
	}
	return nil
}

// CLIExplicit reports an override as explicit when its flag was passed or its
// env var is present, whatever the value.
func CLIExplicit(flags *pflag.FlagSet) func(*Override) bool {
	return func(o *Override) bool {
		_, env := os.LookupEnv(o.Env)
		return env || flags.Changed(o.Flag)
	}
}

// validateListForm errors unless plural, when present, is a non-empty list of
// entries that each set a type. A malformed list would otherwise fail deep in
// decoding with an unclear error, or silently run a type-less entry.
func validateListForm(v *viper.Viper, singular, plural string) error {
	raw := v.Get(plural)
	if raw == nil && !v.InConfig(plural) {
		return nil
	}
	list, ok := raw.([]any)
	if !ok {
		return fmt.Errorf("%s must be a list of %s entries", plural, singular)
	}
	if len(list) == 0 {
		return fmt.Errorf("%s must not be empty; remove it to use %s", plural, singular)
	}
	for i, e := range list {
		entry, ok := e.(map[string]any)
		if !ok {
			return fmt.Errorf("%s[%d] must be a map of %s settings", plural, i, singular)
		}
		if typ, _ := entry["type"].(string); typ == "" {
			return fmt.Errorf("%s[%d]: type is required", plural, i)
		}
	}
	return nil
}
