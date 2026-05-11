package datagen

import (
	"math/rand"
	"time"

	"go.uber.org/zap"
)

// IdentityType is a typed identifier for per-identity-type seed overrides.
// Use the exported constants below; callers passing an unrecognized value
// fall back to SeedConfig.Shared.
type IdentityType string

const (
	IdentitySystems      IdentityType = "systems"
	IdentityUsers        IdentityType = "users"
	IdentityGroups       IdentityType = "groups"
	IdentityServices     IdentityType = "services"
	IdentityApplications IdentityType = "applications"
	IdentityNetworks     IdentityType = "networks"
	IdentityDomains      IdentityType = "domains"
)

// SeedConfig controls deterministic generation across all identity types.
//
// Contract: any negative value (e.g. -1) means "randomize"; 0 and positive
// values are used verbatim as deterministic seeds. Use NewSeedConfig to
// obtain a config whose fields default to -1 (randomize), so that omitted
// YAML/config keys produce randomized runs. A bare &SeedConfig{} struct
// literal yields all zero values, which the contract interprets as
// deterministic seed 0 across the board.
type SeedConfig struct {
	// Shared seed for all identity types. <0 = randomize at Init.
	Shared int64

	// Per-identity-type overrides. <0 = fall back to Shared for that type.
	Systems      int64
	Users        int64
	Groups       int64
	Services     int64
	Applications int64
	Networks     int64
	Domains      int64
}

// NewSeedConfig returns a SeedConfig with every field set to -1 so that an
// uninitialized configuration randomizes at Init. Callers that hydrate
// SeedConfig from YAML/viper should set the same -1 default per field.
func NewSeedConfig() *SeedConfig {
	return &SeedConfig{
		Shared:       -1,
		Systems:      -1,
		Users:        -1,
		Groups:       -1,
		Services:     -1,
		Applications: -1,
		Networks:     -1,
		Domains:      -1,
	}
}

// ResolveSeed returns the effective seed for a given identity type.
// A non-negative per-type override wins; negative overrides fall back to
// Shared. Init must be called first if Shared was negative, so callers always
// observe a non-negative Shared value.
func (s *SeedConfig) ResolveSeed(identityType IdentityType) int64 {
	var override int64 = -1
	switch identityType {
	case IdentitySystems:
		override = s.Systems
	case IdentityUsers:
		override = s.Users
	case IdentityGroups:
		override = s.Groups
	case IdentityServices:
		override = s.Services
	case IdentityApplications:
		override = s.Applications
	case IdentityNetworks:
		override = s.Networks
	case IdentityDomains:
		override = s.Domains
	}
	if override >= 0 {
		return override
	}
	return s.Shared
}

// Init generates a random Shared seed if Shared is negative and logs all
// effective seeds.
func (s *SeedConfig) Init(logger *zap.Logger) {
	if s.Shared < 0 {
		s.Shared = rand.New(rand.NewSource(time.Now().UnixNano())).Int63() // #nosec G404
	}

	logger.Info("datagen seeds initialized",
		zap.Int64("shared", s.Shared),
		zap.Int64("systems", s.ResolveSeed(IdentitySystems)),
		zap.Int64("users", s.ResolveSeed(IdentityUsers)),
		zap.Int64("groups", s.ResolveSeed(IdentityGroups)),
		zap.Int64("services", s.ResolveSeed(IdentityServices)),
		zap.Int64("applications", s.ResolveSeed(IdentityApplications)),
		zap.Int64("networks", s.ResolveSeed(IdentityNetworks)),
		zap.Int64("domains", s.ResolveSeed(IdentityDomains)),
	)
}
