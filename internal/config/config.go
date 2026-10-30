// Package config contains the top level configuration structures and logic
package config

import (
	"fmt"
	"math"
	"strings"
	"time"
)

// Config is the configuration for blitz.
type Config struct {
	// Logging configuration for the logger
	Logging Logging `yaml:"logging,omitempty" mapstructure:"logging,omitempty"`
	// Generator configuration
	Generator Generator `yaml:"generator,omitempty" mapstructure:"generator,omitempty"`
	// Generators is the list of generators for multi-generator mode. It is the
	// alternative to the singular Generator, never a supplement: setting both
	// is rejected. Each entry takes only its type's defaults, never values
	// from Generator.
	Generators []Generator `yaml:"generators,omitempty" mapstructure:"generators,omitempty"`
	// Output configuration
	Output Output `yaml:"output,omitempty" mapstructure:"output,omitempty"`
	// Outputs is the list of outputs for multi-output (fan-out) mode; every
	// generated record is fanned out to all of them. It is the alternative to
	// the singular Output, never a supplement: setting both is rejected. Each
	// entry takes only its type's defaults, never values from Output.
	Outputs []Output `yaml:"outputs,omitempty" mapstructure:"outputs,omitempty"`
	// Metrics configuration (Prometheus scrape endpoint for self-metrics)
	Metrics Metrics `yaml:"metrics,omitempty" mapstructure:"metrics,omitempty"`
	// Environment configures the simulated datagen.Environment identities
	// that generators draw their host.name/OS from.
	Environment EnvironmentConfig `yaml:"environment,omitempty" mapstructure:"environment,omitempty"`
	// Telemetry configures export of blitz's own self-telemetry (self-traces,
	// and later self-logs) via OTLP.
	Telemetry Telemetry `yaml:"telemetry,omitempty" mapstructure:"telemetry,omitempty"`
	// OnFinish controls behavior when finite generation completes.
	// One of: "exit" (default), "idle"
	OnFinish string `yaml:"onFinish,omitempty" mapstructure:"onFinish,omitempty"`
}

// Validate validates the entire configuration
func (c *Config) Validate() error {
	if err := c.Logging.Validate(); err != nil {
		return err
	}
	if len(c.Generators) > 0 {
		// generator.count may sit next to the list as the run's total.
		if c.Generator.Count < 0 {
			return fmt.Errorf("generator count must be 0 or greater, got %d", c.Generator.Count)
		}
		// Index by YAML position: an entry that expands (comma-separated
		// hostmetrics OS values) must not shift the entries after it.
		for i, entry := range c.Generators {
			for _, g := range expandGenerators([]Generator{entry}) {
				if err := g.Validate(); err != nil {
					return fmt.Errorf("generators[%d] validation failed: %w", i, err)
				}
			}
			// A replay ends the run at the end of its input through a count
			// of its own, which other entries would share or override.
			if len(c.Generators) > 1 && entry.Type == GeneratorTypeFile && entry.Filegen.Mode == FileModeReplay {
				return fmt.Errorf("generators[%d]: filegen replay must be the only generator", i)
			}
		}
	} else if err := c.Generator.Validate(); err != nil {
		return err
	}
	if len(c.Outputs) > 0 {
		for i, o := range c.Outputs {
			if err := o.Validate(); err != nil {
				return fmt.Errorf("outputs[%d] validation failed: %w", i, err)
			}
		}
	} else if err := c.Output.Validate(); err != nil {
		return err
	}
	if err := c.validateCountFlush(); err != nil {
		return err
	}
	if err := c.Metrics.Validate(); err != nil {
		return err
	}
	if err := c.Environment.Validate(); err != nil {
		return err
	}
	if err := c.Telemetry.Validate(); err != nil {
		return err
	}
	if c.OnFinish != "" && c.OnFinish != "exit" && c.OnFinish != "idle" {
		return fmt.Errorf("onFinish must be one of: exit, idle, got %q", c.OnFinish)
	}
	return nil
}

