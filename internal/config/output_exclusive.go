package config

import (
	"fmt"

	"github.com/spf13/viper"
)

// ValidateExclusiveOutputConfig rejects a config that sets both the singular
// `output` and the plural `outputs`. The two together are ambiguous: `outputs`
// wins for what runs, while `output` still seeds each entry's defaults, so a
// value on `output` would leak silently into the list. Callers must pick one.
//
// It runs at load against the parsed config sources (`InConfig`), not
// `IsSet`, so the defaults bound for `output.*` do not make it fire when only
// `outputs` is written.
func ValidateExclusiveOutputConfig(v *viper.Viper) error {
	return rejectBothForms(v, "output", "outputs")
}

// ValidateExclusiveGeneratorConfig rejects a config that sets both the
// singular `generator` and the plural `generators`, the same ambiguity the
// output check guards against.
func ValidateExclusiveGeneratorConfig(v *viper.Viper) error {
	return rejectBothForms(v, "generator", "generators")
}

// rejectBothForms errors when both the singular and plural forms of a config
// key are present in the parsed config. Shared by the output check here and,
// later, the generator/generators check.
func rejectBothForms(v *viper.Viper, singular, plural string) error {
	if v.InConfig(singular) && v.InConfig(plural) {
		return fmt.Errorf("set either %s or %s, not both", singular, plural)
	}
	return nil
}
