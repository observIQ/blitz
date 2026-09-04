// Package config contains the top level configuration structures and logic
package config

import (
	"fmt"

	"github.com/observiq/blitz/internal/datagen"
	"go.uber.org/zap"
)

// EnvironmentConfig configures the simulated datagen.Environment that
// generators draw their host identities from. The block is optional; an
// omitted environment yields a randomized default Environment (PIPE-1036).
type EnvironmentConfig struct {
	// DomainName is the AD/DNS domain for the environment. Empty = datagen default.
	DomainName string `yaml:"domain_name,omitempty" mapstructure:"domain_name,omitempty"`
	// SeedConfig controls per-identity-type determinism.
	SeedConfig EnvironmentSeedConfig `yaml:"seed_config,omitempty" mapstructure:"seed_config,omitempty"`
	// Counts controls how many of each identity type are generated.
	Counts EnvironmentCounts `yaml:"counts,omitempty" mapstructure:"counts,omitempty"`
}

// EnvironmentSeedConfig mirrors datagen.SeedConfig as optional YAML keys. An
// omitted (nil) field randomizes that identity type; an explicit value —
// including 0 — is a deterministic seed, per the datagen seed contract.
type EnvironmentSeedConfig struct {
	Shared         *int64 `yaml:"shared,omitempty" mapstructure:"shared,omitempty"`
	Systems        *int64 `yaml:"systems,omitempty" mapstructure:"systems,omitempty"`
	Users          *int64 `yaml:"users,omitempty" mapstructure:"users,omitempty"`
	Groups         *int64 `yaml:"groups,omitempty" mapstructure:"groups,omitempty"`
	Services       *int64 `yaml:"services,omitempty" mapstructure:"services,omitempty"`
	Applications   *int64 `yaml:"applications,omitempty" mapstructure:"applications,omitempty"`
	Networks       *int64 `yaml:"networks,omitempty" mapstructure:"networks,omitempty"`
	Domains        *int64 `yaml:"domains,omitempty" mapstructure:"domains,omitempty"`
	StorageSystems *int64 `yaml:"storage_systems,omitempty" mapstructure:"storage_systems,omitempty"`
	NetworkSystems *int64 `yaml:"network_systems,omitempty" mapstructure:"network_systems,omitempty"`
}

// EnvironmentCounts mirrors the count fields of datagen.EnvironmentOpts. A zero
// (omitted) count uses the datagen package default for that type.
type EnvironmentCounts struct {
	Systems        int `yaml:"systems,omitempty" mapstructure:"systems,omitempty"`
	Users          int `yaml:"users,omitempty" mapstructure:"users,omitempty"`
	Groups         int `yaml:"groups,omitempty" mapstructure:"groups,omitempty"`
	Networks       int `yaml:"networks,omitempty" mapstructure:"networks,omitempty"`
	StorageSystems int `yaml:"storage_systems,omitempty" mapstructure:"storage_systems,omitempty"`
	NetworkSystems int `yaml:"network_systems,omitempty" mapstructure:"network_systems,omitempty"`
	DomainAdmins   int `yaml:"domain_admins,omitempty" mapstructure:"domain_admins,omitempty"`
}

// Validate checks the environment configuration: counts must not be negative.
func (e EnvironmentConfig) Validate() error {
	counts := map[string]int{
		"systems":         e.Counts.Systems,
		"users":           e.Counts.Users,
		"groups":          e.Counts.Groups,
		"networks":        e.Counts.Networks,
		"storage_systems": e.Counts.StorageSystems,
		"network_systems": e.Counts.NetworkSystems,
		"domain_admins":   e.Counts.DomainAdmins,
	}
	for name, v := range counts {
		if v < 0 {
			return fmt.Errorf("environment count %q must not be negative, got %d", name, v)
		}
	}
	return nil
}

// Build resolves the configured Environment, hydrating a datagen.SeedConfig
// (omitted seeds randomize), initializing it (which logs the effective seeds),
// and composing the Environment. A nil logger is treated as a no-op logger.
func (e EnvironmentConfig) Build(logger *zap.Logger) (*datagen.Environment, error) {
	if logger == nil {
		logger = zap.NewNop()
	}

	seeds := datagen.NewSeedConfig()
	sc := e.SeedConfig
	if sc.Shared != nil {
		seeds.Shared = *sc.Shared
	}
	if sc.Systems != nil {
		seeds.Systems = *sc.Systems
	}
	if sc.Users != nil {
		seeds.Users = *sc.Users
	}
	if sc.Groups != nil {
		seeds.Groups = *sc.Groups
	}
	if sc.Services != nil {
		seeds.Services = *sc.Services
	}
	if sc.Applications != nil {
		seeds.Applications = *sc.Applications
	}
	if sc.Networks != nil {
		seeds.Networks = *sc.Networks
	}
	if sc.Domains != nil {
		seeds.Domains = *sc.Domains
	}
	if sc.StorageSystems != nil {
		seeds.StorageSystems = *sc.StorageSystems
	}
	if sc.NetworkSystems != nil {
		seeds.NetworkSystems = *sc.NetworkSystems
	}
	seeds.Init(logger)

	opts := &datagen.EnvironmentOpts{
		DomainName:         e.DomainName,
		SystemCount:        e.Counts.Systems,
		UserCount:          e.Counts.Users,
		GroupCount:         e.Counts.Groups,
		NetworkCount:       e.Counts.Networks,
		StorageSystemCount: e.Counts.StorageSystems,
		NetworkSystemCount: e.Counts.NetworkSystems,
		DomainAdminsCount:  e.Counts.DomainAdmins,
		Logger:             logger,
	}
	return datagen.GenerateEnvironment(seeds, opts)
}
