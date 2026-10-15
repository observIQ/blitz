package config

import (
	"strings"

	"github.com/spf13/viper"
)

// DefaultListSeeds gives every generators: list entry that omits its seed the
// declared seed default (-1, random), as the singular generator.<type>.seed
// gets from its override default. Decoding alone would leave an omitted seed
// at 0, which the seed contract reads as deterministic. An explicit seed,
// including 0, is kept. The entries are edited in place in the config v read,
// not set as an override, which would outlive the next config read.
func DefaultListSeeds(v *viper.Viper) {
	list, ok := v.Get("generators").([]any)
	if !ok {
		return
	}
	defaults := map[string]any{}
	for _, o := range DefaultOverrides() {
		rest, ok := strings.CutPrefix(o.Field, "generator.")
		if typ, isSeed := strings.CutSuffix(rest, ".seed"); ok && isSeed {
			defaults[typ] = o.Default
		}
	}
	for _, e := range list {
		entry, ok := e.(map[string]any)
		if !ok {
			continue
		}
		typ, _ := entry["type"].(string)
		def, ok := defaults[typ]
		if !ok {
			continue
		}
		sub, _ := entry[typ].(map[string]any)
		if sub == nil {
			sub = map[string]any{}
			entry[typ] = sub
		}
		if _, set := sub["seed"]; !set {
			sub["seed"] = def
		}
	}
}