// validateCountFlush rejects a count run whose remote-write output never
// flushes a partial batch: that batch would wait for Stop, which waits for
// the count, so the run would never end.
func (c *Config) validateCountFlush() error {
	counted := c.Generator.Count > 0
	for _, g := range c.Generators {
		counted = counted || g.Count > 0
	}
	if !counted {
		return nil
	}
	for i, o := range c.EffectiveOutputs() {
		if o.Type == OutputTypePrometheusRemoteWrite && o.PrometheusRemoteWrite.BatchTimeout == 0 {
			err := fmt.Errorf("prometheus-remote-write output batchTimeout must be greater than 0 when generator.count or a generators: entry count is set, or a partial batch is never sent")
			if len(c.Outputs) > 0 {
				return fmt.Errorf("outputs[%d] validation failed: %w", i, err)
			}
			return err
		}
	}
	return nil
}

// EffectiveGenerators returns the list of generators to use: Generators when
// set, otherwise the singular Generator. Each Generators entry already carries
// its type's defaults (ApplyListDefaults at load), never values from the
// singular Generator.
// Comma-separated HostMetrics OS values are expanded into separate generators.
func (c *Config) EffectiveGenerators() []Generator {
	if len(c.Generators) == 0 {
		return expandGenerators([]Generator{c.Generator})
	}
	return expandGenerators(c.Generators)
}

// EffectiveOutputs returns the list of outputs to use: Outputs when set,
// otherwise the singular Output. Each Outputs entry already carries its type's
// defaults (ApplyListDefaults at load), never values from the singular Output.
func (c *Config) EffectiveOutputs() []Output {
	if len(c.Outputs) == 0 {
		return []Output{c.Output}
	}
	return c.Outputs
}

// expandedSeedStride separates the seeds of hostmetrics entries expanded from
// one comma-separated os value.
const expandedSeedStride = 1 << 32

// ResolveExpandedSeeds resolves a random (negative) seed on a hostmetrics entry
// that expands across several OSes once for the whole entry, so every call to
// EffectiveGenerators sees the same seeds and the seed the first host logs
// replays every host. Both loaders call it after decoding.
func (c *Config) ResolveExpandedSeeds() {
	resolve := func(g *Generator) {
		if g.Type == GeneratorTypeHostMetrics && strings.Contains(g.HostMetrics.OS, ",") && g.HostMetrics.Seed < 0 {
			g.HostMetrics.Seed = time.Now().UnixNano()
		}
	}
	// Only generators: entries expand; the singular form's Validate rejects
	// a comma-separated os.
	for i := range c.Generators {
		resolve(&c.Generators[i])
	}
}

// expandGenerators expands comma-separated HostMetrics OS values.
func expandGenerators(gens []Generator) []Generator {
	var result []Generator
	for _, g := range gens {
		if g.Type == GeneratorTypeHostMetrics && strings.Contains(g.HostMetrics.OS, ",") {
			parts := strings.Split(g.HostMetrics.OS, ",")
			clone := int64(0)
			for _, os := range parts {
				trimmed := strings.TrimSpace(os)
				if trimmed == "" {
					continue
				}
				expanded := g
				expanded.HostMetrics.OS = trimmed
				// Each host needs its own stream: a shared deterministic seed
				// would emit identical values. The stride clears worker
				// offsets (worker N uses Seed+N), wrapping within the
				// non-negative range so a large seed stays deterministic.
				if g.HostMetrics.Seed >= 0 {
					expanded.HostMetrics.Seed = int64((uint64(g.HostMetrics.Seed) + uint64(clone)*expandedSeedStride) & math.MaxInt64) // #nosec G115 -- masked to the non-negative int64 range
				}
				clone++
				result = append(result, expanded)
			}
		} else {
			result = append(result, g)
		}
	}
	return result
}

// NewConfig returns a new config
func NewConfig() *Config {
	return &Config{}
}
