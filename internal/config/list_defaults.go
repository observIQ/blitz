package config

import (
	"fmt"

	"github.com/spf13/viper"
)

// ApplyListDefaults gives each `outputs:` and `generators:` entry its
// type's declared defaults (from DefaultOverrides), then re-decodes the
// entries on top, so only keys the user actually wrote override them. An
// explicit zero value (facility: 0, compress: false) therefore survives, and
// an omitted key gets the default.
//
// List entries never read the singular `output` or `generator` stanza. The
// singular and plural forms are one-or-the-other by design (see
// ValidateExclusiveOutputConfig), so nothing set on one reaches the other.
//
// Call it after v.Unmarshal(cfg).
func ApplyListDefaults(v *viper.Viper, cfg *Config) error {
	// Each entry gets its own freshly decoded defaults rather than a copy of
	// one value, so no two entries share a slice or map backing array that
	// decoding an entry could write through. mapstructure then decodes into
	// the existing slice elements, touching only the fields present in each
	// entry's map.
	if n := listLen(v, "outputs"); n > 0 {
		cfg.Outputs = make([]Output, n)
		for i := range cfg.Outputs {
			defs, err := typeDefaults()
			if err != nil {
				return err
			}
			cfg.Outputs[i] = defs.Output
		}
		if err := v.UnmarshalKey("outputs", &cfg.Outputs); err != nil {
			return fmt.Errorf("decode outputs: %w", err)
		}
	}
	if n := listLen(v, "generators"); n > 0 {
		cfg.Generators = make([]Generator, n)
		for i := range cfg.Generators {
			defs, err := typeDefaults()
			if err != nil {
				return err
			}
			cfg.Generators[i] = defs.Generator
		}
		if err := v.UnmarshalKey("generators", &cfg.Generators); err != nil {
			return fmt.Errorf("decode generators: %w", err)
		}
	}
	return nil
}

// typeDefaults decodes the declared override defaults, and nothing else, into
// a Config on a fresh viper, so no user config or env value can reach it.
func typeDefaults() (*Config, error) {
	d := viper.New()
	SetDefaults(d)
	cfg := NewConfig()
	if err := d.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("decode type defaults: %w", err)
	}
	return cfg, nil
}

func listLen(v *viper.Viper, key string) int {
	list, _ := v.Get(key).([]any)
	return len(list)
}

// SetDefaults registers every DefaultOverrides() default on v, the same
// defaults the CLI binds, so any loader resolves unset keys identically.
func SetDefaults(v *viper.Viper) {
	for _, o := range DefaultOverrides() {
		v.SetDefault(o.Field, o.Default)
	}
}
