package config

import (
	"fmt"
	"time"
)

// Default flow generator configuration values.
const (
	// DefaultFlowScenario is the traffic-shape preset used when none is set.
	DefaultFlowScenario = "default"
	// DefaultFlowSeed randomizes each run (negative = randomize per the
	// datagen seed contract).
	DefaultFlowSeed = -1
)

// validFlowScenarios is the set of accepted traffic-shape presets.
var validFlowScenarios = map[string]bool{
	"": true, "default": true, "wan-edge": true, "datacenter": true, "ddos-target": true,
}

// FlowGeneratorConfig contains configuration for the network flow generator.
type FlowGeneratorConfig struct {
	// Workers is the number of worker goroutines.
	Workers int `yaml:"workers,omitempty" mapstructure:"workers,omitempty"`
	// Rate is the generation interval per worker.
	Rate time.Duration `yaml:"rate,omitempty" mapstructure:"rate,omitempty"`
	// Scenario is the traffic-shape preset: default, wan-edge, datacenter,
	// ddos-target.
	Scenario string `yaml:"scenario,omitempty" mapstructure:"scenario,omitempty"`
	// Seed is the RNG seed; negative randomizes, 0+ is deterministic.
	Seed int64 `yaml:"seed,omitempty" mapstructure:"seed,omitempty"`
}

// Validate validates the flow generator configuration.
func (c *FlowGeneratorConfig) Validate() error {
	if c.Workers < 1 {
		return fmt.Errorf("flow generator workers must be 1 or greater, got %d", c.Workers)
	}
	if c.Rate <= 0 {
		return fmt.Errorf("flow generator rate must be positive, got %v", c.Rate)
	}
	if !validFlowScenarios[c.Scenario] {
		return fmt.Errorf("flow generator scenario must be one of: default, wan-edge, datacenter, ddos-target; got %q", c.Scenario)
	}
	return nil
}
